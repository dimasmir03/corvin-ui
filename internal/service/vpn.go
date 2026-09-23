package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/jobsvc"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrUnsupportedProtocol  = errors.New("unsupported protocol")
	ErrNoMatchingServers    = errors.New("no matching servers for vpn profile")
	ErrNoCommandsQueued     = errors.New("vpn create commands were not queued")
	ErrInvalidSubscription  = errors.New("invalid subscription")
	ErrInvalidServerAccess  = errors.New("invalid server access")
	ErrSubscriptionInactive = errors.New("subscription is inactive")
	ErrDeviceRevoked        = errors.New("device is not active")
)

type VPNErrorKind string

const (
	VPNErrorKindBroker VPNErrorKind = "broker"
)

type VPNFlowError struct {
	Kind VPNErrorKind
	Err  error
}

func (e *VPNFlowError) Error() string {
	return e.Err.Error()
}

func (e *VPNFlowError) Unwrap() error {
	return e.Err
}

func IsVPNFlowError(err error, kind VPNErrorKind) bool {
	var flowErr *VPNFlowError
	return errors.As(err, &flowErr) && flowErr.Kind == kind
}

type RequestCreateVPNInput struct {
	TgID     int64
	Protocol string
	DeviceID *string
	ServerID string
}

type RequestCreateVPNResult struct {
	TgID              int64
	Protocol          string
	CommandID         uint
	CommandsCount     int
	Status            string
	FinalLink         string
	RequestedServerID string
	ResolvedServers   map[string]string
}

type UpdateSubscriptionInput struct {
	Status      string
	TariffName  string
	ExpiresAt   *time.Time
	DeviceLimit int
}

type SetServerAccessInput struct {
	Enabled     bool
	AllowVLESS  bool
	AllowTrojan bool
	ValidUntil  *time.Time
}

type ConnectionActionResult struct {
	ConnectionID uint   `json:"connection_id"`
	ServerID     string `json:"server_id"`
	DesiredState string `json:"desired_state"`
	Action       string `json:"action"`
}

type VPNReadyNotification struct {
	TgID     int64
	Protocol string
	Link     string
}

type UserVPNDetails struct {
	Client       *UserVPNClientView      `json:"client"`
	Clients      []UserVPNClientView     `json:"clients"`
	Profiles     []UserVPNProfileView    `json:"profiles"`
	Subscription models.UserSubscription `json:"subscription"`
	Devices      []models.MobileDevice   `json:"devices"`
	Servers      []UserVPNServerView     `json:"servers"`
}

type UserVPNClientView struct {
	ID         uint   `json:"id"`
	DeviceID   string `json:"device_id,omitempty"`
	ClientCode string `json:"client_code"`
	Email      string `json:"email"`
}

type UserVPNProfileView struct {
	ID            uint                     `json:"id,omitempty"`
	VPNClientID   uint                     `json:"vpn_client_id,omitempty"`
	DeviceID      string                   `json:"device_id,omitempty"`
	Exists        bool                     `json:"exists"`
	Profile       string                   `json:"profile"`
	EndpointGroup string                   `json:"endpoint_group"`
	Protocol      string                   `json:"protocol"`
	Status        string                   `json:"status,omitempty"`
	FinalLink     string                   `json:"final_link,omitempty"`
	LastError     string                   `json:"last_error,omitempty"`
	CreatedAt     *time.Time               `json:"created_at,omitempty"`
	UpdatedAt     *time.Time               `json:"updated_at,omitempty"`
	Nodes         []UserVPNProfileNodeView `json:"nodes"`
}

type UserVPNServerView struct {
	ServerID         string     `json:"server_id"`
	DisplayName      string     `json:"display_name"`
	EndpointGroup    string     `json:"endpoint_group"`
	Protocol         string     `json:"protocol"`
	CountryCode      string     `json:"country_code,omitempty"`
	CountryName      string     `json:"country_name,omitempty"`
	City             string     `json:"city,omitempty"`
	ServerEnabled    bool       `json:"server_enabled"`
	AccessConfigured bool       `json:"access_configured"`
	Enabled          bool       `json:"enabled"`
	AllowVLESS       bool       `json:"allow_vless"`
	AllowTrojan      bool       `json:"allow_trojan"`
	ValidUntil       *time.Time `json:"valid_until,omitempty"`
}

type UserVPNProfileNodeView struct {
	ID            uint       `json:"id"`
	ServerID      string     `json:"server_id"`
	NodeID        string     `json:"node_id,omitempty"`
	Status        string     `json:"status"`
	DesiredState  string     `json:"desired_state"`
	PendingAction string     `json:"pending_action,omitempty"`
	Protocol      string     `json:"protocol"`
	ConfigLink    string     `json:"config_link,omitempty"`
	InboundID     *int       `json:"inbound_id,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	AppliedAt     *time.Time `json:"applied_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type LinkProfileView struct {
	Profile       string
	EndpointGroup string
	Protocol      string
	Status        string
	Exists        bool
	Usable        bool
	FinalLink     string
	Reason        string
	Source        string
}

type LinkOverviewResult struct {
	TgID       int64
	UserID     uint
	ClientID   uint
	ClientCode string
	Profiles   map[string]LinkProfileView
	Reason     string
}

type ProtocolLinkResult struct {
	TgID     int64
	Protocol string
	Status   string
	Exists   bool
	Usable   bool
	Link     string
	Reason   string
}

type VPNService struct {
	vpnRepo      *repository.VpnRepo
	telegramRepo *repository.TelegramRepo
	publisher    VPNCommandPublisher
}

type VPNCommandPublisher interface {
	PublishJob(msg broker.JobTask) error
}

func NewVPNService(
	vpnRepo *repository.VpnRepo,
	telegramRepo *repository.TelegramRepo,
	publisher VPNCommandPublisher,
) *VPNService {
	return &VPNService{
		vpnRepo:      vpnRepo,
		telegramRepo: telegramRepo,
		publisher:    publisher,
	}
}

func (s *VPNService) CanPublishCommands() bool {
	return s != nil && s.publisher != nil
}

func (s *VPNService) UpdateUserSubscription(userID uint, input UpdateSubscriptionInput) (models.UserSubscription, error) {
	status := strings.ToLower(strings.TrimSpace(input.Status))
	switch status {
	case models.SubscriptionStatusActive, models.SubscriptionStatusTrial, models.SubscriptionStatusExpired, models.SubscriptionStatusBlocked, models.SubscriptionStatusNone:
	default:
		return models.UserSubscription{}, ErrInvalidSubscription
	}
	if input.DeviceLimit < 0 || input.DeviceLimit > 1000 {
		return models.UserSubscription{}, ErrInvalidSubscription
	}
	tariffName := strings.TrimSpace(input.TariffName)
	if tariffName == "" {
		tariffName = "default"
	}
	if _, err := s.vpnRepo.GetOrCreateSubscription(userID); err != nil {
		return models.UserSubscription{}, err
	}
	return s.vpnRepo.UpdateSubscription(models.UserSubscription{UserID: userID, Status: status, TariffName: tariffName, ExpiresAt: input.ExpiresAt, DeviceLimit: input.DeviceLimit})
}

func (s *VPNService) GetAutoRoutingSettings() (models.VPNRoutingSettings, error) {
	return s.vpnRepo.GetOrCreateRoutingSettings()
}

