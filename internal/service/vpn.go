package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/jobsvc"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/utils"

	"gorm.io/gorm"
)

var (
	ErrVPNAlreadyExists    = errors.New("vpn already exists")
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
	ErrNoMatchingServers   = errors.New("no matching servers for vpn profile")
	ErrNoJobsQueued        = errors.New("vpn create jobs were not queued")
)

type VPNErrorKind string

const (
	VPNErrorKindJobs   VPNErrorKind = "jobs"
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

type CreateVPNInput struct {
	TgID int64
}

type CreateVPNProtocolInput struct {
	TgID     int64
	Protocol string
}

type RequestCreateVPNInput struct {
	TgID     int64
	Protocol string
}

type RequestCreateVPNResult struct {
	TgID      int64
	Protocol  string
	BatchID   uint
	JobID     uint
	JobsCount int
	Status    string
	FinalLink string
}

type VPNReadyNotification struct {
	TgID     int64
	Protocol string
	Link     string
}

type UserVPNDetails struct {
	Client   *UserVPNClientView   `json:"client"`
	Profiles []UserVPNProfileView `json:"profiles"`
}

type UserVPNClientView struct {
	ID         uint   `json:"id"`
	ClientCode string `json:"client_code"`
	Email      string `json:"email"`
}

type UserVPNProfileView struct {
	ID            uint                     `json:"id,omitempty"`
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

type UserVPNProfileNodeView struct {
	ServerID  string     `json:"server_id"`
	NodeID    string     `json:"node_id,omitempty"`
	Status    string     `json:"status"`
	Protocol  string     `json:"protocol"`
	InboundID *int       `json:"inbound_id,omitempty"`
	LastError string     `json:"last_error,omitempty"`
	AppliedAt *time.Time `json:"applied_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
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
	jobs         *jobsvc.Service
	audit        *audit.Logger
}

func NewVPNService(
	vpnRepo *repository.VpnRepo,
	telegramRepo *repository.TelegramRepo,
	jobs *jobsvc.Service,
	auditLogger *audit.Logger,
) *VPNService {
	return &VPNService{
		vpnRepo:      vpnRepo,
		telegramRepo: telegramRepo,
		jobs:         jobs,
		audit:        auditLogger,
	}
}

func (s *VPNService) GetUserVPNDetails(userID uint) (UserVPNDetails, error) {
	details := UserVPNDetails{Profiles: defaultUserVPNProfileViews()}
	client, err := s.vpnRepo.GetVPNClientByUserID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return details, nil
	}
	if err != nil {
		return UserVPNDetails{}, err
	}

	details.Client = &UserVPNClientView{ID: client.ID, ClientCode: client.ClientCode, Email: client.Email}
	profiles, err := s.vpnRepo.ListProfilesByClientID(client.ID)
	if err != nil {
		return UserVPNDetails{}, err
	}

	views := map[string]UserVPNProfileView{}
	for _, view := range details.Profiles {
		views[view.Profile] = view
	}
	for _, profile := range profiles {
		createdAt := profile.CreatedAt
		updatedAt := profile.UpdatedAt
		view := UserVPNProfileView{
			ID:            profile.ID,
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
			view.Nodes = append(view.Nodes, UserVPNProfileNodeView{
				ServerID:  serverID,
				NodeID:    node.NodeID,
				Status:    node.Status,
				Protocol:  node.Protocol,
				InboundID: node.InboundID,
				LastError: node.LastError,
				AppliedAt: node.AppliedAt,
				UpdatedAt: node.UpdatedAt,
			})
		}
		views[profile.Profile] = view
	}

	details.Profiles = []UserVPNProfileView{views[jobsvc.VPNProfileVLESS], views[jobsvc.VPNProfileTrojan]}
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

	logger.Info("vpn client lookup started", "component", "vpn_service", "operation", "request_create_vpn", "user_id", telegram.UserID, "telegram_id", input.TgID)
	client, created, err := s.vpnRepo.GetOrCreateVPNClient(telegram.UserID, input.TgID)
	if err != nil {
		logger.Error("vpn client lookup failed", err, "component", "vpn_service", "operation", "request_create_vpn", "user_id", telegram.UserID, "telegram_id", input.TgID, "reason", "db_error")
		return nil, err
	}
	if created {
		logger.Info("vpn client created", "component", "vpn_service", "operation", "request_create_vpn", "user_id", telegram.UserID, "telegram_id", input.TgID, "client_code", client.ClientCode)
	} else {
		logger.Info("vpn client reused", "component", "vpn_service", "operation", "request_create_vpn", "user_id", telegram.UserID, "telegram_id", input.TgID, "client_code", client.ClientCode)
	}

	result := &RequestCreateVPNResult{TgID: input.TgID, Protocol: strings.Join(profiles, ",")}
	activeProfiles := 0
	for _, profileName := range profiles {
		profile, batchID, jobID, jobsCount, err := s.ensureVPNProfile(telegram.UserID, input.TgID, client, profileName)
		if err != nil {
			return nil, err
		}
		if result.BatchID == 0 {
			result.BatchID = batchID
		}
		if result.JobID == 0 {
			result.JobID = jobID
		}
		result.JobsCount += jobsCount
		if len(profiles) == 1 {
			result.Protocol = profile.Profile
			result.Status = profile.Status
			result.FinalLink = profile.FinalLink
		}
		if profile.Status == models.VPNProfileStatusActive && strings.TrimSpace(profile.FinalLink) != "" {
			activeProfiles++
		}
	}
	if result.JobsCount == 0 {
		if activeProfiles == len(profiles) {
			logger.Info("vpn create request returned existing link", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "profile", result.Protocol, "reason", "profile_already_active")
			return result, nil
		}
		logger.Warn("vpn create request not queued", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "profile", result.Protocol, "batch_id", result.BatchID, "job_id", result.JobID, "jobs_count", result.JobsCount, "reason", "no_jobs_created")
		return nil, ErrNoJobsQueued
	}
	if result.BatchID == 0 || result.JobID == 0 {
		logger.Warn("vpn create request not queued", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "profile", result.Protocol, "batch_id", result.BatchID, "job_id", result.JobID, "jobs_count", result.JobsCount, "reason", "invalid_job_ids")
		return nil, ErrNoJobsQueued
	}
	logger.Info("vpn create request queued", "component", "vpn_service", "operation", "request_create_vpn", "telegram_id", input.TgID, "profile", result.Protocol, "batch_id", result.BatchID, "job_id", result.JobID, "jobs_count", result.JobsCount)
	return result, nil
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

func (s *VPNService) ensureVPNProfile(userID uint, tgID int64, client models.VPNClient, profileName string) (models.VPNProfile, uint, uint, int, error) {
	endpointGroup := endpointGroupForVPNProfile(profileName)
	expectedProtocol := profileName
	logger.Info("vpn profile lookup started", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "protocol", expectedProtocol)
	group, err := s.vpnRepo.GetOrCreateEndpointGroup(endpointGroup)
	if err != nil {
		logger.Error("vpn endpoint group lookup failed", err, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup)
		return models.VPNProfile{}, 0, 0, 0, err
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
			return models.VPNProfile{}, 0, 0, 0, err
		}
		profileCreated = true
		logger.Info("vpn profile created", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "status", profile.Status)
	} else {
		logger.Error("vpn profile lookup failed", err, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "reason", "db_error")
		return models.VPNProfile{}, 0, 0, 0, err
	}

	if profile.Status == models.VPNProfileStatusActive && strings.TrimSpace(profile.FinalLink) != "" {
		logger.Info("vpn profile reused", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "profile_id", profile.ID, "status", profile.Status, "reason", "profile_already_active")
		return profile, 0, 0, 0, nil
	}

	logger.Info("vpn provisioning rebuild started", "component", "vpn_service", "operation", "ensure_vpn_profile", "profile_id", profile.ID, "profile", profileName, "reason", "profile_not_active")
	logger.Info("vpn target servers lookup started", "component", "vpn_service", "operation", "ensure_vpn_profile", "endpoint_group", endpointGroup, "expected_protocol", expectedProtocol)
	nodes, err := s.vpnRepo.EnabledNodesByGroup(endpointGroup)
	if err != nil {
		logger.Error("vpn target servers lookup failed", err, "component", "vpn_service", "operation", "ensure_vpn_profile", "profile_id", profile.ID, "endpoint_group", endpointGroup, "expected_protocol", expectedProtocol)
		return models.VPNProfile{}, 0, 0, 0, err
	}
	logger.Info("vpn target servers found", "component", "vpn_service", "operation", "ensure_vpn_profile", "endpoint_group", endpointGroup, "expected_protocol", expectedProtocol, "count", len(nodes))

	if len(nodes) == 0 {
		lastError := "no_matching_servers"
		_ = s.vpnRepo.UpdateProfileStatus(profile.ID, models.VPNProfileStatusFailed, lastError)
		logger.Warn("vpn profile has no matching servers", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "expected_protocol", expectedProtocol, "profile_id", profile.ID, "reason", lastError)
		profile.Status = models.VPNProfileStatusFailed
		profile.LastError = lastError
		return profile, 0, 0, 0, ErrNoMatchingServers
	}

	profile, _, err = s.vpnRepo.EnsureProfileNodes(profile, nodes)
	if err != nil {
		logger.Error("vpn profile nodes ensure failed", err, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID)
		return models.VPNProfile{}, 0, 0, 0, err
	}
	for _, node := range profile.Nodes {
		logger.Info("vpn profile node pending", "component", "vpn_service", "operation", "ensure_vpn_profile", "profile_id", profile.ID, "server_id", node.ServerID, "profile", profileName, "endpoint_group", endpointGroup)
	}
	if profileCreated && len(profile.Nodes) == 0 {
		logger.Warn("vpn profile created without nodes", "component", "vpn_service", "operation", "ensure_vpn_profile", "profile_id", profile.ID, "profile", profileName, "reason", "no_profile_nodes_created")
	}

	if s.jobs == nil {
		logger.Error("vpn create job build failed", nil, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "reason", "jobs_service_not_configured")
		return models.VPNProfile{}, 0, 0, 0, errors.New("jobs service is not configured")
	}

	targetServerIDs := targetServerIDsForProfileNodes(profile.Nodes)
	if len(targetServerIDs) == 0 {
		logger.Warn("vpn create job build skipped", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "status", profile.Status, "reason", "profile_has_no_targets")
		return profile, 0, 0, 0, ErrNoJobsQueued
	}
	if superseded, err := s.jobs.SupersedeCreateClientJobs(profile.ID, targetServerIDs); err != nil {
		return models.VPNProfile{}, 0, 0, 0, err
	} else if superseded > 0 {
		logger.Info("vpn create jobs superseded", "component", "vpn_service", "operation", "ensure_vpn_profile", "profile_id", profile.ID, "jobs_count", superseded, "reason", "rebuild_provisioning_plan")
	}

	logger.Info("vpn create job build started", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "nodes_count", len(profile.Nodes), "target_servers_count", len(targetServerIDs))
	batch, jobs, err := s.jobs.CreateUserConfig(jobsvc.CreateUserConfigInput{
		ProfileID:       profile.ID,
		VPNClientID:     client.ID,
		UserID:          userID,
		TelegramID:      tgID,
		ClientCode:      client.ClientCode,
		Email:           client.Email,
		VlessUUID:       client.VlessUUID,
		VlessFlow:       group.Flow,
		TrojanPassword:  client.TrojanPassword,
		Enable:          true,
		Protocols:       []string{profileName},
		TargetServerIDs: targetServerIDs,
	})
	if err != nil {
		_ = s.vpnRepo.TouchProfilePublishError(profile.ID, err.Error())
		logger.Error("vpn create job publish failed", err, "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "nodes_count", len(profile.Nodes))
		return models.VPNProfile{}, 0, 0, 0, &VPNFlowError{Kind: VPNErrorKindJobs, Err: err}
	}
	if len(jobs) == 0 || batch == nil || batch.ID == 0 {
		logger.Warn("vpn create jobs missing after build", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "reason", "jobs_not_created")
		return profile, 0, 0, 0, ErrNoJobsQueued
	}

	if err := s.vpnRepo.UpdateProfileStatus(profile.ID, models.VPNProfileStatusPending, ""); err != nil {
		return models.VPNProfile{}, 0, 0, 0, err
	}
	profile.Status = models.VPNProfileStatusPending
	profile.LastError = ""
	jobID := jobs[0].ID
	logger.Info("vpn create job published", "component", "vpn_service", "operation", "ensure_vpn_profile", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "endpoint_group", endpointGroup, "profile_id", profile.ID, "batch_id", batch.ID, "job_id", jobID, "jobs_count", len(jobs))
	return profile, batch.ID, jobID, len(jobs), nil
}

func targetServerIDsForProfileNodes(nodes []models.VPNProfileNode) []string {
	ids := []string{}
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

func (s *VPNService) targetServerIDsForProfiles(profiles []string) ([]string, error) {
	ids := []string{}
	seen := map[string]struct{}{}
	for _, profile := range profiles {
		group := endpointGroupForVPNProfile(profile)
		nodes, err := s.vpnRepo.EnabledNodesByGroup(group)
		if err != nil {
			return nil, err
		}
		for _, id := range serverIDsFromNodeStates(nodes) {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func profileReuseReason(status string) string {
	switch status {
	case models.VPNProfileStatusActive:
		return "profile_already_active"
	case models.VPNProfileStatusPending:
		return "profile_pending"
	case models.VPNProfileStatusFailed:
		return "profile_failed"
	case models.VPNProfileStatusPartial:
		return "profile_partial"
	default:
		return "profile_reused"
	}
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
	failedNodes := make([]string, 0)
	for _, node := range nodes {
		switch node.Status {
		case models.VPNProfileNodeStatusSuccess:
			successCount++
		case models.VPNProfileNodeStatusFailed:
			failedCount++
			failedNodes = append(failedNodes, node.NodeID)
		}
	}
	if successCount == len(nodes) {
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

func (s *VPNService) CreateVPN(input CreateVPNInput) (models.Vpn, error) {
	vlessParams := utils.GenVlessLink(input.TgID)
	trojanParams := utils.GenTrojanLink(input.TgID)

	telegram, err := s.telegramRepo.GetTelegramByTgID(input.TgID)
	if err != nil {
		return models.Vpn{}, err
	}

	vpn, err := s.vpnRepo.Create(models.Vpn{
		UUID:       vlessParams.UID,
		UserID:     telegram.UserID,
		Status:     "active",
		Link:       vlessParams.Link,
		VlessLink:  vlessParams.Link,
		TrojanLink: trojanParams.Link,
	})
	if err != nil {
		return models.Vpn{}, err
	}

	if s.jobs != nil {
		targetServerIDs, err := s.targetServerIDsForProfiles([]string{jobsvc.VPNProfileVLESS, jobsvc.VPNProfileTrojan})
		if err != nil {
			return models.Vpn{}, err
		}
		batch, jobs, err := s.jobs.CreateUserConfig(jobsvc.CreateUserConfigInput{
			UserID:            telegram.UserID,
			TelegramID:        input.TgID,
			ClientCode:        vlessParams.Name,
			Email:             vlessParams.Name,
			VlessUUID:         vlessParams.UID,
			VlessFlow:         vlessParams.Flow,
			TrojanPassword:    trojanParams.Password,
			Enable:            true,
			TechnicalClientID: vlessParams.UID,
			Protocols:         []string{"vless", "trojan"},
			TargetServerIDs:   targetServerIDs,
		})
		if err != nil {
			return models.Vpn{}, &VPNFlowError{Kind: VPNErrorKindJobs, Err: err}
		}
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorTelegramUser,
			ActorID:    audit.StringID(input.TgID),
			Action:     "vpn.client.created",
			EntityType: "job_batch",
			EntityID:   audit.StringID(batch.ID),
			Status:     audit.StatusSuccess,
			Message:    "vpn create jobs queued",
			Metadata: map[string]any{
				"jobs_count": len(jobs),
				"user_id":    telegram.UserID,
			},
		})
	}

	task := broker.CreateUserTask{
		UserID:     input.TgID,
		Username:   vlessParams.Name,
		UUID:       vlessParams.UID,
		PBK:        vlessParams.PBK,
		SID:        vlessParams.SID,
		SPX:        vlessParams.SPX,
		Flow:       vlessParams.Flow,
		Encryption: vlessParams.Encryption,

		Type:     trojanParams.Type,
		Security: trojanParams.Security,
		Fp:       trojanParams.Fp,
		Alpn:     trojanParams.Alpn,
		Sni:      trojanParams.Sni,
		Password: trojanParams.Password,
	}

	if err := broker.GlobalProducer.PublishCreateUser(task); err != nil {
		return models.Vpn{}, &VPNFlowError{Kind: VPNErrorKindBroker, Err: err}
	}

	return vpn, nil
}

func (s *VPNService) CreateVPNProtocol(input CreateVPNProtocolInput) (models.Vpn, error) {
	var vlessParams utils.VlessParams
	var trojanParams utils.TrojanParams
	switch input.Protocol {
	case "vless":
		vlessParams = utils.GenVlessLink(input.TgID)
	case "trojan":
		trojanParams = utils.GenTrojanLink(input.TgID)
	}

	telegram, err := s.telegramRepo.GetTelegramByTgID(input.TgID)
	if err != nil {
		return models.Vpn{}, err
	}

	var vpn models.Vpn
	switch input.Protocol {
	case "vless":
		vpn, err = s.upsertVPNProtocol(telegram.UserID, input.Protocol, vlessParams.Link)
		if err != nil {
			return models.Vpn{}, err
		}
	case "trojan":
		vpn, err = s.upsertVPNProtocol(telegram.UserID, input.Protocol, trojanParams.Link)
		if err != nil {
			return models.Vpn{}, err
		}
	}

	var username string
	switch input.Protocol {
	case "vless":
		username = vlessParams.Name
	case "trojan":
		username = trojanParams.Name
	}

	if s.jobs != nil {
		targetServerIDs, err := s.targetServerIDsForProfiles([]string{input.Protocol})
		if err != nil {
			return models.Vpn{}, err
		}
		technicalClientID := vlessParams.UID
		if input.Protocol == "trojan" {
			technicalClientID = trojanParams.Password
		}
		batch, jobs, err := s.jobs.CreateUserConfig(jobsvc.CreateUserConfigInput{
			UserID:            telegram.UserID,
			TelegramID:        input.TgID,
			ClientCode:        username,
			Email:             username,
			VlessUUID:         vlessParams.UID,
			VlessFlow:         vlessParams.Flow,
			TrojanPassword:    trojanParams.Password,
			Enable:            true,
			TechnicalClientID: technicalClientID,
			Protocols:         []string{input.Protocol},
			TargetServerIDs:   targetServerIDs,
		})
		if err != nil {
			return models.Vpn{}, &VPNFlowError{Kind: VPNErrorKindJobs, Err: err}
		}
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorTelegramUser,
			ActorID:    audit.StringID(input.TgID),
			Action:     "vpn.client.created",
			EntityType: "job_batch",
			EntityID:   audit.StringID(batch.ID),
			Status:     audit.StatusSuccess,
			Message:    "vpn protocol create jobs queued",
			Metadata: map[string]any{
				"jobs_count": len(jobs),
				"protocol":   input.Protocol,
				"user_id":    telegram.UserID,
			},
		})
	}

	task := broker.CreateUserTask{
		UserID:     input.TgID,
		Username:   username,
		UUID:       vlessParams.UID,
		PBK:        vlessParams.PBK,
		SID:        vlessParams.SID,
		SPX:        vlessParams.SPX,
		Flow:       vlessParams.Flow,
		Encryption: vlessParams.Encryption,

		Type:     trojanParams.Type,
		Security: trojanParams.Security,
		Fp:       trojanParams.Fp,
		Alpn:     trojanParams.Alpn,
		Sni:      trojanParams.Sni,
		Password: trojanParams.Password,
	}

	if err := broker.GlobalProducer.PublishCreateUser(task); err != nil {
		return models.Vpn{}, &VPNFlowError{Kind: VPNErrorKindBroker, Err: err}
	}

	return vpn, nil
}

func (s *VPNService) upsertVPNProtocol(userID uint, protocol string, link string) (models.Vpn, error) {
	vpn, err := s.vpnRepo.GetByUserID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		vpn = models.Vpn{
			UserID: userID,
		}
		switch protocol {
		case "vless":
			vpn.VlessLink = link
		case "trojan":
			vpn.TrojanLink = link
		}
		return s.vpnRepo.Create(vpn)
	}
	if err != nil {
		return models.Vpn{}, err
	}

	switch protocol {
	case "vless":
		vpn.VlessLink = link
	case "trojan":
		vpn.TrojanLink = link
	}
	return s.vpnRepo.Save(vpn)
}

func (s *VPNService) GetVPNByTelegramID(tgID int64) (models.Vpn, error) {
	logger.Info("telegram user lookup started", "component", "vpn_service", "operation", "get_vpn_by_telegram_id", "telegram_id", tgID)
	telegram, err := s.telegramRepo.FindByTgID(tgID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("telegram user lookup failed", "component", "vpn_service", "operation", "get_vpn_by_telegram_id", "telegram_id", tgID, "reason", "telegram_user_not_found")
		} else {
			logger.Error("telegram user lookup failed", err, "component", "vpn_service", "operation", "get_vpn_by_telegram_id", "telegram_id", tgID, "reason", "db_error")
		}
		return models.Vpn{}, err
	}
	logger.Info("telegram user found", "component", "vpn_service", "operation", "get_vpn_by_telegram_id", "telegram_id", tgID, "user_id", telegram.UserID)

	logger.Info("vpn client lookup started", "component", "vpn_service", "operation", "get_vpn_by_telegram_id", "telegram_id", tgID, "user_id", telegram.UserID)
	vpn, err := s.vpnRepo.GetByUserID(telegram.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("vpn client lookup failed", "component", "vpn_service", "operation", "get_vpn_by_telegram_id", "telegram_id", tgID, "user_id", telegram.UserID, "reason", "vpn_client_not_found")
		} else {
			logger.Error("vpn client lookup failed", err, "component", "vpn_service", "operation", "get_vpn_by_telegram_id", "telegram_id", tgID, "user_id", telegram.UserID, "reason", "db_error")
		}
		return models.Vpn{}, err
	}
	logger.Info("vpn client found", "component", "vpn_service", "operation", "get_vpn_by_telegram_id", "telegram_id", tgID, "user_id", telegram.UserID, "vpn_id", vpn.ID)
	return vpn, nil
}

func (s *VPNService) GetVPNLinkByProtocol(tgID int64, protocol string) (string, error) {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	logger.Info("vpn link lookup started", "component", "vpn_service", "operation", "get_vpn_link_by_protocol", "telegram_id", tgID, "protocol", protocol)
	vpn, err := s.GetVPNByTelegramID(tgID)
	if err != nil {
		return "", err
	}

	var link string
	switch protocol {
	case "vless":
		link = vpn.VlessLink
	case "trojan":
		link = vpn.TrojanLink
	default:
		logger.Warn("vpn link lookup failed", "component", "vpn_service", "operation", "get_vpn_link_by_protocol", "telegram_id", tgID, "protocol", protocol, "reason", "unsupported_protocol")
		return "", ErrUnsupportedProtocol
	}
	if strings.TrimSpace(link) == "" {
		logger.Warn("vpn link lookup failed", "component", "vpn_service", "operation", "get_vpn_link_by_protocol", "telegram_id", tgID, "protocol", protocol, "vpn_id", vpn.ID, "reason", "no_usable_profiles")
		return link, nil
	}
	logger.Info("vpn link lookup completed", "component", "vpn_service", "operation", "get_vpn_link_by_protocol", "telegram_id", tgID, "protocol", protocol, "vpn_id", vpn.ID, "reason", "usable_profiles_found")
	return link, nil
}
