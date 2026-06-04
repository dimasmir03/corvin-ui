package jobsvc

import (
	"encoding/json"
	"fmt"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	BatchTypeCreateUserConfig = "create_user_config"
	ActionCreateClient        = "create_client"

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
			for _, protocol := range input.Protocols {
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

		return txJobsRepo.UpdateBatchStatus(batch.ID, BatchStatusProcessing)
	})
	if err != nil {
		return nil, nil, err
	}
	batch.Status = BatchStatusProcessing

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

	batchStatus, err := s.RecalculateBatchStatus(job.BatchID)
	if err != nil {
		return nil, nil, err
	}

	batch := &models.JobBatch{ID: job.BatchID, Status: batchStatus}
	if jobStatus == JobStatusFailed {
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

	if jobStatus == JobStatusSuccess {
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
