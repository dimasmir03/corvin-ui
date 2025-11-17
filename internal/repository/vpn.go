package repository

import (
	"gorm.io/gorm"
)

type VpnRepo struct {
	DB *gorm.DB
}

func NewVpnRepo(db *gorm.DB) *VpnRepo {
	return &VpnRepo{DB: db}
}
