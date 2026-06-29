package jobsvc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	EndpointGroupDirect = "direct"
	EndpointGroupRU     = "ru"

	VPNProfileVLESS  = "vless"
	VPNProfileTrojan = "trojan"

	BatchTypeCreateUserConfig = "create_user_config"
	BatchTypeProbeNode        = "probe_node"
	BatchTypeCollectNodeStats = "collect_node_stats"

	ActionCreateClient     = "create_client"
	ActionProbeNode        = "probe_node"
	ActionCollectNodeStats = "collect_node_stats"

	BatchStatusPending        = models.JobBatchStatusPending
	BatchStatusProcessing     = models.JobBatchStatusProcessing
	BatchStatusSuccess        = models.JobBatchStatusSuccess
	BatchStatusPartialSuccess = models.JobBatchStatusPartialSuccess
	BatchStatusFailed         = models.JobBatchStatusFailed

	JobStatusPending    = models.JobStatusPending
	JobStatusProcessing = models.JobStatusProcessing
	JobStatusSuccess    = models.JobStatusSuccess
	JobStatusFailed     = models.JobStatusFailed
	JobStatusRetrying   = models.JobStatusRetrying
)

type Service struct {
	jobsRepo   *repository.JobsRepo
	serverRepo *repository.ServerRepo
	audit      *audit.Logger
	producer   JobPublisher
}

type JobPublisher interface {
	PublishJob(msg broker.JobTask) error
}

type CreateUserConfigInput struct {
	ProfileID         uint
	VPNClientID       uint
	UserID            uint
	TelegramID        int64
	ClientCode        string
	Email             string
	VlessUUID         string
	VlessFlow         string
	TrojanPassword    string
	Enable            bool
	ExpiryTime        int64
	TotalGB           int64
	TechnicalClientID string
	Protocols         []string
	TargetServerIDs   []string
}

func NewService(
	jobsRepo *repository.JobsRepo,
	serverRepo *repository.ServerRepo,
	auditLogger *audit.Logger,
	producer JobPublisher,
) *Service {
	return &Service{
		jobsRepo:   jobsRepo,
		serverRepo: serverRepo,
		audit:      auditLogger,
		producer:   producer,
	}
}

func (s *Service) CreateBatch(batchType string, userID *uint) (*models.JobBatch, error) {
	batch := &models.JobBatch{
		Type:   batchType,
		UserID: userID,
		Status: BatchStatusPending,
	}
	if err := s.jobsRepo.CreateBatch(batch); err != nil {
		return nil, err
	}
	return batch, nil
}

