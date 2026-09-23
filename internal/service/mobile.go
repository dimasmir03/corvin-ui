package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"vpnpanel/internal/config"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MobileError struct {
	Code    string
	Message string
	Details map[string]any
}

func (e *MobileError) Error() string       { return e.Message }
func mobileErr(code, message string) error { return &MobileError{Code: code, Message: message} }

type MobileIdentity struct {
	UserID              uint
	DeviceID, SessionID string
}
type MobileStartInput struct{ InstallID, Platform, AppVersion, DeviceName, CodeChallenge, CodeChallengeMethod string }
type MobileStartResult struct {
	LoginID          string    `json:"login_id"`
	TelegramDeeplink string    `json:"telegram_deeplink"`
	ExpiresAt        time.Time `json:"expires_at"`
	PollAfterSeconds int       `json:"poll_after_seconds"`
}
type MobileTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	SessionID    string `json:"session_id"`
	ExpiresIn    int    `json:"expires_in"`
}
type MobileConnectInput struct {
	DeviceID       string `json:"device_id"`
	ServerID       string `json:"server_id"`
	Protocol       string `json:"protocol"`
	Mode           string `json:"mode"`
	ConfigRevision string `json:"config_revision"`
}
type MobileVPNConfig struct {
	ConnectionID   string     `json:"connection_id"`
	ServerID       string     `json:"server_id"`
	Protocol       string     `json:"protocol"`
	ConfigURL      string     `json:"config_url"`
	ConfigRevision string     `json:"config_revision"`
	ValidUntil     *time.Time `json:"valid_until"`
}
type MobileOperationView struct {
	OperationID       string                `json:"operation_id"`
	Status            string                `json:"status"`
	RetryAfterSeconds int                   `json:"retry_after_seconds"`
	Result            *MobileVPNConfig      `json:"result"`
	Error             *MobileOperationError `json:"error"`
}
type MobileOperationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type MobileServerView struct {
	ServerID    string   `json:"server_id"`
	CountryCode string   `json:"country_code"`
	CountryName string   `json:"country_name"`
	City        string   `json:"city"`
	Status      string   `json:"status"`
	LoadPercent int      `json:"load_percent"`
	Protocols   []string `json:"protocols"`
	Recommended bool     `json:"recommended"`
	Available   bool     `json:"available"`
}

type MobileService struct {
	repo    *repository.MobileRepo
	vpnRepo *repository.VpnRepo
	vpn     *VPNService
	cfg     config.MobileConfig
	now     func() time.Time
}

func NewMobileService(repo *repository.MobileRepo, vpnRepo *repository.VpnRepo, vpn *VPNService, cfg config.MobileConfig) *MobileService {
	return &MobileService{repo: repo, vpnRepo: vpnRepo, vpn: vpn, cfg: cfg, now: time.Now}
}

func (s *MobileService) StartTelegramLogin(input MobileStartInput) (MobileStartResult, error) {
	input.InstallID = strings.TrimSpace(input.InstallID)
	input.Platform = strings.ToLower(strings.TrimSpace(input.Platform))
	input.DeviceName = strings.TrimSpace(input.DeviceName)
	if _, err := uuid.Parse(input.InstallID); err != nil {
		return MobileStartResult{}, mobileErr("INVALID_REQUEST", "install_id must be a UUID")
	}
	if input.Platform != "android" && input.Platform != "ios" {
		return MobileStartResult{}, mobileErr("INVALID_REQUEST", "unsupported platform")
	}
	if input.CodeChallengeMethod != "S256" || len(input.CodeChallenge) < 43 || len(input.DeviceName) == 0 || len(input.DeviceName) > 120 {
		return MobileStartResult{}, mobileErr("INVALID_REQUEST", "invalid PKCE or device metadata")
	}
	rawApproval, err := randomToken(32)
	if err != nil {
		return MobileStartResult{}, err
	}
	now := s.now().UTC()
	login := models.MobileLoginSession{ID: uuid.NewString(), ApprovalTokenHash: tokenHash(rawApproval), CodeChallenge: input.CodeChallenge, InstallID: input.InstallID, Platform: input.Platform, AppVersion: input.AppVersion, DeviceName: input.DeviceName, Status: "pending", ExpiresAt: now.Add(time.Duration(s.cfg.LoginTTLMinutes) * time.Minute)}
	if err := s.repo.CreateLogin(&login); err != nil {
		return MobileStartResult{}, err
	}
	username := strings.TrimPrefix(strings.TrimSpace(s.cfg.TelegramBotUsername), "@")
	deeplink := "https://t.me/" + username + "?start=login_" + rawApproval
	if username == "" {
		deeplink = "tg://resolve?start=login_" + rawApproval
	}
	return MobileStartResult{LoginID: login.ID, TelegramDeeplink: deeplink, ExpiresAt: login.ExpiresAt, PollAfterSeconds: 2}, nil
}

