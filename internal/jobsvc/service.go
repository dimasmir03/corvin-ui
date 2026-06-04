package jobsvc

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
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
	UserID            uint
	TechnicalClientID string
	Protocols         []string
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

		payload := broker.JobTask{
			BatchID:  batch.ID,
			ServerID: server.Id,
			Action:   ActionCollectNodeStats,
			Protocol: server.Type,
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
		return nil, nil, err
	}
	batch.Status = BatchStatusProcessing

	var payload broker.JobTask
	if err := json.Unmarshal(job.PayloadJSON, &payload); err != nil {
		return batch, &job, err
	}
	if s.producer != nil {
		if err := s.producer.PublishJob(payload); err != nil {
			_, _ = s.MarkJobFailed(job.ID, err.Error())
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
			BatchID:  batch.ID,
			ServerID: server.Id,
			Action:   ActionProbeNode,
			Protocol: server.Type,
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
		return nil, nil, err
	}
	batch.Status = BatchStatusProcessing

	var payload broker.JobTask
	if err := json.Unmarshal(job.PayloadJSON, &payload); err != nil {
		return batch, &job, err
	}
	if s.producer != nil {
		if err := s.producer.PublishJob(payload); err != nil {
			_, _ = s.MarkJobFailed(job.ID, err.Error())
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

	servers, err := s.serverRepo.GetAll()
	if err != nil {
		return nil, nil, err
	}

	userID := input.UserID
	batch := &models.JobBatch{
		Type:   BatchTypeCreateUserConfig,
		UserID: &userID,
		Status: BatchStatusPending,
	}

	var createdJobs []models.Job
	err = s.jobsRepo.DB().Transaction(func(tx *gorm.DB) error {
		txJobsRepo := s.jobsRepo.WithTx(tx)
		if err := txJobsRepo.CreateBatch(batch); err != nil {
			return err
		}

		for _, server := range servers {
			if !server.Enabled || server.Status == models.ServerStatusDisabled {
				continue
			}
			for _, protocol := range input.Protocols {
				if !serverSupportsProtocol(server, protocol) {
					continue
				}

				payload := broker.JobTask{
					BatchID:           batch.ID,
					ServerID:          server.Id,
					Action:            ActionCreateClient,
					Protocol:          protocol,
					UserID:            input.UserID,
					TechnicalClientID: input.TechnicalClientID,
				}

				payloadJSON, err := json.Marshal(payload)
				if err != nil {
					return err
				}

				serverID := server.Id
				job := models.Job{
					BatchID:        batch.ID,
					ServerID:       &serverID,
					Protocol:       protocol,
					Action:         ActionCreateClient,
					Status:         JobStatusPending,
					PayloadJSON:    datatypes.JSON(payloadJSON),
					IdempotencyKey: idempotencyKey(ActionCreateClient, input.UserID, server.Id, protocol),
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
		if s.producer != nil {
			if err := s.producer.PublishJob(payload); err != nil {
				_, _ = s.MarkJobFailed(job.ID, err.Error())
				return batch, createdJobs, err
			}
		}
		s.logAudit(audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "job.created",
			EntityType: "job",
			EntityID:   audit.StringID(job.ID),
			Status:     audit.StatusSuccess,
			Message:    "job created and queued",
			Metadata: map[string]any{
				"batch_id":  job.BatchID,
				"server_id": job.ServerID,
				"protocol":  job.Protocol,
				"action":    job.Action,
			},
		})
	}

	return batch, createdJobs, nil
}

func (s *Service) MarkJobProcessing(jobID uint) (*models.Job, error) {
	job, err := s.jobsRepo.MarkJobProcessing(jobID)
	if err != nil {
		return nil, err
	}
	if _, err := s.RecalculateBatchStatus(job.BatchID); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Service) MarkJobSuccess(jobID uint, result datatypes.JSON) (*models.Job, error) {
	job, err := s.jobsRepo.MarkJobSuccess(jobID, result)
	if err != nil {
		return nil, err
	}
	if _, err := s.RecalculateBatchStatus(job.BatchID); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Service) MarkJobFailed(jobID uint, jobError string) (*models.Job, error) {
	job, err := s.jobsRepo.MarkJobFailed(jobID, jobError)
	if err != nil {
		return nil, err
	}
	if _, err := s.RecalculateBatchStatus(job.BatchID); err != nil {
		return nil, err
	}
	s.logAudit(audit.Event{
		ActorType:  audit.ActorSystem,
		Action:     "job.failed",
		EntityType: "job",
		EntityID:   audit.StringID(job.ID),
		Status:     audit.StatusFailed,
		Message:    jobError,
	})
	return job, nil
}

func (s *Service) ApplyResult(event broker.JobResultEvent) (*models.JobBatch, *models.Job, error) {
	job, err := s.jobsRepo.GetJob(event.JobID)
	if err != nil {
		return nil, nil, err
	}

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
		return nil, nil, err
	}

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
	} else if event.ServerID != nil {
		serverID = *event.ServerID
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
	} else if event.ServerID != nil {
		serverID = *event.ServerID
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
		"server_id":        event.ServerID,
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

func serverSupportsProtocol(server models.Server, protocol string) bool {
	return strings.EqualFold(strings.TrimSpace(server.Type), strings.TrimSpace(protocol))
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