func (s *VPNService) UpdateAutoRoutingSettings(autoMode, autoServerID string) (models.VPNRoutingSettings, error) {
	autoMode = strings.ToLower(strings.TrimSpace(autoMode))
	autoServerID = strings.TrimSpace(autoServerID)
	if autoMode == "" {
		autoMode = models.AutoServerModeAutomatic
	}
	if autoMode != models.AutoServerModeAutomatic && autoMode != models.AutoServerModePinned {
		return models.VPNRoutingSettings{}, fmt.Errorf("invalid auto mode %q", autoMode)
	}
	if autoMode == models.AutoServerModePinned {
		if autoServerID == "" {
			return models.VPNRoutingSettings{}, fmt.Errorf("auto_server_id is required in pinned mode")
		}
		servers, err := s.vpnRepo.ListRegisteredServers()
		if err != nil {
			return models.VPNRoutingSettings{}, err
		}
		found := false
		for _, server := range servers {
			if server.ServerID == autoServerID && server.Enabled && server.ArchivedAt == nil {
				found = true
				break
			}
		}
		if !found {
			return models.VPNRoutingSettings{}, fmt.Errorf("selected auto server is not enabled")
		}
	} else {
		autoServerID = ""
	}
	return s.vpnRepo.UpdateRoutingSettings(autoMode, autoServerID)
}

func (s *VPNService) SetUserServerAccess(userID uint, serverID string, input SetServerAccessInput) (models.UserServerAccess, error) {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return models.UserServerAccess{}, ErrInvalidServerAccess
	}
	servers, err := s.vpnRepo.ListRegisteredServers()
	if err != nil {
		return models.UserServerAccess{}, err
	}
	found := false
	for _, server := range servers {
		if server.ServerID == serverID {
			found = true
			break
		}
	}
	if !found {
		return models.UserServerAccess{}, ErrInvalidServerAccess
	}
	return s.vpnRepo.SetServerAccess(models.UserServerAccess{UserID: userID, ServerID: serverID, Enabled: input.Enabled, AllowVLESS: input.AllowVLESS, AllowTrojan: input.AllowTrojan, ValidUntil: input.ValidUntil})
}

func (s *VPNService) ProvisionUserServer(userID uint, deviceID *string, serverID, protocol string) (*RequestCreateVPNResult, error) {
	telegram, err := s.telegramRepo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}
	return s.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, DeviceID: deviceID, ServerID: strings.TrimSpace(serverID), Protocol: protocol})
}

func (s *VPNService) RequestConnectionState(userID, connectionID uint, desiredState string) (ConnectionActionResult, error) {
	desiredState = strings.ToLower(strings.TrimSpace(desiredState))
	action := ""
	switch desiredState {
	case models.VPNConnectionDesiredEnabled:
		action = jobsvc.ActionEnableClient
	case models.VPNConnectionDesiredDisabled:
		action = jobsvc.ActionDisableClient
	case models.VPNConnectionDesiredDeleted:
		action = jobsvc.ActionDeleteClient
	default:
		return ConnectionActionResult{}, fmt.Errorf("invalid desired connection state %q", desiredState)
	}
	if s.publisher == nil {
		return ConnectionActionResult{}, errors.New("vpn command publisher is not configured")
	}
	node, err := s.vpnRepo.GetProfileNodeByID(connectionID)
	if err != nil {
		return ConnectionActionResult{}, err
	}
	profile, err := s.vpnRepo.GetProfileByID(node.VPNProfileID)
	if err != nil {
		return ConnectionActionResult{}, err
	}
	if profile.VPNClient.UserID != userID {
		return ConnectionActionResult{}, gorm.ErrRecordNotFound
	}
	group, err := s.vpnRepo.GetEndpointGroup(profile.EndpointGroup)
	if err != nil {
		return ConnectionActionResult{}, err
	}
	node, err = s.vpnRepo.PrepareProfileNodeAction(node.ID, desiredState, action)
	if err != nil {
		return ConnectionActionResult{}, err
	}
	command := profileNodeCommand(node, profile, group)
	attemptedAt := time.Now().UTC()
	nextAttemptAt := attemptedAt.Add(provisioningRetryDelay(node.Attempts))
	if err := s.publisher.PublishJob(command); err != nil {
		_ = s.vpnRepo.MarkProfileNodePublishFailed(node.ID, err.Error(), attemptedAt, nextAttemptAt)
		return ConnectionActionResult{}, &VPNFlowError{Kind: VPNErrorKindBroker, Err: err}
	}
	if err := s.vpnRepo.MarkProfileNodePublished(node.ID, attemptedAt, nextAttemptAt); err != nil {
		return ConnectionActionResult{}, err
	}
	return ConnectionActionResult{ConnectionID: node.ID, ServerID: command.ServerID, DesiredState: desiredState, Action: action}, nil
}

func (s *VPNService) GetUserVPNDetails(userID uint) (UserVPNDetails, error) {
	details := UserVPNDetails{Clients: []UserVPNClientView{}, Profiles: []UserVPNProfileView{}, Devices: []models.MobileDevice{}, Servers: []UserVPNServerView{}}
	subscription, err := s.vpnRepo.GetOrCreateSubscription(userID)
	if err != nil {
		return UserVPNDetails{}, err
	}
	details.Subscription = subscription
	details.Devices, err = s.vpnRepo.ListDevicesByUserID(userID)
	if err != nil {
		return UserVPNDetails{}, err
	}
	servers, err := s.vpnRepo.ListRegisteredServers()
	if err != nil {
		return UserVPNDetails{}, err
	}
	access, err := s.vpnRepo.ListServerAccessByUserID(userID)
	if err != nil {
		return UserVPNDetails{}, err
	}
	accessByServer := make(map[string]models.UserServerAccess, len(access))
	for _, item := range access {
		accessByServer[item.ServerID] = item
	}
	legacyAllowAll := len(access) == 0
	serverByID := make(map[string]models.ServerRegistry, len(servers))
	for _, server := range servers {
		serverByID[server.ServerID] = server
		view := UserVPNServerView{ServerID: server.ServerID, DisplayName: server.DisplayName, EndpointGroup: server.EndpointGroup, Protocol: server.ExpectedProtocol, CountryCode: server.CountryCode, CountryName: server.CountryName, City: server.City, ServerEnabled: server.Enabled}
		if item, ok := accessByServer[server.ServerID]; ok {
			view.AccessConfigured = true
			view.Enabled = item.Enabled
			view.AllowVLESS = item.AllowVLESS
			view.AllowTrojan = item.AllowTrojan
			view.ValidUntil = item.ValidUntil
		} else if legacyAllowAll {
			view.Enabled = server.Enabled
			view.AllowVLESS = true
			view.AllowTrojan = true
		}
		details.Servers = append(details.Servers, view)
	}

	clients, err := s.vpnRepo.ListVPNClientsByUserID(userID)
	if err != nil {
		return UserVPNDetails{}, err
	}
	for _, client := range clients {
		clientView := UserVPNClientView{ID: client.ID, ClientCode: client.ClientCode, Email: client.Email}
		if client.DeviceID != nil {
			clientView.DeviceID = *client.DeviceID
		}
		details.Clients = append(details.Clients, clientView)
		if details.Client == nil {
			copyView := clientView
			details.Client = &copyView
		}
		profiles, err := s.vpnRepo.ListProfilesByClientID(client.ID)
		if err != nil {
			return UserVPNDetails{}, err
		}
		for _, profile := range profiles {
			group, _ := s.vpnRepo.GetEndpointGroup(profile.EndpointGroup)
			createdAt := profile.CreatedAt
			updatedAt := profile.UpdatedAt
			view := UserVPNProfileView{
				ID:            profile.ID,
				VPNClientID:   client.ID,
				DeviceID:      clientView.DeviceID,
				Exists:        true,
				Profile:       profile.Profile,
				EndpointGroup: profile.EndpointGroup,
				Protocol:      profile.Protocol,
				Status:        profile.Status,
				FinalLink:     profile.FinalLink,
				LastError:     profile.LastError,
				CreatedAt:     &createdAt,
				UpdatedAt:     &updatedAt,
				Nodes:         make([]UserVPNProfileNodeView, 0, len(profile.Nodes)),
			}
			for _, node := range profile.Nodes {
				serverID := node.ServerID
				if serverID == "" {
					serverID = node.NodeID
				}
				configLink := ""
				if server, ok := serverByID[serverID]; ok && node.Status == models.VPNProfileNodeStatusSuccess {
					configLink = buildServerProfileLink(server, group, client, profile.Profile)
				}
				view.Nodes = append(view.Nodes, UserVPNProfileNodeView{
					ID:            node.ID,
					ServerID:      serverID,
					NodeID:        node.NodeID,
					Status:        node.Status,
					DesiredState:  node.DesiredState,
					PendingAction: node.PendingAction,
					Protocol:      node.Protocol,
					ConfigLink:    configLink,
					InboundID:     node.InboundID,
					LastError:     node.LastError,
					AppliedAt:     node.AppliedAt,
					UpdatedAt:     node.UpdatedAt,
				})
			}
			details.Profiles = append(details.Profiles, view)
		}
	}
	if len(details.Profiles) == 0 {
		details.Profiles = defaultUserVPNProfileViews()
	}
	return details, nil
}