func (s *MobileService) TelegramLoginStatus(loginID string) (string, error) {
	login, err := s.repo.GetLogin(strings.TrimSpace(loginID))
	if err != nil {
		return "", err
	}
	if !login.ExpiresAt.After(s.now().UTC()) {
		return "expired", nil
	}
	switch login.Status {
	case "approved", "denied":
		return login.Status, nil
	default:
		return "pending", nil
	}
}

func (s *MobileService) ApproveTelegramLogin(rawToken string, telegramID int64) error {
	telegram, err := s.vpn.telegramRepo.FindByTgID(telegramID)
	if err != nil {
		return err
	}
	user, err := s.repo.GetUser(telegram.UserID)
	if err != nil {
		return err
	}
	if !user.Status {
		return mobileErr("USER_BLOCKED", "user is blocked")
	}
	return s.repo.ApproveLogin(tokenHash(strings.TrimSpace(rawToken)), telegram.UserID, s.now().UTC())
}

func (s *MobileService) ExchangeTelegramLogin(loginID, verifier, installID string) (MobileTokens, error) {
	now := s.now().UTC()
	var result MobileTokens
	err := s.repo.ExchangeLogin(strings.TrimSpace(loginID), now, func(tx *gorm.DB, login models.MobileLoginSession) error {
		if login.InstallID != installID || !verifyPKCE(verifier, login.CodeChallenge) {
			return mobileErr("UNAUTHORIZED", "PKCE verification failed")
		}
		var user models.User
		err := tx.Where("id = ?", *login.UserID).Take(&user).Error
		if err != nil {
			return err
		}
		if !user.Status {
			return mobileErr("USER_BLOCKED", "user is blocked")
		}
		var subscription models.UserSubscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", *login.UserID).Take(&subscription).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			subscription = models.UserSubscription{UserID: *login.UserID, Status: models.SubscriptionStatusActive, TariffName: "default", DeviceLimit: 1}
			if err := tx.Create(&subscription).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		var existing models.MobileDevice
		existingErr := tx.Where("install_id = ?", installID).Take(&existing).Error
		if errors.Is(existingErr, gorm.ErrRecordNotFound) {
			var count int64
			if err := tx.Model(&models.MobileDevice{}).Where("user_id = ? AND status = ?", *login.UserID, models.MobileDeviceStatusActive).Count(&count).Error; err != nil {
				return err
			}
			if subscription.DeviceLimit <= 0 || count >= int64(subscription.DeviceLimit) {
				return mobileErr("DEVICE_LIMIT_REACHED", "device limit reached")
			}
		} else if existingErr != nil {
			return existingErr
		}
		deviceID := uuid.NewString()
		if existingErr == nil {
			deviceID = existing.ID
		}
		refresh, err := randomToken(48)
		if err != nil {
			return err
		}
		session := models.MobileSession{ID: uuid.NewString(), UserID: *login.UserID, DeviceID: deviceID, InstallID: installID, RefreshTokenHash: tokenHash(refresh), ExpiresAt: now.Add(time.Duration(s.cfg.RefreshTTLHours) * time.Hour), LastUsedAt: now}
		device := models.MobileDevice{ID: deviceID, UserID: *login.UserID, InstallID: installID, DeviceName: login.DeviceName, Platform: login.Platform, Status: models.MobileDeviceStatusActive, LastSeenAt: now, UpdatedAt: now}
		if err := s.repo.CreateDeviceAndSession(tx, device, session); err != nil {
			if strings.Contains(err.Error(), "revoked") {
				return mobileErr("DEVICE_REVOKED", "device is revoked")
			}
			return err
		}
		access, err := s.issueAccess(MobileIdentity{UserID: *login.UserID, DeviceID: deviceID, SessionID: session.ID})
		if err != nil {
			return err
		}
		result = MobileTokens{AccessToken: access, RefreshToken: refresh, SessionID: session.ID, ExpiresIn: s.cfg.AccessTTL}
		return nil
	})
	if errors.Is(err, repository.ErrMobileLoginExpired) {
		return MobileTokens{}, mobileErr("UNAUTHORIZED", "login expired")
	}
	if errors.Is(err, repository.ErrMobileLoginReplayed) {
		return MobileTokens{}, mobileErr("UNAUTHORIZED", "login already used")
	}
	if errors.Is(err, repository.ErrMobileLoginPending) {
		return MobileTokens{}, mobileErr("UNAUTHORIZED", "login is not approved")
	}
	return result, err
}