func (s *Service) CreateJob(batchID uint, serverID *int, protocol, action string, payload datatypes.JSON, idempotencyKey string) (*models.Job, error) {
	job := &models.Job{
		BatchID:        batchID,
		ServerID:       serverID,
		Protocol:       protocol,
		Action:         action,
		Status:         JobStatusPending,
		PayloadJSON:    payload,
		IdempotencyKey: idempotencyKey,
	}
	if err := s.jobsRepo.CreateJob(job); err != nil {
		return nil, err
	}
	if _, err := s.RecalculateBatchStatus(batchID); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Service) CollectNodeStats(serverID uint) (*models.JobBatch, *models.Job, error) {
	server, err := s.serverRepo.GetByID(int(serverID))
	if err != nil {
		return nil, nil, err
	}
	if !server.Enabled || server.Status == models.ServerStatusDisabled {
		return nil, nil, fmt.Errorf("server is disabled")
	}

	batch := &models.JobBatch{
		Type:   BatchTypeCollectNodeStats,
		Status: BatchStatusPending,
	}
	var job models.Job
	err = s.jobsRepo.DB().Transaction(func(tx *gorm.DB) error {
		txJobsRepo := s.jobsRepo.WithTx(tx)
		if err := txJobsRepo.CreateBatch(batch); err != nil {
			return err
		}
		logger.Info("job batch created", "component", "jobsvc", "operation", "collect_node_stats", "batch_id", batch.ID, "server_id", server.Id, "action", ActionCollectNodeStats)

		payload := broker.JobTask{
			BatchID:        batch.ID,
			ServerID:       serverAgentID(*server),
			TargetServerID: serverAgentID(*server),
			NodeID:         serverAgentID(*server),
			Action:         ActionCollectNodeStats,
			Protocol:       server.Type,
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		serverID := server.Id
		job = models.Job{
			BatchID:        batch.ID,
			ServerID:       &serverID,
			Protocol:       server.Type,
			Action:         ActionCollectNodeStats,
			Status:         JobStatusPending,
			PayloadJSON:    datatypes.JSON(payloadJSON),
			IdempotencyKey: fmt.Sprintf("%s:%d:%d", ActionCollectNodeStats, server.Id, time.Now().UnixNano()),
		}
		if err := txJobsRepo.CreateJob(&job); err != nil {
			return err
		}

		payload.JobID = job.ID
		payloadJSON, err = json.Marshal(payload)
		if err != nil {
			return err
		}
		job.PayloadJSON = datatypes.JSON(payloadJSON)
		if err := tx.Model(&models.Job{}).Where("id = ?", job.ID).Update("payload_json", job.PayloadJSON).Error; err != nil {
			return err
		}

		return txJobsRepo.UpdateBatchStatus(batch.ID, BatchStatusProcessing)
	})
	if err != nil {
		logger.Error("job collect node stats transaction failed", err, "component", "jobsvc", "operation", "collect_node_stats", "server_id", server.Id)
		return nil, nil, err
	}
	batch.Status = BatchStatusProcessing

	var payload broker.JobTask
	if err := json.Unmarshal(job.PayloadJSON, &payload); err != nil {
		return batch, &job, err
	}
	if s.producer != nil {
		if err := s.producer.PublishJob(payload); err != nil {
			_, _ = s.jobsRepo.MarkJobFailed(job.ID, err.Error())
			return batch, &job, err
		}
	}

	return batch, &job, nil
}

func (s *Service) ProbeServer(serverID int) (*models.JobBatch, *models.Job, error) {
	server, err := s.serverRepo.GetByID(serverID)
	if err != nil {
		return nil, nil, err
	}
	if !server.Enabled || server.Status == models.ServerStatusDisabled {
		return nil, nil, fmt.Errorf("server is disabled")
	}

	batch := &models.JobBatch{
		Type:   BatchTypeProbeNode,
		Status: BatchStatusPending,
	}
	var job models.Job
	err = s.jobsRepo.DB().Transaction(func(tx *gorm.DB) error {
		txJobsRepo := s.jobsRepo.WithTx(tx)
		if err := txJobsRepo.CreateBatch(batch); err != nil {
			return err
		}

		payload := broker.JobTask{
			BatchID:        batch.ID,
			ServerID:       serverAgentID(*server),
			TargetServerID: serverAgentID(*server),
			NodeID:         serverAgentID(*server),
			Action:         ActionProbeNode,
			Protocol:       server.Type,
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		serverID := server.Id
		job = models.Job{
			BatchID:        batch.ID,
			ServerID:       &serverID,
			Protocol:       server.Type,
			Action:         ActionProbeNode,
			Status:         JobStatusPending,
			PayloadJSON:    datatypes.JSON(payloadJSON),
			IdempotencyKey: fmt.Sprintf("%s:%d:%d", ActionProbeNode, server.Id, time.Now().UnixNano()),
		}
		if err := txJobsRepo.CreateJob(&job); err != nil {
			return err
		}

		payload.JobID = job.ID
		payloadJSON, err = json.Marshal(payload)
		if err != nil {
			return err
		}
		job.PayloadJSON = datatypes.JSON(payloadJSON)
		if err := tx.Model(&models.Job{}).Where("id = ?", job.ID).Update("payload_json", job.PayloadJSON).Error; err != nil {
			return err
		}

		return txJobsRepo.UpdateBatchStatus(batch.ID, BatchStatusProcessing)
	})
	if err != nil {
		logger.Error("job probe node transaction failed", err, "component", "jobsvc", "operation", "probe_node", "server_id", server.Id)
		return nil, nil, err
	}
	batch.Status = BatchStatusProcessing

	var payload broker.JobTask
	if err := json.Unmarshal(job.PayloadJSON, &payload); err != nil {
		return batch, &job, err
	}
	if s.producer != nil {
		if err := s.producer.PublishJob(payload); err != nil {
			_, _ = s.jobsRepo.MarkJobFailed(job.ID, err.Error())
			return batch, &job, err
		}
	}
	s.logAudit(audit.Event{
		ActorType:  audit.ActorAdmin,
		Action:     "server.probe.requested",
		EntityType: "server",
		EntityID:   audit.StringID(server.Id),
		Status:     audit.StatusSuccess,
		Message:    "server probe requested",
		Metadata: map[string]any{
			"batch_id": batch.ID,
			"job_id":   job.ID,
		},
	})

	return batch, &job, nil
}

func (s *Service) CreateUserConfig(input CreateUserConfigInput) (*models.JobBatch, []models.Job, error) {
	if len(input.Protocols) == 0 {
		input.Protocols = []string{"vless", "trojan"}
	}
	logger.Info("job create user config started", "component", "jobsvc", "operation", "create_user_config", "user_id", input.UserID, "tg_id", input.TelegramID, "client_code", input.ClientCode, "profile_id", input.ProfileID, "protocols", strings.Join(input.Protocols, ","))

	targetServerIDs := make([]string, 0, len(input.TargetServerIDs))
	for _, serverID := range input.TargetServerIDs {
		serverID = strings.TrimSpace(serverID)
		if serverID != "" {
			targetServerIDs = append(targetServerIDs, serverID)
		}
	}
	if len(targetServerIDs) == 0 {
		logger.Warn("job create user config rejected", "component", "jobsvc", "operation", "create_user_config", "user_id", input.UserID, "tg_id", input.TelegramID, "client_code", input.ClientCode, "reason", "target_servers_required")
		return nil, nil, fmt.Errorf("target servers are required")
	}
	logger.Info("job create user config target servers found", "component", "jobsvc", "operation", "create_user_config", "user_id", input.UserID, "tg_id", input.TelegramID, "client_code", input.ClientCode, "servers_count", len(targetServerIDs))

	userID := input.UserID
	batch := &models.JobBatch{Type: BatchTypeCreateUserConfig, UserID: &userID, Status: BatchStatusPending}
	var createdJobs []models.Job
	err := s.jobsRepo.DB().Transaction(func(tx *gorm.DB) error {
		txJobsRepo := s.jobsRepo.WithTx(tx)
		if err := txJobsRepo.CreateBatch(batch); err != nil {
			return err
		}
		logger.Info("job batch created", "component", "jobsvc", "operation", "create_user_config", "batch_id", batch.ID, "user_id", input.UserID, "tg_id", input.TelegramID, "client_code", input.ClientCode)
		for _, serverID := range targetServerIDs {
			for _, protocol := range input.Protocols {
				profile := normalizeProfile(protocol)
				targetGroup := endpointGroupForProfile(profile)
				payload := createClientJobTask(batch.ID, serverID, profile, targetGroup, input)
				payloadJSON, err := json.Marshal(payload)
				if err != nil {
					return err
				}
				job := models.Job{BatchID: batch.ID, TargetServerID: serverID, ProfileID: input.ProfileID, Protocol: profile, Action: ActionCreateClient, Status: JobStatusPending, PayloadJSON: datatypes.JSON(payloadJSON), IdempotencyKey: idempotencyKeyForTarget(ActionCreateClient, input.UserID, input.ProfileID, serverID, profile)}
				if err := txJobsRepo.CreateJob(&job); err != nil {
					return err
				}
				logger.Info("create client job created", "component", "jobsvc", "operation", "create_user_config", "batch_id", batch.ID, "job_id", job.ID, "profile_id", input.ProfileID, "server_id", serverID, "profile", profile, "target_group", targetGroup, "action", ActionCreateClient, "client_code", input.ClientCode)
				payload.JobID = job.ID
				payloadJSON, err = json.Marshal(payload)
				if err != nil {
					return err
				}
				job.PayloadJSON = datatypes.JSON(payloadJSON)
				if err := tx.Model(&models.Job{}).Where("id = ?", job.ID).Update("payload_json", job.PayloadJSON).Error; err != nil {
					return err
				}
				createdJobs = append(createdJobs, job)
			}
		}
		status := BatchStatusProcessing
		if len(createdJobs) == 0 {
			status = BatchStatusPending
		}
		return txJobsRepo.UpdateBatchStatus(batch.ID, status)
	})
	if err != nil {
		logger.Error("job create user config transaction failed", err, "component", "jobsvc", "operation", "create_user_config", "user_id", input.UserID, "tg_id", input.TelegramID, "client_code", input.ClientCode)
		return nil, nil, err
	}
	batch.Status = BatchStatusProcessing
	if len(createdJobs) == 0 {
		batch.Status = BatchStatusPending
	}
	for _, job := range createdJobs {
		var payload broker.JobTask
		if err := json.Unmarshal(job.PayloadJSON, &payload); err != nil {
			return batch, createdJobs, err
		}
		logger.Info("create client job publish started", "component", "jobsvc", "operation", "create_user_config", "batch_id", batch.ID, "job_id", job.ID, "profile_id", payload.ProfileID, "server_id", payload.ServerID, "exchange", "corvin.job.commands", "routing_key", "create.server."+payload.ServerID, "profile", payload.Profile, "target_group", payload.TargetGroup, "protocol", payload.Protocol, "client_code", payload.ClientCode)
		if s.producer != nil {
			if err := s.producer.PublishJob(payload); err != nil {
				_, _ = s.jobsRepo.MarkJobFailed(job.ID, err.Error())
				return batch, createdJobs, err
			}
		}
		logger.Info("create client job published", "component", "jobsvc", "operation", "create_user_config", "batch_id", batch.ID, "job_id", job.ID, "profile_id", payload.ProfileID, "server_id", payload.ServerID, "routing_key", "create.server."+payload.ServerID, "profile", payload.Profile, "target_group", payload.TargetGroup, "protocol", payload.Protocol, "client_code", payload.ClientCode)
	}
	logger.Info("job create user config finished", "component", "jobsvc", "operation", "create_user_config", "batch_id", batch.ID, "user_id", input.UserID, "tg_id", input.TelegramID, "client_code", input.ClientCode, "jobs_count", len(createdJobs), "status", batch.Status, "reason", "success")
	return batch, createdJobs, nil
}

func (s *Service) ExistingCreateClientTargets(profileID uint) (map[string]struct{}, error) {
	if profileID == 0 {
		return map[string]struct{}{}, nil
	}
	return s.jobsRepo.ExistingCreateClientTargets(profileID)
}

func (s *Service) ApplyResult(event broker.JobResultEvent) (*models.JobBatch, *models.Job, error) {
	logger.Info("job_result apply started", "component", "jobsvc", "operation", "apply_result", "job_id", event.JobID, "batch_id", event.BatchID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "status", event.Status)
	job, err := s.jobsRepo.GetJob(event.JobID)
	if err != nil {
		logger.Error("job_result job lookup failed", err, "component", "jobsvc", "operation", "apply_result", "job_id", event.JobID, "batch_id", event.BatchID)
		return nil, nil, err
	}
	logger.Info("job_result job found", "component", "jobsvc", "operation", "apply_result", "job_id", job.ID, "batch_id", job.BatchID, "action", job.Action, "old_status", job.Status)

	jobStatus := normalizeJobResultStatus(event.Status)
	if jobStatus == JobStatusSuccess {
		resultJSON, err := resultPayload(event)
		if err != nil {
			return nil, nil, err
		}
		job, err = s.jobsRepo.MarkJobSuccess(job.ID, resultJSON)
	} else {
		jobError := "job failed"
		if event.Error != nil && *event.Error != "" {
			jobError = *event.Error
		}
		job, err = s.jobsRepo.MarkJobFailed(job.ID, jobError)
	}
	if err != nil {
		logger.Error("job_result status update failed", err, "component", "jobsvc", "operation", "apply_result", "job_id", event.JobID, "batch_id", event.BatchID, "new_status", jobStatus)
		return nil, nil, err
	}
	logger.Info("job_result status updated", "component", "jobsvc", "operation", "apply_result", "job_id", job.ID, "batch_id", job.BatchID, "new_status", job.Status)

	switch job.Action {
	case ActionProbeNode:
		if err := s.applyProbeResult(job, event, jobStatus); err != nil {
			return nil, nil, err
		}
	case ActionCollectNodeStats:
		if err := s.applyNodeStatsResult(job, event, jobStatus); err != nil {
			return nil, nil, err
		}
	}

	batchStatus, err := s.RecalculateBatchStatus(job.BatchID)
	if err != nil {
		return nil, nil, err
	}

	logger.Info("job_result batch status recalculated", "component", "jobsvc", "operation", "apply_result", "job_id", job.ID, "batch_id", job.BatchID, "new_status", batchStatus)
	batch := &models.JobBatch{ID: job.BatchID, Status: batchStatus}
	if job.Action != ActionProbeNode && job.Action != ActionCollectNodeStats && jobStatus == JobStatusFailed {
		s.logAudit(audit.Event{
			ActorType:  audit.ActorAgent,
			Action:     "job.failed",
			EntityType: "job",
			EntityID:   audit.StringID(job.ID),
			Status:     audit.StatusFailed,
			Message:    valueOrEmpty(event.Error),
			Metadata:   event,
		})
	}

	if job.Action != ActionProbeNode && job.Action != ActionCollectNodeStats && jobStatus == JobStatusSuccess {
		s.logAudit(audit.Event{
			ActorType:  audit.ActorAgent,
			Action:     "vpn.client.created",
			EntityType: "job",
			EntityID:   audit.StringID(job.ID),
			Status:     audit.StatusSuccess,
			Message:    "vpn client created by agent",
			Metadata:   event,
		})
	}

	logger.Info("job_result apply finished", "component", "jobsvc", "operation", "apply_result", "job_id", job.ID, "batch_id", job.BatchID, "new_status", job.Status, "reason", "success")
	return batch, job, nil
}

func (s *Service) RecalculateBatchStatus(batchID uint) (string, error) {
	jobs, err := s.jobsRepo.JobsByBatch(batchID)
	if err != nil {
		return "", err
	}
	status := CalculateBatchStatus(jobs)
	if err := s.jobsRepo.UpdateBatchStatus(batchID, status); err != nil {
		return "", err
	}
	return status, nil
}

func CalculateBatchStatus(jobs []models.Job) string {
	if len(jobs) == 0 {
		return BatchStatusPending
	}

	successCount := 0
	failedCount := 0
	activeCount := 0

	for _, job := range jobs {
		switch job.Status {
		case JobStatusSuccess:
			successCount++
		case JobStatusFailed:
			failedCount++
		default:
			activeCount++
		}
	}

	if activeCount > 0 {
		return BatchStatusProcessing
	}
	if successCount == len(jobs) {
		return BatchStatusSuccess
	}
	if failedCount == len(jobs) {
		return BatchStatusFailed
	}
	return BatchStatusPartialSuccess
}

func (s *Service) applyNodeStatsResult(job *models.Job, event broker.JobResultEvent, jobStatus string) error {
	serverID := 0
	if job.ServerID != nil {
		serverID = *job.ServerID
	} else if parsedServerID, ok := parseLegacyNumericServerID(event.ServerID); ok {
		serverID = parsedServerID
	}
	if serverID == 0 {
		return fmt.Errorf("node stats result has no server_id")
	}

	server, err := s.serverRepo.GetByID(serverID)
	if err != nil {
		return err
	}

	stats := parseNodeStatsResult(event)
	if stats.Error == nil && event.Error != nil && *event.Error != "" {
		stats.Error = event.Error
	}
	newStatus := nodeStatusFromStats(jobStatus, stats)
	observedAt := time.Now()
	if err := s.serverRepo.ApplyNodeStats(serverID, repository.NodeStatsUpdate{
		OnlineCount:   stats.OnlineCount,
		UploadBytes:   stats.UploadBytes,
		DownloadBytes: stats.DownloadBytes,
		TotalBytes:    stats.TotalBytes,
		PanelStatus:   stats.PanelStatus,
		XrayStatus:    stats.XrayStatus,
		PanelVersion:  stats.PanelVersion,
		XrayVersion:   stats.XrayVersion,
		AgentVersion:  stats.AgentVersion,
		Error:         stats.Error,
		RawJSON:       rawNodeStatsJSON(event),
		ObservedAt:    observedAt,
		Status:        newStatus,
	}); err != nil {
		return err
	}

	if server.Status != newStatus {
		s.logAudit(audit.Event{
			ActorType:  audit.ActorAgent,
			Action:     "server.status.changed",
			EntityType: "server",
			EntityID:   audit.StringID(serverID),
			Status:     audit.StatusSuccess,
			Message:    "server status changed from node stats",
			OldValue:   map[string]any{"status": server.Status},
			NewValue:   map[string]any{"status": newStatus},
			Metadata: map[string]any{
				"job_id":   job.ID,
				"batch_id": job.BatchID,
			},
		})
	}
	return nil
}

type nodeStatsResultPayload struct {
	OnlineCount   int     `json:"online_count"`
	UploadBytes   int64   `json:"upload_bytes"`
	DownloadBytes int64   `json:"download_bytes"`
	TotalBytes    int64   `json:"total_bytes"`
	PanelStatus   *string `json:"panel_status"`
	XrayStatus    *string `json:"xray_status"`
	PanelVersion  *string `json:"panel_version"`
	XrayVersion   *string `json:"xray_version"`
	AgentVersion  *string `json:"agent_version"`
	Error         *string `json:"error"`
}

func parseNodeStatsResult(event broker.JobResultEvent) nodeStatsResultPayload {
	if event.ResultJSON == nil || len(*event.ResultJSON) == 0 {
		return nodeStatsResultPayload{}
	}
	var result nodeStatsResultPayload
	_ = json.Unmarshal(*event.ResultJSON, &result)
	return result
}

func rawNodeStatsJSON(event broker.JobResultEvent) []byte {
	if event.ResultJSON == nil {
		return nil
	}
	return []byte(*event.ResultJSON)
}

func nodeStatusFromStats(jobStatus string, stats nodeStatsResultPayload) string {
	if jobStatus != JobStatusSuccess {
		return models.ServerStatusOffline
	}
	if stats.Error != nil && *stats.Error != "" {
		return models.ServerStatusDegraded
	}
	if !componentStatusHealthy(stats.PanelStatus) || !componentStatusHealthy(stats.XrayStatus) {
		return models.ServerStatusDegraded
	}
	return models.ServerStatusOnline
}

func componentStatusHealthy(status *string) bool {
	if status == nil || *status == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(*status)) {
	case "ok", "online", "running", "healthy", "active":
		return true
	default:
		return false
	}
}

func (s *Service) applyProbeResult(job *models.Job, event broker.JobResultEvent, jobStatus string) error {
	serverID := 0
	if job.ServerID != nil {
		serverID = *job.ServerID
	} else if parsedServerID, ok := parseLegacyNumericServerID(event.ServerID); ok {
		serverID = parsedServerID
	}
	if serverID == 0 {
		return fmt.Errorf("probe result has no server_id")
	}

	probedAt := time.Now()
	probeResult := parseProbeResult(event)
	if jobStatus == JobStatusSuccess {
		if err := s.serverRepo.UpdateProbeSuccess(serverID, probedAt, repository.ServerProbeVersions{
			PanelVersion: probeResult.PanelVersion,
			XrayVersion:  probeResult.XrayVersion,
			AgentVersion: probeResult.AgentVersion,
		}); err != nil {
			return err
		}
		s.logAudit(audit.Event{
			ActorType:  audit.ActorAgent,
			Action:     "server.probe.success",
			EntityType: "server",
			EntityID:   audit.StringID(serverID),
			Status:     audit.StatusSuccess,
			Message:    "server probe succeeded",
			Metadata:   event,
		})
		return nil
	}

	status := models.ServerStatusOffline
	if probeResult.Status == models.ServerStatusDegraded {
		status = models.ServerStatusDegraded
	}
	jobError := "probe failed"
	if event.Error != nil && *event.Error != "" {
		jobError = *event.Error
	}
	if err := s.serverRepo.UpdateProbeFailed(serverID, probedAt, status, jobError); err != nil {
		return err
	}
	s.logAudit(audit.Event{
		ActorType:  audit.ActorAgent,
		Action:     "server.probe.failed",
		EntityType: "server",
		EntityID:   audit.StringID(serverID),
		Status:     audit.StatusFailed,
		Message:    jobError,
		Metadata:   event,
	})
	return nil
}

type probeResultPayload struct {
	Status       string  `json:"status"`
	PanelVersion *string `json:"panel_version"`
	XrayVersion  *string `json:"xray_version"`
	AgentVersion *string `json:"agent_version"`
}

func parseProbeResult(event broker.JobResultEvent) probeResultPayload {
	if event.ResultJSON == nil || len(*event.ResultJSON) == 0 {
		return probeResultPayload{}
	}
	var result probeResultPayload
	_ = json.Unmarshal(*event.ResultJSON, &result)
	return result
}

func resultPayload(event broker.JobResultEvent) (datatypes.JSON, error) {
	if event.ResultJSON != nil {
		return datatypes.JSON(*event.ResultJSON), nil
	}

	data, err := json.Marshal(map[string]any{
		"job_id":           event.JobID,
		"batch_id":         event.BatchID,
		"server_id":        event.EffectiveServerID(),
		"status":           event.Status,
		"remote_client_id": event.RemoteClientID,
		"config_link":      event.ConfigLink,
	})
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(data), nil
}

func (s *Service) logAudit(event audit.Event) {
	if s.audit != nil {
		_ = s.audit.Log(event)
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case VPNProfileTrojan:
		return VPNProfileTrojan
	default:
		return VPNProfileVLESS
	}
}

func endpointGroupForProfile(profile string) string {
	switch normalizeProfile(profile) {
	case VPNProfileTrojan:
		return EndpointGroupRU
	default:
		return EndpointGroupDirect
	}
}

func serverSupportsProtocol(server models.Server, protocol string) bool {
	return strings.EqualFold(strings.TrimSpace(server.Type), strings.TrimSpace(protocol))
}

func serverSupportsEndpointProfile(server models.Server, profile string, targetGroup string) bool {
	return serverSupportsProtocol(server, profile) && strings.EqualFold(strings.TrimSpace(server.NodeRole), targetGroup)
}

func createClientJobTask(batchID uint, serverID string, profile string, targetGroup string, input CreateUserConfigInput) broker.JobTask {
	clientCode := strings.TrimSpace(input.ClientCode)
	if clientCode == "" {
		clientCode = strings.TrimSpace(input.TechnicalClientID)
	}
	email := strings.TrimSpace(input.Email)
	if email == "" {
		email = clientCode
	}
	enable := input.Enable
	if !enable {
		enable = true
	}

	technicalClientID := strings.TrimSpace(input.TechnicalClientID)
	if technicalClientID == "" {
		switch normalizeProfile(profile) {
		case VPNProfileTrojan:
			technicalClientID = input.TrojanPassword
		default:
			technicalClientID = input.VlessUUID
		}
	}

	return broker.JobTask{
		EventType:         ActionCreateClient,
		BatchID:           batchID,
		ServerID:          serverID,
		TargetServerID:    serverID,
		Action:            ActionCreateClient,
		CommandType:       ActionCreateClient,
		Protocol:          normalizeProfile(profile),
		ProfileID:         input.ProfileID,
		VPNClientID:       input.VPNClientID,
		Profile:           normalizeProfile(profile),
		TargetGroup:       targetGroup,
		TelegramID:        input.TelegramID,
		UserID:            input.UserID,
		ClientCode:        clientCode,
		Email:             email,
		Enable:            enable,
		ExpiryTime:        input.ExpiryTime,
		TotalGB:           input.TotalGB,
		TechnicalClientID: technicalClientID,
		CreatedAt:         time.Now().UTC(),
		Credentials: broker.VPNCredentials{
			VLESS: broker.VLESSCredentials{
				ID:   input.VlessUUID,
				Flow: input.VlessFlow,
			},
			Trojan: broker.TrojanCredentials{
				Password: input.TrojanPassword,
			},
		},
	}
}

func serverAgentID(server models.Server) string {
	if strings.TrimSpace(server.Name) != "" {
		return strings.TrimSpace(server.Name)
	}
	return strconv.Itoa(server.Id)
}

func parseLegacyNumericServerID(value string) (int, bool) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed == 0 {
		return 0, false
	}
	return parsed, true
}

func idempotencyKeyForTarget(action string, userID uint, profileID uint, serverID string, protocol string) string {
	return fmt.Sprintf("%s:%d:%d:%s:%s:%d", action, userID, profileID, serverID, protocol, time.Now().UnixNano())
}

func idempotencyKey(action string, userID uint, serverID int, protocol string) string {
	return fmt.Sprintf("%s:%d:%d:%s", action, userID, serverID, protocol)
}

func normalizeJobResultStatus(status string) string {
	switch status {
	case JobStatusSuccess:
		return JobStatusSuccess
	default:
		return JobStatusFailed
	}
}