func defaultUserVPNProfileViews() []UserVPNProfileView {
	return []UserVPNProfileView{
		{Profile: jobsvc.VPNProfileVLESS, EndpointGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Nodes: []UserVPNProfileNodeView{}},
		{Profile: jobsvc.VPNProfileTrojan, EndpointGroup: jobsvc.EndpointGroupRU, Protocol: jobsvc.VPNProfileTrojan, Nodes: []UserVPNProfileNodeView{}},
	}
}

func (s *VPNService) RequestCreateVPN(input RequestCreateVPNInput) (*RequestCreateVPNResult, error) {
	profiles, err := requestedVPNProfiles(input.Protocol)
	if err != nil {
		logger.Warn("vpn create rejected", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "protocol", input.Protocol, "reason", "unsupported_protocol")
		return nil, err
	}

	logger.Info("vpn create requested", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "profile", strings.Join(profiles, ","))
	logger.Info("telegram user lookup started", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID)
	telegram, err := s.telegramRepo.GetTelegramByTgID(input.TgID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("telegram user lookup failed", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "reason", "telegram_user_not_found")
		} else {
			logger.Error("telegram user lookup failed", err, "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "reason", "db_error")
		}
		return nil, err
	}
	logger.Info("telegram user found", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "user_id", telegram.UserID)
	subscription, err := s.vpnRepo.GetOrCreateSubscription(telegram.UserID)
	if err != nil {
		return nil, err
	}
	if subscription.Status != models.SubscriptionStatusActive && subscription.Status != models.SubscriptionStatusTrial {
		return nil, ErrSubscriptionInactive
	}
	if subscription.ExpiresAt != nil && !subscription.ExpiresAt.After(time.Now()) {
		return nil, ErrSubscriptionInactive
	}
	if input.DeviceID != nil && strings.TrimSpace(*input.DeviceID) != "" {
		devices, err := s.vpnRepo.ListDevicesByUserID(telegram.UserID)
		if err != nil {
			return nil, err
		}
		deviceActive := false
		for _, device := range devices {
			if device.ID == strings.TrimSpace(*input.DeviceID) && device.Status == models.MobileDeviceStatusActive {
				deviceActive = true
				break
			}
		}
		if !deviceActive {
			return nil, ErrDeviceRevoked
		}
	}

	logger.Info("vpn client lookup started", "component", "vpn_service", "operation", "request_create_vpn", "user_id", telegram.UserID, "telegram_id", input.TgID)
	client, created, err := s.vpnRepo.GetOrCreateVPNClientForDevice(telegram.UserID, input.TgID, input.DeviceID)
	if err != nil {
		logger.Error("vpn client lookup failed", err, "component", "vpn_service", "operation", "request_create_vpn", "user_id", telegram.UserID, "telegram_id", input.TgID, "reason", "db_error")
		return nil, err
	}
	if created {
		logger.Info("vpn client created", "component", "vpn_service", "operation", "request_create_vpn", "user_id", telegram.UserID, "telegram_id", input.TgID, "client_code", client.ClientCode)
	} else {
		logger.Info("vpn client reused", "component", "vpn_service", "operation", "request_create_vpn", "user_id", telegram.UserID, "telegram_id", input.TgID, "client_code", client.ClientCode)
	}

	result := &RequestCreateVPNResult{TgID: input.TgID, Protocol: strings.Join(profiles, ","), RequestedServerID: strings.TrimSpace(input.ServerID), ResolvedServers: map[string]string{}}
	activeProfiles := 0
	for _, profileName := range profiles {
		targetServerID := strings.TrimSpace(input.ServerID)
		if strings.EqualFold(targetServerID, "auto") {
			targetServerID, err = s.resolveAutoServer(telegram.UserID, profileName)
			if err != nil {
				return nil, err
			}
		}
		if targetServerID != "" {
			result.ResolvedServers[profileName] = targetServerID
		}
		profile, commandID, commandsCount, err := s.ensureVPNProfile(telegram.UserID, input.TgID, client, profileName, targetServerID)
		if err != nil {
			return nil, err
		}
		if result.CommandID == 0 {
			result.CommandID = commandID
		}
		result.CommandsCount += commandsCount
		if len(profiles) == 1 {
			result.Protocol = profile.Profile
			result.Status = profile.Status
			result.FinalLink = profile.FinalLink
		}
		if profile.Status == models.VPNProfileStatusActive && strings.TrimSpace(profile.FinalLink) != "" {
			activeProfiles++
		}
	}
	if result.CommandsCount == 0 {
		if activeProfiles == len(profiles) {
			logger.Info("vpn create request returned existing link", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "profile", result.Protocol, "reason", "profile_already_active")
			return result, nil
		}
		logger.Warn("vpn create request not queued", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "profile", result.Protocol, "command_id", result.CommandID, "commands_count", result.CommandsCount, "reason", "no_commands_published")
		return nil, ErrNoCommandsQueued
	}
	if result.CommandID == 0 {
		logger.Warn("vpn create request not queued", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "profile", result.Protocol, "command_id", result.CommandID, "commands_count", result.CommandsCount, "reason", "invalid_command_id")
		return nil, ErrNoCommandsQueued
	}
	logger.Info("vpn create request queued", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "profile", result.Protocol, "command_id", result.CommandID, "commands_count", result.CommandsCount)
	return result, nil
}

func (s *VPNService) resolveAutoServer(userID uint, profileName string) (string, error) {
	settings, err := s.vpnRepo.GetOrCreateRoutingSettings()
	if err != nil {
		return "", err
	}
	groupCode := endpointGroupForVPNProfile(profileName)
	group, err := s.vpnRepo.GetOrCreateEndpointGroup(groupCode)
	if err != nil {
		return "", err
	}
	protocol := strings.TrimSpace(group.Protocol)
	if protocol == "" {
		protocol = profileName
	}
	nodes, err := s.vpnRepo.EnabledNodesByGroupForUser(userID, groupCode, protocol, "")
	if err != nil {
		return "", err
	}
	if settings.AutoMode == models.AutoServerModePinned {
		for _, node := range nodes {
			if effectiveNodeStateServerID(node) == settings.AutoServerID {
				return settings.AutoServerID, nil
			}
		}
		return "", ErrNoMatchingServers
	}
	if len(nodes) == 0 {
		return "", ErrNoMatchingServers
	}
	best := nodes[0]
	for _, candidate := range nodes[1:] {
		if healthierNode(candidate, best) {
			best = candidate
		}
	}
	return effectiveNodeStateServerID(best), nil
}

func effectiveNodeStateServerID(node models.NodeState) string {
	if strings.TrimSpace(node.ServerID) != "" {
		return strings.TrimSpace(node.ServerID)
	}
	return strings.TrimSpace(node.NodeID)
}

func healthierNode(left, right models.NodeState) bool {
	leftScore := nodeHealthScore(left)
	rightScore := nodeHealthScore(right)
	if leftScore != rightScore {
		return leftScore > rightScore
	}
	if left.OnlineCount != right.OnlineCount {
		return left.OnlineCount < right.OnlineCount
	}
	if !left.LastSeenAt.Equal(right.LastSeenAt) {
		return left.LastSeenAt.After(right.LastSeenAt)
	}
	return effectiveNodeStateServerID(left) < effectiveNodeStateServerID(right)
}