func (s *MobileService) Refresh(refreshToken, installID string) (MobileTokens, error) {
	now := s.now().UTC()
	next, err := randomToken(48)
	if err != nil {
		return MobileTokens{}, err
	}
	session, err := s.repo.RotateRefresh(tokenHash(refreshToken), tokenHash(next), installID, now.Add(time.Duration(s.cfg.RefreshTTLHours)*time.Hour), now)
	if errors.Is(err, repository.ErrRefreshReused) {
		return MobileTokens{}, mobileErr("TOKEN_REUSED", "refresh token reuse detected")
	}
	if err != nil {
		return MobileTokens{}, mobileErr("UNAUTHORIZED", "invalid refresh token")
	}
	device, err := s.repo.GetDevice(session.DeviceID)
	if err != nil || device.Status != models.MobileDeviceStatusActive {
		return MobileTokens{}, mobileErr("DEVICE_REVOKED", "device is not active")
	}
	access, err := s.issueAccess(MobileIdentity{UserID: session.UserID, DeviceID: session.DeviceID, SessionID: session.ID})
	if err != nil {
		return MobileTokens{}, err
	}
	return MobileTokens{AccessToken: access, RefreshToken: next, SessionID: session.ID, ExpiresIn: s.cfg.AccessTTL}, nil
}

func (s *MobileService) Authenticate(accessToken string) (MobileIdentity, error) {
	identity, exp, err := s.parseAccess(accessToken)
	if err != nil || !exp.After(s.now().UTC()) {
		return MobileIdentity{}, mobileErr("UNAUTHORIZED", "invalid access token")
	}
	session, err := s.repo.GetSession(identity.SessionID)
	if err != nil || session.RevokedAt != nil || !session.ExpiresAt.After(s.now().UTC()) || session.UserID != identity.UserID || session.DeviceID != identity.DeviceID {
		return MobileIdentity{}, mobileErr("UNAUTHORIZED", "session is not active")
	}
	device, err := s.repo.GetDevice(identity.DeviceID)
	if err != nil || device.Status != models.MobileDeviceStatusActive {
		return MobileIdentity{}, mobileErr("DEVICE_REVOKED", "device is not active")
	}
	user, err := s.repo.GetUser(identity.UserID)
	if err != nil || !user.Status {
		return MobileIdentity{}, mobileErr("USER_BLOCKED", "user is blocked")
	}
	_ = s.repo.TouchDevice(identity.DeviceID, s.now().UTC())
	return identity, nil
}

func (s *MobileService) Logout(identity MobileIdentity, refreshToken string) error {
	if refreshToken != "" {
		session, err := s.repo.FindSessionByRefreshHash(tokenHash(refreshToken))
		if err != nil || session.ID != identity.SessionID {
			return mobileErr("UNAUTHORIZED", "refresh token does not match session")
		}
	}
	return s.repo.RevokeSession(identity.SessionID, s.now().UTC())
}

