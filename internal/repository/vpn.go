package repository

import (
	"errors"
	"fmt"
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

func (r *VpnRepo) UpsertLinkByUserID(userID uint, protocol string, link string) (models.Vpn, error) {
	vpn, err := r.GetByUserID(userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Vpn{}, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		vpn = models.Vpn{
			UUID:   fmt.Sprintf("agent-user-%d", userID),
			UserID: userID,
			Status: "active",
			Link:   link,
		}
	}

	switch protocol {
	case "vless":
		vpn.VlessLink = link
	case "trojan":
		vpn.TrojanLink = link
	}
	if vpn.Link == "" {
		vpn.Link = link
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.Create(vpn)
	}
	return r.Save(vpn)
}