func nodeHealthScore(node models.NodeState) int {
	score := 0
	switch node.Status {
	case models.ServerStatusOnline:
		score += 100
	case models.ServerStatusDegraded:
		score += 50
	case models.ServerStatusStale:
		score += 20
	}
	if node.AgentAlive {
		score += 20
	}
	if node.XUIAvailable != nil && *node.XUIAvailable {
		score += 20
	}
	return score
}

func requestedVPNProfiles(protocol string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", "all":
		return []string{jobsvc.VPNProfileVLESS, jobsvc.VPNProfileTrojan}, nil
	case jobsvc.VPNProfileVLESS:
		return []string{jobsvc.VPNProfileVLESS}, nil
	case jobsvc.VPNProfileTrojan:
		return []string{jobsvc.VPNProfileTrojan}, nil
	default:
		return nil, ErrUnsupportedProtocol
	}
}

func (s *VPNService) ensureVPNProfile(userID uint, tgID int64, client models.VPNClient, profileName, onlyServerID string) (models.VPNProfile, uint, int, error) {
	endpointGroup := endpointGroupForVPNProfile(profileName)
	expectedProtocol := profileName
	logger.Info("vpn profile lookup started", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "protocol", expectedProtocol)
	group, err := s.vpnRepo.GetOrCreateEndpointGroup(endpointGroup)
	if err != nil {
		logger.Error("vpn endpoint group lookup failed", err, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup)
		return models.VPNProfile{}, 0, 0, err
	}
	if strings.TrimSpace(group.Protocol) != "" {
		expectedProtocol = strings.TrimSpace(group.Protocol)
	}

	existing, err := s.vpnRepo.GetProfile(client.ID, profileName)
	profileCreated := false
	var profile models.VPNProfile
	if err == nil {
		profile = existing
		logger.Info("vpn profile reused", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "profile_id", profile.ID, "status", profile.Status)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Info("vpn profile not found", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "reason", "vpn_profiles_not_found")
		profile = models.VPNProfile{
			VPNClientID:   client.ID,
			Profile:       profileName,
			EndpointGroup: endpointGroup,
			Protocol:      expectedProtocol,
			Status:        models.VPNProfileStatusPending,
			FinalLink:     buildProfileLink(group, client, profileName),
		}
		profile, err = s.vpnRepo.CreateProfileWithNodes(profile, nil)
		if err != nil {
			logger.Error("vpn profile create failed", err, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup)
			return models.VPNProfile{}, 0, 0, err
		}
		profileCreated = true
		logger.Info("vpn profile created", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "status", profile.Status)
	} else {
		logger.Error("vpn profile lookup failed", err, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "reason", "db_error")
		return models.VPNProfile{}, 0, 0, err
	}
	// Legacy Telegram calls without an explicit server keep their idempotent
	// "already active" behaviour. Server-specific admin/mobile calls continue
	// below so a newly granted server can be provisioned onto an active profile.
	if profile.Status == models.VPNProfileStatusActive && strings.TrimSpace(profile.FinalLink) != "" && strings.TrimSpace(onlyServerID) == "" {
		return profile, 0, 0, nil
	}

	logger.Info("vpn provisioning rebuild started", "component", "vpn_service", "operation", "ensure_vpn_profile", "profile_id", profile.ID, "profile", profileName, "reason", "profile_not_active")
	logger.Info("vpn target servers lookup started", "component", "vpn_service", "operation", "ensure_vpn_profile", "endpoint_group", endpointGroup, "expected_protocol", expectedProtocol)
	nodes, err := s.vpnRepo.EnabledNodesByGroupForUser(userID, endpointGroup, expectedProtocol, onlyServerID)
	if err != nil {
		logger.Error("vpn target servers lookup failed", err, "component", "vpn_service", "operation", "ensure_vpn_profile", "profile_id", profile.ID, "endpoint_group", endpointGroup, "expected_protocol", expectedProtocol)
		return models.VPNProfile{}, 0, 0, err
	}
	logger.Info("vpn target servers found", "component", "vpn_service", "operation", "ensure_vpn_profile", "endpoint_group", endpointGroup, "expected_protocol", expectedProtocol, "count", len(nodes))

	if len(nodes) == 0 {
		lastError := "no_matching_servers"
		_ = s.vpnRepo.UpdateProfileStatus(profile.ID, models.VPNProfileStatusFailed, lastError)
		logger.Warn("vpn profile has no matching servers", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "expected_protocol", expectedProtocol, "profile_id", profile.ID, "reason", lastError)
		profile.Status = models.VPNProfileStatusFailed
		profile.LastError = lastError
		return profile, 0, 0, ErrNoMatchingServers
	}

	profile, _, err = s.vpnRepo.EnsureProfileNodes(profile, nodes)
	if err != nil {
		logger.Error("vpn profile nodes ensure failed", err, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID)
		return models.VPNProfile{}, 0, 0, err
	}
	for _, node := range profile.Nodes {
		logger.Info("vpn profile node pending", "component", "vpn_service", "operation", "ensure_vpn_profile", "profile_id", profile.ID, "server_id", node.ServerID, "profile", profileName, "endpoint_group", endpointGroup)
	}
	if profileCreated && len(profile.Nodes) == 0 {
		logger.Warn("vpn profile created without nodes", "component", "vpn_service", "operation", "ensure_vpn_profile", "profile_id", profile.ID, "profile", profileName, "reason", "no_profile_nodes_created")
	}

	if s.publisher == nil {
		logger.Error("vpn command publish failed", nil, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "reason", "publisher_not_configured")
		return models.VPNProfile{}, 0, 0, errors.New("vpn command publisher is not configured")
	}

	pendingNodes := pendingProfileNodesForTargets(profile.Nodes, serverIDsFromNodeStates(nodes))
	if len(pendingNodes) == 0 {
		logger.Warn("vpn command publish skipped", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "status", profile.Status, "reason", "profile_has_no_pending_targets")
		return profile, 0, 0, ErrNoCommandsQueued
	}

	logger.Info("vpn commands publish started", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "nodes_count", len(profile.Nodes), "commands_count", len(pendingNodes))
	firstCommandID, publishedCount, publishErr := s.publishProfileNodes(profile, group, pendingNodes)
	if publishedCount == 0 {
		if publishErr == nil {
			publishErr = ErrNoCommandsQueued
		}
		_ = s.vpnRepo.TouchProfilePublishError(profile.ID, publishErr.Error())
		logger.Error("vpn commands publish failed", publishErr, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "nodes_count", len(profile.Nodes))
		return models.VPNProfile{}, 0, 0, &VPNFlowError{Kind: VPNErrorKindBroker, Err: publishErr}
	}
	if publishErr != nil {
		logger.Warn("vpn commands partially published", "component", "vpn_service", "operation", "ensure_vpn_profile", "profile_id", profile.ID, "published_count", publishedCount, "commands_count", len(pendingNodes), "error", publishErr.Error())
	}

	if err := s.vpnRepo.UpdateProfileStatus(profile.ID, models.VPNProfileStatusPending, ""); err != nil {
		return models.VPNProfile{}, 0, 0, err
	}
	profile.Status = models.VPNProfileStatusPending
	profile.LastError = ""
	logger.Info("vpn commands published", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "command_id", firstCommandID, "commands_count", publishedCount)
	return profile, firstCommandID, publishedCount, nil
}

func pendingProfileNodesForTargets(nodes []models.VPNProfileNode, targetServerIDs []string) []models.VPNProfileNode {
	targets := make(map[string]struct{}, len(targetServerIDs))
	for _, serverID := range targetServerIDs {
		targets[strings.TrimSpace(serverID)] = struct{}{}
	}
	pending := make([]models.VPNProfileNode, 0, len(nodes))
	for _, node := range nodes {
		serverID := strings.TrimSpace(node.ServerID)
		if serverID == "" {
			serverID = strings.TrimSpace(node.NodeID)
		}
		if _, ok := targets[serverID]; !ok || node.Status == models.VPNProfileNodeStatusSuccess {
			continue
		}
		pending = append(pending, node)
	}
	return pending
}