func (s *MobileService) Subscription(userID uint) (map[string]any, error) {
	sub, err := s.vpnRepo.GetOrCreateSubscription(userID)
	if err != nil {
		return nil, err
	}
	count, err := s.repo.CountActiveDevices(userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": sub.Status, "tariff_name": sub.TariffName, "expires_at": sub.ExpiresAt, "device_limit": sub.DeviceLimit, "devices_used": count}, nil
}

func (s *MobileService) Devices(identity MobileIdentity) ([]map[string]any, error) {
	devices, err := s.repo.ListDevices(identity.UserID)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(devices))
	for _, d := range devices {
		result = append(result, map[string]any{"id": d.ID, "device_name": d.DeviceName, "platform": d.Platform, "status": d.Status, "current": d.ID == identity.DeviceID, "last_seen_at": d.LastSeenAt})
	}
	return result, nil
}

func (s *MobileService) RevokeDevice(identity MobileIdentity, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	details, _ := s.vpn.GetUserVPNDetails(identity.UserID)
	if err := s.repo.RevokeDevice(identity.UserID, deviceID, s.now().UTC()); err != nil {
		return err
	}
	for _, profile := range details.Profiles {
		if profile.DeviceID != deviceID {
			continue
		}
		for _, node := range profile.Nodes {
			if node.Status == models.VPNProfileNodeStatusDeleted {
				continue
			}
			if _, err := s.vpn.RequestConnectionState(identity.UserID, node.ID, models.VPNConnectionDesiredDeleted); err != nil {
				logger.Warn("revoked mobile device remote credential cleanup queued unsuccessfully", "component", "mobile_service", "device_id", deviceID, "connection_id", node.ID, "error", err)
			}
		}
	}
	return nil
}

func (s *MobileService) Servers(userID uint) (string, []MobileServerView, error) {
	registries, err := s.vpnRepo.ListRegisteredServers()
	if err != nil {
		return "", nil, err
	}
	access, err := s.vpnRepo.ListServerAccessByUserID(userID)
	if err != nil {
		return "", nil, err
	}
	accessMap := map[string]models.UserServerAccess{}
	for _, a := range access {
		accessMap[a.ServerID] = a
	}
	explicit := len(access) > 0
	result := []MobileServerView{}
	autoProtocols := map[string]bool{}
	now := s.now()
	latest := time.Time{}
	for _, r := range registries {
		if r.UpdatedAt.After(latest) {
			latest = r.UpdatedAt
		}
		allowed := r.Enabled && r.ArchivedAt == nil
		a, ok := accessMap[r.ServerID]
		if explicit {
			allowed = allowed && ok && a.Enabled && (a.ValidUntil == nil || a.ValidUntil.After(now))
		}
		protocols := []string{}
		p := strings.ToLower(strings.TrimSpace(r.ExpectedProtocol))
		if p == "vless" && (!explicit || a.AllowVLESS) {
			protocols = append(protocols, "vless")
		}
		if p == "trojan" && (!explicit || a.AllowTrojan) {
			protocols = append(protocols, "trojan")
		}
		var state models.NodeState
		_ = s.vpnRepo.DB.Where("server_id = ?", r.ServerID).Take(&state).Error
		status := mobileServerStatus(state.Status)
		if !allowed {
			status = "maintenance"
		}
		load := state.OnlineCount * 5
		if load > 100 {
			load = 100
		}
		available := allowed && len(protocols) > 0 && status != "offline" && status != "maintenance"
		if available {
			for _, protocol := range protocols {
				autoProtocols[protocol] = true
			}
		}
		result = append(result, MobileServerView{ServerID: r.ServerID, CountryCode: r.CountryCode, CountryName: r.CountryName, City: r.City, Status: status, LoadPercent: load, Protocols: protocols, Available: available})
	}
	autoList := []string{}
	for _, protocol := range []string{"vless", "trojan"} {
		if autoProtocols[protocol] {
			autoList = append(autoList, protocol)
		}
	}
	autoStatus := "offline"
	if len(autoList) > 0 {
		autoStatus = "online"
	}
	result = append([]MobileServerView{{ServerID: "auto", CountryCode: "AUTO", CountryName: "Automatic", City: "", Status: autoStatus, LoadPercent: 0, Protocols: autoList, Recommended: true, Available: len(autoList) > 0}}, result...)
	version := strconv.FormatInt(latest.Unix(), 10)
	if latest.IsZero() {
		version = "0"
	}
	return version, result, nil
}

