package repository

import (
	"vpnpanel/internal/models"

	"gorm.io/gorm"
)

type AuditRepo struct {
	db *gorm.DB
}

func NewAuditRepo(db *gorm.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Create(log *models.AuditLog) error {
	return r.db.Create(log).Error
}
