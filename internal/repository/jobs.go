package repository

import (
	"errors"
	"vpnpanel/internal/models"

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

func (r *JobsRepo) UpdateJobResult(jobID uint, status, resultJSON, jobError string) (*models.Job, error) {
	var job models.Job
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&job, jobID).Error; err != nil {
			return err
		}

		var result *string
		if resultJSON != "" {
			result = &resultJSON
		}

		updates := map[string]any{
			"status":      status,
			"result_json": result,
			"error":       jobError,
		}
		if status == "failed" || status == "retrying" {
			updates["attempts"] = gorm.Expr("attempts + 1")
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