func (s *MobileService) Bootstrap(identity MobileIdentity) (map[string]any, error) {
	user, err := s.repo.GetUser(identity.UserID)
	if err != nil {
		return nil, err
	}
	tg, err := s.repo.GetTelegram(identity.UserID)
	if err != nil {
		return nil, err
	}
	device, err := s.repo.GetDevice(identity.DeviceID)
	if err != nil {
		return nil, err
	}
	sub, err := s.Subscription(identity.UserID)
	if err != nil {
		return nil, err
	}
	version, _, err := s.Servers(identity.UserID)
	if err != nil {
		return nil, err
	}
	details, err := s.vpn.GetUserVPNDetails(identity.UserID)
	if err != nil {
		return nil, err
	}
	status := "missing"
	revision := "0"
	protocols := []string{}
	lastServer := any(nil)
	for _, p := range details.Profiles {
		if p.DeviceID != identity.DeviceID {
			continue
		}
		status = mobileVPNStatus(p.Status)
		protocols = append(protocols, p.Protocol)
		if p.UpdatedAt != nil {
			revision = strconv.FormatInt(p.UpdatedAt.UnixNano(), 10)
		}
		for _, n := range p.Nodes {
			if n.Status == models.VPNProfileNodeStatusSuccess {
				lastServer = n.ServerID
			}
		}
	}
	connectivity := s.cfg.ConnectivityCheckURL
	if connectivity == "" && s.cfg.PublicBaseURL != "" {
		connectivity = s.cfg.PublicBaseURL + "/api/mobile/v1/connectivity-check"
	}
	return map[string]any{"user": map[string]any{"id": strconv.FormatUint(uint64(user.ID), 10), "telegram_id": tg.TgID, "username": tg.Username, "status": map[bool]string{true: "active", false: "blocked"}[user.Status]}, "subscription": sub, "current_device": map[string]any{"id": device.ID, "device_name": device.DeviceName, "platform": device.Platform, "status": device.Status, "current": true, "last_seen_at": device.LastSeenAt}, "vpn": map[string]any{"status": status, "config_revision": revision, "supported_protocols": protocols, "last_server_id": lastServer}, "capabilities": map[string]any{"min_app_version": s.cfg.MinAppVersion, "force_update": false, "connectivity_check_url": connectivity, "payments_url": nullableString(s.cfg.PaymentsURL), "support_url": nullableString(s.cfg.SupportURL), "privacy_url": nullableString(s.cfg.PrivacyURL)}, "server_catalog_version": version}, nil
}

