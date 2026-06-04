package jobsvc

import (
	"encoding/json"
	"fmt"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"gorm.io/gorm"
)

const (
	BatchTypeCreateUserConfig = "create_user_config"

	BatchStatusPending        = "pending"
	BatchStatusProcessing     = "processing"
	BatchStatusSuccess        = "success"
	BatchStatusPartialSuccess = "partial_success"
	BatchStatusFailed         = "failed"

	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusSuccess    = "success"
	JobStatusFailed     = "failed"
	JobStatusRetrying   = "retrying"

	ActionCreateClient = "create_client"
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

type ResultEvent struct {
	JobID          uint   `json:"job_id"`
	BatchID        uint   `json:"batch_id"`
	ServerID       int    `json:"server_id"`
	Status         string `json:"status"`
	RemoteClientID string `json:"remote_client_id"`
	ConfigLink     string `json:"config_link"`
	Error          string `json:"error"`
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

				job := models.Job{
					BatchID:        batch.ID,
					ServerID:       server.Id,
					Protocol:       protocol,
					Action:         ActionCreateClient,
					Status:         JobStatusPending,
					PayloadJSON:    string(payloadJSON),
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
				job.PayloadJSON = string(payloadJSON)
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
		if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
			return batch, createdJobs, err
		}
		if s.producer != nil {
			if err := s.producer.PublishJob(payload); err != nil {
				_ = s.MarkJobFailed(job.ID, err)
				return batch, createdJobs, err
			}
		}
		_ = s.audit.Log(audit.Event{
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

func (s *Service) ApplyResult(event ResultEvent) (*models.JobBatch, *models.Job, error) {
	resultJSON, err := json.Marshal(map[string]any{
		"job_id":           event.JobID,
		"batch_id":         event.BatchID,
		"server_id":        event.ServerID,
		"status":           event.Status,
		"remote_client_id": event.RemoteClientID,
		"config_link":      event.ConfigLink,
	})
	if err != nil {
		return nil, nil, err
	}

	jobStatus := normalizeJobResultStatus(event.Status)
	job, err := s.jobsRepo.UpdateJobResult(event.JobID, jobStatus, string(resultJSON), event.Error)
	if err != nil {
		return nil, nil, err
	}

	batchStatus, err := s.RecalculateBatchStatus(job.BatchID)
	if err != nil {
		return nil, nil, err
	}

	batch := &models.JobBatch{ID: job.BatchID, Status: batchStatus}
	if jobStatus == JobStatusFailed {
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorAgent,
			Action:     "job.failed",
			EntityType: "job",
			EntityID:   audit.StringID(job.ID),
			Status:     audit.StatusFailed,
			Message:    event.Error,
			Metadata:   event,
		})
	}

	if jobStatus == JobStatusRetrying {
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorAgent,
			Action:     "job.retried",
			EntityType: "job",
			EntityID:   audit.StringID(job.ID),
			Status:     audit.StatusSuccess,
			Message:    "job retry requested by agent",
			Metadata:   event,
		})
	}

	if jobStatus == JobStatusSuccess {
		_ = s.audit.Log(audit.Event{
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

func (s *Service) MarkJobFailed(jobID uint, cause error) error {
	job, err := s.jobsRepo.UpdateJobResult(jobID, JobStatusFailed, "", cause.Error())
	if err != nil {
		return err
	}
	if _, err := s.RecalculateBatchStatus(job.BatchID); err != nil {
		return err
	}
	return s.audit.Log(audit.Event{
		ActorType:  audit.ActorSystem,
		Action:     "job.failed",
		EntityType: "job",
		EntityID:   audit.StringID(job.ID),
		Status:     audit.StatusFailed,
		Message:    cause.Error(),
	})
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

func idempotencyKey(action string, userID uint, serverID int, protocol string) string {
	return fmt.Sprintf("%s:%d:%d:%s", action, userID, serverID, protocol)
}

func normalizeJobResultStatus(status string) string {
	switch status {
	case JobStatusSuccess:
		return JobStatusSuccess
	case JobStatusRetrying:
		return JobStatusRetrying
	case JobStatusProcessing:
		return JobStatusProcessing
	default:
		return JobStatusFailed
	}
}
