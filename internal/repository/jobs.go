package repository

import (
	"errors"
	"vpnpanel/internal/models"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type JobsRepo struct {
	db *gorm.DB
}

func NewJobsRepo(db *gorm.DB) *JobsRepo {
	return &JobsRepo{db: db}
}

func (r *JobsRepo) WithTx(tx *gorm.DB) *JobsRepo {
	return &JobsRepo{db: tx}
}

func (r *JobsRepo) DB() *gorm.DB {
	return r.db
}

func (r *JobsRepo) CreateBatch(batch *models.JobBatch) error {
	return r.db.Create(batch).Error
}

func (r *JobsRepo) CreateJob(job *models.Job) error {
	return r.db.Create(job).Error
}

func (r *JobsRepo) GetJob(id uint) (*models.Job, error) {
	var job models.Job
	if err := r.db.First(&job, id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *JobsRepo) JobsByBatch(batchID uint) ([]models.Job, error) {
	var jobs []models.Job
	if err := r.db.Where("batch_id = ?", batchID).Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *JobsRepo) MarkJobProcessing(jobID uint) (*models.Job, error) {
	return r.updateJob(jobID, map[string]any{
		"status": models.JobStatusProcessing,
	})
}

func (r *JobsRepo) MarkJobSuccess(jobID uint, result datatypes.JSON) (*models.Job, error) {
	return r.updateJob(jobID, map[string]any{
		"status":      models.JobStatusSuccess,
		"result_json": &result,
		"error":       nil,
	})
}

func (r *JobsRepo) MarkJobFailed(jobID uint, jobError string) (*models.Job, error) {
	return r.updateJob(jobID, map[string]any{
		"status":   models.JobStatusFailed,
		"error":    &jobError,
		"attempts": gorm.Expr("attempts + 1"),
	})
}

func (r *JobsRepo) UpdateBatchStatus(batchID uint, status string) error {
	res := r.db.Model(&models.JobBatch{}).Where("id = ?", batchID).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("job batch not found")
	}
	return nil
}

func (r *JobsRepo) updateJob(jobID uint, updates map[string]any) (*models.Job, error) {
	var job models.Job
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&job, jobID).Error; err != nil {
			return err
		}
		if err := tx.Model(&job).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&job, jobID).Error
	})
	if err != nil {
		return nil, err
	}
	return &job, nil
}