func (s *MobileService) Connect(identity MobileIdentity, idempotencyKey string, input MobileConnectInput) (MobileOperationView, int, error) {
	if _, err := uuid.Parse(idempotencyKey); err != nil {
		return MobileOperationView{}, 0, mobileErr("INVALID_REQUEST", "Idempotency-Key must be a UUID")
	}
	if identity.DeviceID != input.DeviceID {
		return MobileOperationView{}, 0, mobileErr("DEVICE_REVOKED", "device does not match access token")
	}
	if input.Protocol != "vless" && input.Protocol != "trojan" {
		return MobileOperationView{}, 0, mobileErr("PROTOCOL_UNAVAILABLE", "protocol is unavailable")
	}
	if input.ServerID == "auto" && input.Mode != "auto" {
		return MobileOperationView{}, 0, mobileErr("INVALID_REQUEST", "auto server requires auto mode")
	}
	if err := s.validateEntitlement(identity); err != nil {
		return MobileOperationView{}, 0, err
	}
	body, _ := json.Marshal(input)
	requestHash := tokenHash(string(body))
	if existing, err := s.repo.GetIdempotency(identity.DeviceID, idempotencyKey); err == nil {
		if existing.RequestHash != requestHash {
			return MobileOperationView{}, 0, mobileErr("INVALID_REQUEST", "idempotency key was used for another request")
		}
		view, err := s.Operation(identity, existing.OperationID)
		if err != nil {
			return MobileOperationView{}, 0, err
		}
		if view.Status == "success" {
			return view, 200, nil
		}
		return view, 202, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return MobileOperationView{}, 0, err
	}
	now := s.now().UTC()
	op := models.MobileOperation{ID: uuid.NewString(), UserID: identity.UserID, DeviceID: identity.DeviceID, ServerID: input.ServerID, Protocol: input.Protocol, Status: "pending", ExpiresAt: now.Add(30 * time.Minute)}
	record := models.MobileIdempotencyRecord{DeviceID: identity.DeviceID, Key: idempotencyKey, RequestHash: requestHash, OperationID: op.ID, ExpiresAt: now.Add(24 * time.Hour)}
	if err := s.repo.CreateOperationWithIdempotency(op, record); err != nil {
		existing, lookupErr := s.repo.GetIdempotency(identity.DeviceID, idempotencyKey)
		if lookupErr == nil && existing.RequestHash == requestHash {
			view, viewErr := s.Operation(identity, existing.OperationID)
			if viewErr != nil {
				return MobileOperationView{}, 0, viewErr
			}
			return view, 202, nil
		}
		return MobileOperationView{}, 0, err
	}
	deviceID := identity.DeviceID
	result, err := s.vpn.RequestCreateVPN(RequestCreateVPNInput{TgID: mustTelegramID(s.repo, identity.UserID), Protocol: input.Protocol, DeviceID: &deviceID, ServerID: input.ServerID})
	if err != nil {
		code := "PROVISIONING_FAILED"
		if errors.Is(err, ErrNoMatchingServers) {
			code = "ROUTE_UNAVAILABLE"
		}
		_ = s.repo.UpdateOperation(op.ID, map[string]any{"status": "failed", "last_error_code": code, "last_error": "VPN provisioning failed"})
		return MobileOperationView{}, 0, mobileErr(code, "VPN provisioning failed")
	}
	resolved := input.ServerID
	if v := result.ResolvedServers[input.Protocol]; v != "" {
		resolved = v
	}
	_ = s.repo.UpdateOperation(op.ID, map[string]any{"resolved_server": resolved, "status": "processing"})
	view, err := s.Operation(identity, op.ID)
	if err != nil {
		return MobileOperationView{}, 0, err
	}
	if view.Status == "success" {
		return view, 200, nil
	}
	return view, 202, nil
}

func (s *MobileService) Operation(identity MobileIdentity, operationID string) (MobileOperationView, error) {
	op, err := s.repo.GetOperation(operationID)
	if err != nil {
		return MobileOperationView{}, err
	}
	if op.UserID != identity.UserID || op.DeviceID != identity.DeviceID {
		return MobileOperationView{}, mobileErr("UNAUTHORIZED", "operation does not belong to session")
	}
	if !op.ExpiresAt.After(s.now().UTC()) {
		return MobileOperationView{OperationID: op.ID, Status: "expired", RetryAfterSeconds: 0}, nil
	}
	if op.Status == "failed" {
		return MobileOperationView{OperationID: op.ID, Status: "failed", RetryAfterSeconds: 0, Error: &MobileOperationError{Code: op.LastErrorCode, Message: op.LastError}}, nil
	}
	config, ok, err := s.findConfig(identity.UserID, identity.DeviceID, op.ResolvedServer, op.Protocol)
	if err != nil {
		return MobileOperationView{}, err
	}
	if ok {
		_ = s.repo.UpdateOperation(op.ID, map[string]any{"status": "success"})
		return MobileOperationView{OperationID: op.ID, Status: "success", RetryAfterSeconds: 0, Result: &config}, nil
	}
	return MobileOperationView{OperationID: op.ID, Status: "processing", RetryAfterSeconds: 2}, nil
}

