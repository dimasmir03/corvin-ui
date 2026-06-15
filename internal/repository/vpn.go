package repository

import (
	"vpnpanel/internal/models"

	"gorm.io/gorm"
)

type VpnRepo struct {
	DB *gorm.DB
}

func NewVpnRepo(db *gorm.DB) *VpnRepo {
	return &VpnRepo{DB: db}
}

func (r *VpnRepo) GetByUserID(userID uint) (models.Vpn, error) {
	var vpn models.Vpn
	err := r.DB.Where("user_id = ?", userID).First(&vpn).Error
	return vpn, err
}

func (r *VpnRepo) Create(vpn models.Vpn) (models.Vpn, error) {
	if err := r.DB.Create(&vpn).Error; err != nil {
		return models.Vpn{}, err
	}
	return vpn, nil
}

func (r *VpnRepo) Save(vpn models.Vpn) (models.Vpn, error) {
	if err := r.DB.Save(&vpn).Error; err != nil {
		return models.Vpn{}, err
	}
	return vpn, nil
}