func (s *VPNService) publishProfileNodes(profile models.VPNProfile, group models.EndpointGroup, nodes []models.VPNProfileNode) (uint, int, error) {
	var firstCommandID uint
	published := 0
	var firstErr error
	for _, node := range nodes {
		command := profileNodeCommand(node, profile, group)
		attemptedAt := time.Now().UTC()
		nextAttemptAt := attemptedAt.Add(provisioningRetryDelay(node.Attempts))
		if err := s.publisher.PublishJob(command); err != nil {
			_ = s.vpnRepo.MarkProfileNodePublishFailed(node.ID, err.Error(), attemptedAt, nextAttemptAt)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := s.vpnRepo.MarkProfileNodePublished(node.ID, attemptedAt, nextAttemptAt); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if firstCommandID == 0 {
			firstCommandID = node.ID
		}
		published++
	}
	return firstCommandID, published, firstErr
}

func createClientCommand(node models.VPNProfileNode, profile models.VPNProfile, group models.EndpointGroup) broker.JobTask {
	serverID := strings.TrimSpace(node.ServerID)
	if serverID == "" {
		serverID = strings.TrimSpace(node.NodeID)
	}
	client := profile.VPNClient
	return broker.JobTask{
		EventType:         jobsvc.ActionCreateClient,
		JobID:             node.ID,
		ServerID:          serverID,
		TargetServerID:    serverID,
		Action:            jobsvc.ActionCreateClient,
		CommandType:       jobsvc.ActionCreateClient,
		Protocol:          profile.Protocol,
		ProfileID:         profile.ID,
		VPNClientID:       client.ID,
		Profile:           profile.Profile,
		TargetGroup:       profile.EndpointGroup,
		TelegramID:        client.TelegramID,
		UserID:            client.UserID,
		ClientCode:        client.ClientCode,
		Email:             client.Email,
		Enable:            true,
		TechnicalClientID: client.ClientCode,
		CreatedAt:         time.Now().UTC(),
		Credentials: broker.VPNCredentials{
			VLESS:  broker.VLESSCredentials{ID: client.VlessUUID, Flow: group.Flow},
			Trojan: broker.TrojanCredentials{Password: client.TrojanPassword},
		},
	}
}

func profileNodeCommand(node models.VPNProfileNode, profile models.VPNProfile, group models.EndpointGroup) broker.JobTask {
	command := createClientCommand(node, profile, group)
	action := strings.TrimSpace(node.PendingAction)
	if action == "" {
		action = jobsvc.ActionCreateClient
	}
	command.EventType = action
	command.Action = action
	command.CommandType = action
	command.Enable = action != jobsvc.ActionDisableClient && action != jobsvc.ActionDeleteClient
	return command
}

func provisioningRetryDelay(attempts int) time.Duration {
	switch {
	case attempts >= 4:
		return 5 * time.Minute
	case attempts == 3:
		return 4 * time.Minute
	case attempts == 2:
		return 2 * time.Minute
	case attempts == 1:
		return time.Minute
	default:
		return 30 * time.Second
	}
}

// ReconcilePending republishes durable pending deployments. Delivery is
// intentionally at-least-once; corvin-agent makes create_client idempotent by
// technical_client_id.
func (s *VPNService) ReconcilePending(ctx context.Context, limit int) (int, error) {
	_ = ctx
	if s.publisher == nil {
		return 0, errors.New("vpn command publisher is not configured")
	}
	nodes, err := s.vpnRepo.PendingProfileNodes(time.Now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	published := 0
	var firstErr error
	for _, node := range nodes {
		profile, err := s.vpnRepo.GetProfileByID(node.VPNProfileID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		serverID := strings.TrimSpace(node.ServerID)
		if serverID == "" {
			serverID = strings.TrimSpace(node.NodeID)
		}
		eligible, err := s.vpnRepo.IsServerEligible(serverID, profile.EndpointGroup, profile.Protocol)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if !eligible {
			continue
		}
		group, err := s.vpnRepo.GetEndpointGroup(profile.EndpointGroup)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		_, count, publishErr := s.publishProfileNodes(profile, group, []models.VPNProfileNode{node})
		published += count
		if publishErr != nil && firstErr == nil {
			firstErr = publishErr
		}
	}
	return published, firstErr
}

func serverIDsFromNodeStates(nodes []models.NodeState) []string {
	ids := make([]string, 0, len(nodes))
	seen := map[string]struct{}{}
	for _, node := range nodes {
		serverID := strings.TrimSpace(node.ServerID)
		if serverID == "" {
			serverID = strings.TrimSpace(node.NodeID)
		}
		if serverID == "" {
			continue
		}
		if _, ok := seen[serverID]; ok {
			continue
		}
		seen[serverID] = struct{}{}
		ids = append(ids, serverID)
	}
	return ids
}

func endpointGroupForVPNProfile(profile string) string {
	if strings.EqualFold(strings.TrimSpace(profile), jobsvc.VPNProfileTrojan) {
		return jobsvc.EndpointGroupRU
	}
	return jobsvc.EndpointGroupDirect
}

func buildProfileLink(group models.EndpointGroup, client models.VPNClient, profileName string) string {
	host := strings.TrimSpace(group.PublicHost)
	if host == "" {
		return ""
	}
	port := group.PublicPort
	if port == 0 {
		port = 443
	}
	query := url.Values{}
	network := strings.TrimSpace(group.Network)
	if network == "" {
		network = "tcp"
	}
	query.Set("type", network)
	if group.Security != "" {
		query.Set("security", group.Security)
	}
	if group.SNI != "" {
		query.Set("sni", group.SNI)
	}
	if group.Path != "" {
		query.Set("path", group.Path)
	}
	if strings.EqualFold(profileName, jobsvc.VPNProfileVLESS) {
		if group.Flow != "" {
			query.Set("flow", group.Flow)
		}
		query.Set("encryption", "none")
		return (&url.URL{Scheme: "vless", User: url.User(client.VlessUUID), Host: fmt.Sprintf("%s:%d", host, port), RawQuery: query.Encode(), Fragment: client.ClientCode}).String()
	}
	return (&url.URL{Scheme: "trojan", User: url.User(client.TrojanPassword), Host: fmt.Sprintf("%s:%d", host, port), RawQuery: query.Encode(), Fragment: client.ClientCode}).String()
}

func buildServerProfileLink(server models.ServerRegistry, fallback models.EndpointGroup, client models.VPNClient, profileName string) string {
	group := fallback
	if strings.TrimSpace(server.PublicHost) != "" {
		group.PublicHost = strings.TrimSpace(server.PublicHost)
	}
	if server.PublicPort > 0 {
		group.PublicPort = server.PublicPort
	}
	if server.Security != "" {
		group.Security = server.Security
	}
	if server.Network != "" {
		group.Network = server.Network
	}
	if server.SNI != "" {
		group.SNI = server.SNI
	}
	if server.Path != "" {
		group.Path = server.Path
	}
	if server.Flow != "" {
		group.Flow = server.Flow
	}
	return buildProfileLink(group, client, profileName)
}

// ApplyCommandResult validates the durable command identity before applying an
// agent result. On the wire job_id is retained for agent compatibility, but in
// the new flow it is the vpn_profile_nodes primary key, not a jobs row.
func (s *VPNService) ApplyCommandResult(ctx context.Context, event broker.JobResultEvent) (*VPNReadyNotification, error) {
	command, err := s.vpnRepo.GetProfileNodeByID(event.JobID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Warn("vpn command result ignored", "component", "vpn_service", "operation", "apply_command_result", "command_id", event.JobID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "reason", "command_not_found")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	commandServerID := strings.TrimSpace(command.ServerID)
	if commandServerID == "" {
		commandServerID = strings.TrimSpace(command.NodeID)
	}
	if command.VPNProfileID != event.ProfileID || commandServerID != strings.TrimSpace(event.EffectiveServerID()) {
		logger.Warn("vpn command result ignored", "component", "vpn_service", "operation", "apply_command_result", "command_id", event.JobID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "reason", "command_identity_mismatch")
		return nil, nil
	}
	pendingAction := strings.TrimSpace(command.PendingAction)
	eventAction := strings.TrimSpace(event.CommandType)
	if pendingAction != "" && eventAction != "" && pendingAction != eventAction {
		logger.Warn("vpn command result ignored", "component", "vpn_service", "operation", "apply_command_result", "command_id", event.JobID, "pending_action", pendingAction, "result_action", eventAction, "reason", "command_action_mismatch")
		return nil, nil
	}
	if pendingAction != "" && pendingAction != jobsvc.ActionCreateClient {
		status, ok := normalizeProfileNodeResultStatus(event.Status)
		if !ok {
			return nil, nil
		}
		appliedAt := time.Now().UTC()
		if event.CreatedAt != nil && !event.CreatedAt.IsZero() {
			appliedAt = *event.CreatedAt
		}
		if _, err := s.vpnRepo.ApplyProfileNodeActionResult(command.ID, pendingAction, status, valueOrEmptyString(event.Error), appliedAt); err != nil {
			return nil, err
		}
		profile, err := s.vpnRepo.GetProfileByID(command.VPNProfileID)
		if err != nil {
			return nil, err
		}
		profileStatus, profileError := recalculateVPNProfileStatus(profile.Nodes)
		_, err = s.vpnRepo.UpdateProfileResult(profile.ID, profileStatus, profile.FinalLink, profileError, nil)
		return nil, err
	}
	return s.ApplyJobResult(ctx, event)
}

func (s *VPNService) ApplyJobResult(ctx context.Context, event broker.JobResultEvent) (*VPNReadyNotification, error) {
	_ = ctx
	status, ok := normalizeProfileNodeResultStatus(event.Status)
	if !ok {
		logger.Error("vpn job_result unknown status", nil, "job_id", event.JobID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
		return nil, nil
	}
	if event.ProfileID == 0 {
		logger.Warn("vpn job_result unknown profile", "job_id", event.JobID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
		return nil, nil
	}

	logger.Info("job_result received", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
	logger.Info("job_result profile lookup started", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID)

	profile, err := s.vpnRepo.GetProfileByID(event.ProfileID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Warn("vpn job_result unknown profile", "job_id", event.JobID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
		return nil, nil
	}
	if err != nil {
		logger.Error("job_result profile lookup failed", err, "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "db_error")
		return nil, err
	}
	logger.Info("job_result profile found", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", profile.Status, "client_code", profile.VPNClient.ClientCode)

	serverID := strings.TrimSpace(event.EffectiveServerID())
	if serverID == "" {
		logger.Warn("vpn job_result missing node", "job_id", event.JobID, "profile_id", event.ProfileID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
		return nil, nil
	}

	protocol := strings.TrimSpace(event.Protocol)
	if protocol == "" {
		protocol = profile.Protocol
	}
	if protocol == "" {
		protocol = profile.Profile
	}
	appliedAt := time.Now()
	if event.CreatedAt != nil && !event.CreatedAt.IsZero() {
		appliedAt = *event.CreatedAt
	}
	logger.Info("job_result profile node lookup started", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "server_id", serverID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", protocol)
	node, duplicate, err := s.vpnRepo.ApplyProfileNodeResult(profile, serverID, protocol, status, event.InboundID, valueOrEmptyString(event.Error), appliedAt)
	if err != nil {
		logger.Error("job_result profile node update failed", err, "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "server_id", serverID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", protocol)
		return nil, err
	}
	if duplicate {
		logger.Info("vpn job_result ignored duplicate", "job_id", event.JobID, "profile_id", profile.ID, "server_id", serverID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", protocol, "status", status, "client_code", profile.VPNClient.ClientCode)
	}
	logger.Info("vpn profile node result applied", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "server_id", node.NodeID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", node.Protocol, "status", node.Status, "client_code", profile.VPNClient.ClientCode)

	logger.Info("vpn profile status recalculation started", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "server_id", node.NodeID, "profile", profile.Profile, "target_group", profile.EndpointGroup)
	profile, err = s.vpnRepo.GetProfileByID(profile.ID)
	if err != nil {
		return nil, err
	}
	newStatus, profileError := recalculateVPNProfileStatus(profile.Nodes)
	finalLink := profile.FinalLink
	if isUsableVPNProfileStatus(newStatus) && strings.TrimSpace(finalLink) == "" {
		logger.Info("vpn final link generation started", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", newStatus, "client_code", profile.VPNClient.ClientCode)
		group, err := s.vpnRepo.GetEndpointGroup(profile.EndpointGroup)
		if err != nil {
			logger.Error("vpn final link generation failed", err, "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", newStatus, "client_code", profile.VPNClient.ClientCode)
			return nil, err
		}
		finalLink = buildProfileLink(group, profile.VPNClient, profile.Profile)
		logger.Info("vpn final link generated", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", newStatus, "client_code", profile.VPNClient.ClientCode)
	} else {
		logger.Info("vpn final link generation skipped", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", newStatus, "client_code", profile.VPNClient.ClientCode, "reason", finalLinkSkipReason(newStatus, finalLink))
	}

	var notifiedAt *time.Time
	shouldNotifyUser := isUsableVPNProfileStatus(newStatus) && profile.NotifiedAt == nil && strings.TrimSpace(finalLink) != ""
	if shouldNotifyUser {
		now := time.Now()
		notifiedAt = &now
	}
	profile, err = s.vpnRepo.UpdateProfileResult(profile.ID, newStatus, finalLink, profileError, notifiedAt)
	if err != nil {
		return nil, err
	}
	logger.Info("vpn profile status recalculated", "job_id", event.JobID, "profile_id", profile.ID, "server_id", serverID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", profile.Status, "client_code", profile.VPNClient.ClientCode)

	if profile.Status == models.VPNProfileStatusPartial {
		logger.Warn("vpn admin notified partial", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", profile.Status, "client_code", profile.VPNClient.ClientCode, "failed_nodes", failedProfileNodes(profile.Nodes))
	}
	if profile.Status == models.VPNProfileStatusFailed {
		logger.Warn("vpn profile failed", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", profile.Status, "client_code", profile.VPNClient.ClientCode, "failed_nodes", failedProfileNodes(profile.Nodes))
	}

	if shouldNotifyUser {
		logger.Info("telegram notification lookup started", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "user_id", profile.VPNClient.UserID)
		telegram, err := s.telegramRepo.GetByUserID(profile.VPNClient.UserID)
		if err != nil {
			logger.Error("telegram notification lookup failed", err, "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "user_id", profile.VPNClient.UserID)
			return nil, err
		}
		logger.Info("telegram notification ready", "component", "vpn_service", "operation", "apply_job_result", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", profile.Status, "client_code", profile.VPNClient.ClientCode, "tg_id", telegram.TgID)
		return &VPNReadyNotification{TgID: telegram.TgID, Protocol: profile.Profile, Link: finalLink}, nil
	}

	return nil, nil
}

func finalLinkSkipReason(status string, finalLink string) string {
	if !isUsableVPNProfileStatus(status) {
		return "profile_not_usable"
	}
	if strings.TrimSpace(finalLink) != "" {
		return "final_link_already_exists"
	}
	return "unknown"
}

func normalizeProfileNodeResultStatus(status string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case models.VPNProfileNodeStatusSuccess:
		return models.VPNProfileNodeStatusSuccess, true
	case models.VPNProfileNodeStatusFailed:
		return models.VPNProfileNodeStatusFailed, true
	default:
		return "", false
	}
}

func recalculateVPNProfileStatus(nodes []models.VPNProfileNode) (string, string) {
	if len(nodes) == 0 {
		return models.VPNProfileStatusFailed, "no profile nodes"
	}
	successCount := 0
	failedCount := 0
	disabledCount := 0
	failedNodes := make([]string, 0)
	for _, node := range nodes {
		switch node.Status {
		case models.VPNProfileNodeStatusSuccess:
			successCount++
		case models.VPNProfileNodeStatusFailed:
			failedCount++
			failedNodes = append(failedNodes, node.NodeID)
		case models.VPNProfileNodeStatusDisabled, models.VPNProfileNodeStatusDeleted:
			disabledCount++
		}
	}
	if disabledCount == len(nodes) {
		return models.VPNProfileStatusDisabled, ""
	}
	if successCount+disabledCount == len(nodes) && successCount > 0 {
		return models.VPNProfileStatusActive, ""
	}
	if successCount > 0 {
		if len(failedNodes) > 0 {
			return models.VPNProfileStatusPartial, "failed nodes: " + strings.Join(failedNodes, ",")
		}
		return models.VPNProfileStatusPartial, "waiting for nodes"
	}
	if failedCount == len(nodes) {
		return models.VPNProfileStatusFailed, "failed nodes: " + strings.Join(failedNodes, ",")
	}
	return models.VPNProfileStatusPending, ""
}

func isUsableVPNProfileStatus(status string) bool {
	return status == models.VPNProfileStatusActive || status == models.VPNProfileStatusPartial
}

func failedProfileNodes(nodes []models.VPNProfileNode) []string {
	failed := make([]string, 0)
	for _, node := range nodes {
		if node.Status == models.VPNProfileNodeStatusFailed {
			failed = append(failed, node.NodeID)
		}
	}
	return failed
}

func (s *VPNService) ApplyAgentCreateResult(job *models.Job, event broker.JobResultEvent) (*VPNReadyNotification, error) {
	if job == nil || job.Action != jobsvc.ActionCreateClient {
		return nil, nil
	}
	if event.ProfileID != 0 {
		return s.ApplyJobResult(context.Background(), event)
	}
	if job.Status != jobsvc.JobStatusSuccess {
		return nil, nil
	}

	var payload broker.JobTask
	if len(job.PayloadJSON) > 0 {
		if err := json.Unmarshal(job.PayloadJSON, &payload); err != nil {
			return nil, err
		}
	}

	protocol := strings.ToLower(strings.TrimSpace(payload.Protocol))
	if protocol == "" {
		protocol = strings.ToLower(strings.TrimSpace(job.Protocol))
	}
	if protocol != "vless" && protocol != "trojan" {
		return nil, ErrUnsupportedProtocol
	}

	userID := payload.UserID
	if userID == 0 {
		return nil, errors.New("job payload user_id is empty")
	}

	link := strings.TrimSpace(valueOrEmptyString(event.ConfigLink))
	if link == "" {
		link = strings.TrimSpace(configLinkFromResult(event.ResultJSON))
	}
	if link == "" {
		return nil, errors.New("agent result config link is empty")
	}

	if _, err := s.vpnRepo.UpsertLinkByUserID(userID, protocol, link); err != nil {
		return nil, err
	}

	telegram, err := s.telegramRepo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	return &VPNReadyNotification{
		TgID:     telegram.TgID,
		Protocol: protocol,
		Link:     link,
	}, nil
}

func configLinkFromResult(raw *json.RawMessage) string {
	if raw == nil || len(*raw) == 0 {
		return ""
	}
	var payload struct {
		ConfigLink string `json:"config_link"`
		Link       string `json:"link"`
	}
	if err := json.Unmarshal(*raw, &payload); err != nil {
		return ""
	}
	if payload.ConfigLink != "" {
		return payload.ConfigLink
	}
	return payload.Link
}

func valueOrEmptyString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *VPNService) GetLinkOverview(tgID int64) (LinkOverviewResult, error) {
	logger.Info("vpn link overview lookup started", "component", "vpn_service", "operation", "link_overview", "tg_id", tgID)
	overview := LinkOverviewResult{TgID: tgID, Profiles: defaultLinkProfiles()}

	logger.Info("telegram user lookup started", "component", "vpn_service", "operation", "link_overview", "tg_id", tgID)
	telegram, err := s.telegramRepo.FindByTgID(tgID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			overview.Reason = "telegram_user_not_found"
			logger.Warn("telegram user not found", "component", "vpn_service", "operation", "link_overview", "tg_id", tgID, "reason", overview.Reason)
			return overview, nil
		}
		logger.Error("telegram user lookup failed", err, "component", "vpn_service", "operation", "link_overview", "tg_id", tgID, "reason", "db_error")
		return overview, err
	}
	overview.UserID = telegram.UserID
	logger.Info("telegram user found", "component", "vpn_service", "operation", "link_overview", "tg_id", tgID, "user_id", telegram.UserID)

	canonicalFound, err := s.applyCanonicalLinkProfiles(&overview, telegram.UserID)
	if err != nil {
		return overview, err
	}
	canonicalReason := linkOverviewReason(overview)
	if !overviewHasUsableLinks(overview) {
		if err := s.applyLegacyLinkFallback(&overview, telegram.UserID); err != nil {
			return overview, err
		}
	}

	overview.Reason = linkOverviewReason(overview)
	if !overviewHasUsableLinks(overview) {
		if !canonicalFound {
			overview.Reason = "vpn_not_configured"
		} else if canonicalReason == "profiles_pending" || canonicalReason == "profiles_failed" {
			overview.Reason = canonicalReason
		}
	}
	logger.Info("vpn link overview response selected", "component", "vpn_service", "operation", "link_overview", "tg_id", tgID, "user_id", telegram.UserID, "client_code", overview.ClientCode, "reason", overview.Reason, "vless_status", overview.Profiles[jobsvc.VPNProfileVLESS].Status, "trojan_status", overview.Profiles[jobsvc.VPNProfileTrojan].Status)
	return overview, nil
}

func (s *VPNService) applyCanonicalLinkProfiles(overview *LinkOverviewResult, userID uint) (bool, error) {
	logger.Info("vpn client lookup started", "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID)
	client, err := s.vpnRepo.GetVPNClientByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("vpn client not_found", "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "reason", "vpn_client_not_found")
			return false, nil
		}
		logger.Error("vpn client lookup failed", err, "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "reason", "db_error")
		return false, err
	}
	overview.ClientID = client.ID
	overview.ClientCode = client.ClientCode
	logger.Info("vpn client found", "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "client_code", client.ClientCode)

	logger.Info("vpn profiles lookup started", "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "client_code", client.ClientCode)
	profiles, err := s.vpnRepo.ListProfilesByClientID(client.ID)
	if err != nil {
		logger.Error("vpn profiles lookup failed", err, "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "client_code", client.ClientCode, "reason", "db_error")
		return true, err
	}
	if len(profiles) == 0 {
		logger.Warn("vpn profiles not_found", "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "client_code", client.ClientCode, "reason", "vpn_profiles_not_found")
		return false, nil
	}
	logger.Info("vpn profiles found", "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "client_code", client.ClientCode, "profiles_count", len(profiles))

	hasSupportedProfile := false
	for _, profile := range profiles {
		name := strings.ToLower(strings.TrimSpace(profile.Profile))
		if name != jobsvc.VPNProfileVLESS && name != jobsvc.VPNProfileTrojan {
			continue
		}
		hasSupportedProfile = true
		view := canonicalLinkProfileView(profile)
		overview.Profiles[name] = view
		logger.Info("vpn profile evaluated", "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "client_code", client.ClientCode, "protocol", name, "profile_id", profile.ID, "status", view.Status, "has_final_link", strings.TrimSpace(view.FinalLink) != "", "usable", view.Usable, "reason", view.Reason)
	}
	return hasSupportedProfile, nil
}

func (s *VPNService) applyLegacyLinkFallback(overview *LinkOverviewResult, userID uint) error {
	logger.Info("legacy vpn fallback started", "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID)
	vpn, err := s.vpnRepo.GetByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("legacy vpn not_found", "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "reason", "legacy_vpn_not_found")
			return nil
		}
		logger.Error("legacy vpn lookup failed", err, "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "reason", "db_error")
		return err
	}
	hasVless := hasUsableLink(vpn.VlessLink)
	hasTrojan := hasUsableLink(vpn.TrojanLink)
	logger.Info("legacy vpn found", "component", "vpn_service", "operation", "link_overview", "tg_id", overview.TgID, "user_id", userID, "vpn_id", vpn.ID, "has_vless", hasVless, "has_trojan", hasTrojan)
	if hasVless {
		mergeLegacyLinkProfile(overview, jobsvc.VPNProfileVLESS, jobsvc.EndpointGroupDirect, vpn.VlessLink)
	}
	if hasTrojan {
		mergeLegacyLinkProfile(overview, jobsvc.VPNProfileTrojan, jobsvc.EndpointGroupRU, vpn.TrojanLink)
	}
	return nil
}

func mergeLegacyLinkProfile(overview *LinkOverviewResult, profile string, endpointGroup string, link string) {
	current := overview.Profiles[profile]
	if current.Usable && strings.TrimSpace(current.FinalLink) != "" {
		return
	}
	overview.Profiles[profile] = LinkProfileView{
		Profile:       profile,
		EndpointGroup: endpointGroup,
		Protocol:      profile,
		Status:        models.VPNProfileStatusActive,
		Exists:        true,
		Usable:        true,
		FinalLink:     link,
		Reason:        "protocol_link_found",
		Source:        "legacy",
	}
}

func (s *VPNService) GetProtocolLink(tgID int64, protocol string) (ProtocolLinkResult, error) {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	logger.Info("vpn protocol link lookup started", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "protocol", protocol)
	if protocol != jobsvc.VPNProfileVLESS && protocol != jobsvc.VPNProfileTrojan {
		logger.Warn("vpn protocol link rejected", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "protocol", protocol, "reason", "unsupported_protocol")
		return ProtocolLinkResult{TgID: tgID, Protocol: protocol, Reason: "unsupported_protocol"}, ErrUnsupportedProtocol
	}

	overview, err := s.GetLinkOverview(tgID)
	if err != nil {
		logger.Error("vpn protocol link overview failed", err, "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "protocol", protocol, "reason", "internal_error")
		return ProtocolLinkResult{}, err
	}
	profile := overview.Profiles[protocol]
	result := ProtocolLinkResult{TgID: tgID, Protocol: protocol, Status: profile.Status, Exists: profile.Exists, Usable: profile.Usable, Link: profile.FinalLink}
	if profile.Source == "canonical" && profile.Usable && strings.TrimSpace(profile.FinalLink) != "" {
		logger.Info("vpn protocol link found", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "user_id", overview.UserID, "client_code", overview.ClientCode, "protocol", protocol, "status", profile.Status)
	} else {
		logger.Info("vpn protocol link not_found", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "user_id", overview.UserID, "client_code", overview.ClientCode, "protocol", protocol, "status", profile.Status, "reason", profile.Reason)
		logger.Info("legacy protocol fallback started", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "user_id", overview.UserID, "protocol", protocol, "reason", "legacy_vpn_fallback")
		if profile.Source == "legacy" && profile.Usable && strings.TrimSpace(profile.FinalLink) != "" {
			logger.Info("legacy protocol link found", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "user_id", overview.UserID, "protocol", protocol, "reason", "legacy_vpn_fallback")
		} else {
			logger.Warn("legacy protocol link not_found", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "user_id", overview.UserID, "protocol", protocol, "reason", profile.Reason)
		}
	}
	switch {
	case profile.Usable && strings.TrimSpace(profile.FinalLink) != "":
		result.Reason = "protocol_link_found"
		logger.Info("vpn protocol link found", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "user_id", overview.UserID, "client_code", overview.ClientCode, "protocol", protocol, "status", profile.Status, "reason", result.Reason)
	case !profile.Exists:
		result.Reason = "protocol_link_not_found"
		logger.Warn("vpn protocol link not found", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "user_id", overview.UserID, "client_code", overview.ClientCode, "protocol", protocol, "reason", result.Reason)
	case profile.Status == models.VPNProfileStatusPending:
		result.Reason = "protocol_pending"
		logger.Warn("vpn protocol link not found", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "user_id", overview.UserID, "client_code", overview.ClientCode, "protocol", protocol, "status", profile.Status, "reason", result.Reason)
	case profile.Status == models.VPNProfileStatusFailed:
		result.Reason = "protocol_failed"
		logger.Warn("vpn protocol link not found", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "user_id", overview.UserID, "client_code", overview.ClientCode, "protocol", protocol, "status", profile.Status, "reason", result.Reason)
	default:
		result.Reason = "protocol_link_not_found"
		logger.Warn("vpn protocol link not found", "component", "vpn_service", "operation", "protocol_link", "tg_id", tgID, "user_id", overview.UserID, "client_code", overview.ClientCode, "protocol", protocol, "status", profile.Status, "reason", result.Reason)
	}
	return result, nil
}

func defaultLinkProfiles() map[string]LinkProfileView {
	return map[string]LinkProfileView{
		jobsvc.VPNProfileVLESS:  {Profile: jobsvc.VPNProfileVLESS, EndpointGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: "missing", Reason: "protocol_link_not_found"},
		jobsvc.VPNProfileTrojan: {Profile: jobsvc.VPNProfileTrojan, EndpointGroup: jobsvc.EndpointGroupRU, Protocol: jobsvc.VPNProfileTrojan, Status: "missing", Reason: "protocol_link_not_found"},
	}
}

func canonicalLinkProfileView(profile models.VPNProfile) LinkProfileView {
	view := LinkProfileView{Profile: profile.Profile, EndpointGroup: profile.EndpointGroup, Protocol: profile.Protocol, Status: profile.Status, Exists: true, FinalLink: profile.FinalLink, Source: "canonical"}
	view.Usable = isUsableVPNProfileStatus(profile.Status) && strings.TrimSpace(profile.FinalLink) != ""
	switch {
	case view.Usable:
		view.Reason = "protocol_link_found"
	case profile.Status == models.VPNProfileStatusPending:
		view.Reason = "protocol_pending"
	case profile.Status == models.VPNProfileStatusFailed:
		view.Reason = "protocol_failed"
	default:
		view.Reason = "protocol_link_not_found"
	}
	return view
}

func overviewHasUsableLinks(overview LinkOverviewResult) bool {
	vless := overview.Profiles[jobsvc.VPNProfileVLESS]
	trojan := overview.Profiles[jobsvc.VPNProfileTrojan]
	return (vless.Usable && strings.TrimSpace(vless.FinalLink) != "") || (trojan.Usable && strings.TrimSpace(trojan.FinalLink) != "")
}

func hasUsableLink(link string) bool {
	trimmed := strings.TrimSpace(link)
	return trimmed != "" && trimmed != "null"
}

func linkOverviewReason(overview LinkOverviewResult) string {
	vless := overview.Profiles[jobsvc.VPNProfileVLESS]
	trojan := overview.Profiles[jobsvc.VPNProfileTrojan]
	if vless.Usable && strings.TrimSpace(vless.FinalLink) != "" && trojan.Usable && strings.TrimSpace(trojan.FinalLink) != "" {
		return "both_links_available"
	}
	if (vless.Usable && strings.TrimSpace(vless.FinalLink) != "") || (trojan.Usable && strings.TrimSpace(trojan.FinalLink) != "") {
		return "single_link_available"
	}
	if vless.Exists && trojan.Exists && vless.Status == models.VPNProfileStatusPending && trojan.Status == models.VPNProfileStatusPending {
		return "profiles_pending"
	}
	if (vless.Exists && vless.Status == models.VPNProfileStatusFailed) || (trojan.Exists && trojan.Status == models.VPNProfileStatusFailed) {
		return "profiles_failed"
	}
	return "no_usable_profiles"
}