func (s *MobileService) findConfig(userID uint, deviceID, serverID, protocol string) (MobileVPNConfig, bool, error) {
	details, err := s.vpn.GetUserVPNDetails(userID)
	if err != nil {
		return MobileVPNConfig{}, false, err
	}
	for _, p := range details.Profiles {
		if p.DeviceID != deviceID || p.Protocol != protocol {
			continue
		}
		for _, n := range p.Nodes {
			if n.ServerID == serverID && n.Status == models.VPNProfileNodeStatusSuccess && n.ConfigLink != "" {
				revision := strconv.FormatInt(n.UpdatedAt.UnixNano(), 10)
				return MobileVPNConfig{ConnectionID: strconv.FormatUint(uint64(n.ID), 10), ServerID: serverID, Protocol: protocol, ConfigURL: n.ConfigLink, ConfigRevision: revision, ValidUntil: details.Subscription.ExpiresAt}, true, nil
			}
		}
	}
	return MobileVPNConfig{}, false, nil
}

func (s *MobileService) validateEntitlement(identity MobileIdentity) error {
	user, err := s.repo.GetUser(identity.UserID)
	if err != nil {
		return err
	}
	if !user.Status {
		return mobileErr("USER_BLOCKED", "user is blocked")
	}
	device, err := s.repo.GetDevice(identity.DeviceID)
	if err != nil || device.Status != models.MobileDeviceStatusActive {
		return mobileErr("DEVICE_REVOKED", "device is not active")
	}
	sub, err := s.vpnRepo.GetOrCreateSubscription(identity.UserID)
	if err != nil {
		return err
	}
	if sub.Status != models.SubscriptionStatusActive && sub.Status != models.SubscriptionStatusTrial {
		return mobileErr("SUBSCRIPTION_INACTIVE", "subscription is inactive")
	}
	if sub.ExpiresAt != nil && !sub.ExpiresAt.After(s.now()) {
		return mobileErr("SUBSCRIPTION_INACTIVE", "subscription is expired")
	}
	return nil
}

func (s *MobileService) Cleanup() error              { return s.repo.Cleanup(s.now().UTC()) }
func (s *MobileService) Config() config.MobileConfig { return s.cfg }

func (s *MobileService) issueAccess(identity MobileIdentity) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(map[string]any{"sub": identity.UserID, "device_id": identity.DeviceID, "sid": identity.SessionID, "iat": s.now().Unix(), "exp": s.now().Add(time.Duration(s.cfg.AccessTTL) * time.Second).Unix()})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (s *MobileService) parseAccess(token string) (MobileIdentity, time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return MobileIdentity{}, time.Time{}, errors.New("invalid JWT")
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return MobileIdentity{}, time.Time{}, errors.New("invalid JWT signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return MobileIdentity{}, time.Time{}, err
	}
	var claims struct {
		Sub      uint   `json:"sub"`
		DeviceID string `json:"device_id"`
		SID      string `json:"sid"`
		Exp      int64  `json:"exp"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return MobileIdentity{}, time.Time{}, err
	}
	return MobileIdentity{UserID: claims.Sub, DeviceID: claims.DeviceID, SessionID: claims.SID}, time.Unix(claims.Exp, 0), nil
}
func randomToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
func verifyPKCE(verifier, challenge string) bool {
	return hmac.Equal([]byte(tokenHash(verifier)), []byte(challenge))
}
func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func mobileServerStatus(status string) string {
	switch status {
	case models.ServerStatusOnline:
		return "online"
	case models.ServerStatusDegraded, models.ServerStatusStale:
		return "degraded"
	case models.ServerStatusDisabled:
		return "maintenance"
	default:
		return "offline"
	}
}

func mobileVPNStatus(status string) string {
	switch status {
	case models.VPNProfileStatusPending, models.VPNProfileStatusActive, models.VPNProfileStatusPartial, models.VPNProfileStatusFailed:
		return status
	default:
		return "failed"
	}
}
func mustTelegramID(repo *repository.MobileRepo, userID uint) int64 {
	tg, err := repo.GetTelegram(userID)
	if err != nil {
		return 0
	}
	return tg.TgID
}
