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
	TgID     int64
	Protocol string
	BatchID  uint
	JobID    uint
}

type VPNReadyNotification struct {
	TgID     int64
	Protocol string
	Link     string
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

func (s *VPNService) RequestCreateVPN(input RequestCreateVPNInput) (*RequestCreateVPNResult, error) {
	profiles, err := requestedVPNProfiles(input.Protocol)
	if err != nil {
		return nil, err
	}

	telegram, err := s.telegramRepo.GetTelegramByTgID(input.TgID)
	if err != nil {
		return nil, err
	}
	logger.Info("vpn create requested", "user_id", telegram.UserID, "telegram_id", input.TgID, "profile", strings.Join(profiles, ","))

	client, created, err := s.vpnRepo.GetOrCreateVPNClient(telegram.UserID, input.TgID)
	if err != nil {
		return nil, err
	}
	if created {
		logger.Info("vpn canonical client created", "user_id", telegram.UserID, "telegram_id", input.TgID, "client_code", client.ClientCode)
	} else {
		logger.Info("vpn canonical client reused", "user_id", telegram.UserID, "telegram_id", input.TgID, "client_code", client.ClientCode)
	}

	result := &RequestCreateVPNResult{TgID: input.TgID, Protocol: strings.Join(profiles, ",")}
	for _, profileName := range profiles {
		profile, batchID, jobID, err := s.ensureVPNProfile(telegram.UserID, input.TgID, client, profileName)
		if err != nil {
			return nil, err
		}
		if result.BatchID == 0 {
			result.BatchID = batchID
		}
		if result.JobID == 0 {
			result.JobID = jobID
		}
		if len(profiles) == 1 {
			result.Protocol = profile.Profile
		}
	}
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

func (s *VPNService) ensureVPNProfile(userID uint, tgID int64, client models.VPNClient, profileName string) (models.VPNProfile, uint, uint, error) {
	endpointGroup := endpointGroupForVPNProfile(profileName)
	group, err := s.vpnRepo.GetOrCreateEndpointGroup(endpointGroup)
	if err != nil {
		return models.VPNProfile{}, 0, 0, err
	}

	existing, err := s.vpnRepo.GetProfile(client.ID, profileName)
	if err == nil {
		logger.Info("vpn profile reused", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "target_group", endpointGroup, "profile_id", existing.ID)
		return existing, 0, 0, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.VPNProfile{}, 0, 0, err
	}

	nodes, err := s.vpnRepo.EnabledNodesByGroup(endpointGroup)
	if err != nil {
		return models.VPNProfile{}, 0, 0, err
	}

	profileStatus := models.VPNProfileStatusPending
	lastError := ""
	if len(nodes) == 0 {
		profileStatus = models.VPNProfileStatusFailed
		lastError = "no enabled nodes in endpoint group"
	}

	profile := models.VPNProfile{
		VPNClientID:   client.ID,
		Profile:       profileName,
		EndpointGroup: endpointGroup,
		Protocol:      group.Protocol,
		Status:        profileStatus,
		FinalLink:     buildProfileLink(group, client, profileName),
		LastError:     lastError,
	}
	profile, err = s.vpnRepo.CreateProfileWithNodes(profile, nodes)
	if err != nil {
		return models.VPNProfile{}, 0, 0, err
	}
	logger.Info("vpn profile created", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "target_group", endpointGroup, "profile_id", profile.ID, "nodes_count", len(nodes))
	logger.Info("vpn profile pending nodes created", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "target_group", endpointGroup, "profile_id", profile.ID, "nodes_count", len(nodes))

	if len(nodes) == 0 {
		return profile, 0, 0, nil
	}
	if s.jobs == nil {
		return models.VPNProfile{}, 0, 0, errors.New("jobs service is not configured")
	}

	batch, jobs, err := s.jobs.CreateUserConfig(jobsvc.CreateUserConfigInput{
		ProfileID:      profile.ID,
		UserID:         userID,
		TelegramID:     tgID,
		ClientCode:     client.ClientCode,
		Email:          client.Email,
		VlessUUID:      client.VlessUUID,
		VlessFlow:      group.Flow,
		TrojanPassword: client.TrojanPassword,
		Enable:         true,
		Protocols:      []string{profileName},
	})
	if err != nil {
		_ = s.vpnRepo.TouchProfilePublishError(profile.ID, err.Error())
		logger.Error("vpn create job publish failed", err, "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "target_group", endpointGroup, "profile_id", profile.ID, "nodes_count", len(nodes))
		return models.VPNProfile{}, 0, 0, &VPNFlowError{Kind: VPNErrorKindJobs, Err: err}
	}

	var jobID uint
	if len(jobs) > 0 {
		jobID = jobs[0].ID
	}
	logger.Info("vpn create job published", "user_id", userID, "telegram_id", tgID, "client_code", client.ClientCode, "profile", profileName, "target_group", endpointGroup, "profile_id", profile.ID, "batch_id", batch.ID, "job_id", jobID, "nodes_count", len(nodes))
	return profile, batch.ID, jobID, nil
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
		logger.Error("vpn job_result unknown status", nil, "job_id", event.JobID, "profile_id", event.ProfileID, "node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
		return nil, nil
	}
	if event.ProfileID == 0 {
		logger.Warn("vpn job_result unknown profile", "job_id", event.JobID, "profile_id", event.ProfileID, "node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
		return nil, nil
	}

	logger.Info("job_result received", "job_id", event.JobID, "profile_id", event.ProfileID, "node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)

	profile, err := s.vpnRepo.GetProfileByID(event.ProfileID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Warn("vpn job_result unknown profile", "job_id", event.JobID, "profile_id", event.ProfileID, "node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	nodeID := strings.TrimSpace(event.NodeID)
	if nodeID == "" {
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
	node, duplicate, err := s.vpnRepo.ApplyProfileNodeResult(profile, nodeID, protocol, status, event.InboundID, valueOrEmptyString(event.Error), appliedAt)
	if err != nil {
		return nil, err
	}
	if duplicate {
		logger.Info("vpn job_result ignored duplicate", "job_id", event.JobID, "profile_id", profile.ID, "node_id", nodeID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", protocol, "status", status, "client_code", profile.VPNClient.ClientCode)
	}
	logger.Info("vpn profile node result applied", "job_id", event.JobID, "profile_id", profile.ID, "node_id", node.NodeID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", node.Protocol, "status", node.Status, "client_code", profile.VPNClient.ClientCode)

	profile, err = s.vpnRepo.GetProfileByID(profile.ID)
	if err != nil {
		return nil, err
	}
	newStatus, profileError := recalculateVPNProfileStatus(profile.Nodes)
	finalLink := profile.FinalLink
	if isUsableVPNProfileStatus(newStatus) && strings.TrimSpace(finalLink) == "" {
		group, err := s.vpnRepo.GetEndpointGroup(profile.EndpointGroup)
		if err != nil {
			return nil, err
		}
		finalLink = buildProfileLink(group, profile.VPNClient, profile.Profile)
		logger.Info("vpn final link generated", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", newStatus, "client_code", profile.VPNClient.ClientCode)
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
	logger.Info("vpn profile status recalculated", "job_id", event.JobID, "profile_id", profile.ID, "node_id", nodeID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", profile.Status, "client_code", profile.VPNClient.ClientCode)

	if profile.Status == models.VPNProfileStatusPartial {
		logger.Warn("vpn admin notified partial", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", profile.Status, "client_code", profile.VPNClient.ClientCode, "failed_nodes", failedProfileNodes(profile.Nodes))
	}
	if profile.Status == models.VPNProfileStatusFailed {
		logger.Warn("vpn profile failed", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", profile.Status, "client_code", profile.VPNClient.ClientCode, "failed_nodes", failedProfileNodes(profile.Nodes))
	}

	if shouldNotifyUser {
		telegram, err := s.telegramRepo.GetByUserID(profile.VPNClient.UserID)
		if err != nil {
			return nil, err
		}
		logger.Info("vpn user notified", "job_id", event.JobID, "profile_id", profile.ID, "profile", profile.Profile, "target_group", profile.EndpointGroup, "protocol", profile.Protocol, "status", profile.Status, "client_code", profile.VPNClient.ClientCode, "tg_id", telegram.TgID)
		return &VPNReadyNotification{TgID: telegram.TgID, Protocol: profile.Profile, Link: finalLink}, nil
	}

	return nil, nil
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
	telegram, err := s.telegramRepo.FindByTgID(tgID)
	if err != nil {
		return models.Vpn{}, err
	}

	return s.vpnRepo.GetByUserID(telegram.UserID)
}

func (s *VPNService) GetVPNLinkByProtocol(tgID int64, protocol string) (string, error) {
	vpn, err := s.GetVPNByTelegramID(tgID)
	if err != nil {
		return "", err
	}

	switch protocol {
	case "vless":
		return vpn.VlessLink, nil
	case "trojan":
		return vpn.TrojanLink, nil
	default:
		return "", ErrUnsupportedProtocol
	}
}
