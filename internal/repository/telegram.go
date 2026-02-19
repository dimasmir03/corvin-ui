package repository

import (
	"errors"
	"fmt"
	"vpnpanel/internal/models"

	"gorm.io/gorm"
)

type TelegramRepo struct {
	DB *gorm.DB
}

func NewTelegramRepo(db *gorm.DB) *TelegramRepo {
	return &TelegramRepo{DB: db}
}

// Create user
func (c *TelegramRepo) CreateUser(m models.Telegram) (models.Telegram, error) {
	err := c.DB.Transaction(func(tx *gorm.DB) error {
		user := models.User{
			Username: fmt.Sprintf("%s%s(%d)", m.Firstname, m.Lastname, m.TgID),
			Status:   true,
		}

		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		m.UserID = user.ID

		if err := tx.Create(&m).Error; err != nil {
			return err
		}

		// return tx.Model(&m).Update("user_id", user.ID).Error
		return nil
	})

	return m, err
}

// Get user
func (c *TelegramRepo) GetUser(tgID string) (models.Telegram, error) {
	var tg models.Telegram
	err := c.DB.Where("tg_id = ?", tgID).First(&tg).Error
	return tg, err
}

type CreateVpnParams struct {
	UserID     uint
	TgID       int64
	UUID       string
	Status     string
	VlessLink  string
	TrojanLink string
}

// Create VPN
func (c *TelegramRepo) CreateVpn(params CreateVpnParams) (models.Vpn, error) {
	var tg models.Telegram
	if err := c.DB.Where("tg_id = ?", params.TgID).First(&tg).Error; err != nil {
		return models.Vpn{}, err
	}

	vpn := models.Vpn{
		UUID:       params.UUID,
		UserID:     tg.UserID,
		Status:     params.Status,
		Link:       params.VlessLink,
		VlessLink:  params.VlessLink,
		TrojanLink: params.TrojanLink,
	}

	if err := c.DB.Create(&vpn).Error; err != nil {
		return models.Vpn{}, err
	}

	return vpn, nil
}

func (c *TelegramRepo) CreateVpnProtocol(params CreateVpnParams, protocol string) (models.Vpn, error) {
	var tg models.Telegram
	if err := c.DB.Where("tg_id = ?", params.TgID).First(&tg).Error; err != nil {
		return models.Vpn{}, err
	}
	var vpn models.Vpn

	err := c.DB.Where("user_id = ?", tg.UserID).First(&vpn).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// создаём
		vpn = models.Vpn{
			UUID:   params.UUID,
			UserID: tg.UserID,
		}

		if protocol == "vless" {
			vpn.VlessLink = params.VlessLink
		} else if protocol == "trojan" {
			vpn.TrojanLink = params.TrojanLink
		}

		if err := c.DB.Create(&vpn).Error; err != nil {
			return models.Vpn{}, err
		}

	} else if err == nil {
		// обновляем
		if protocol == "vless" {
			vpn.VlessLink = params.VlessLink
		} else if protocol == "trojan" {
			vpn.TrojanLink = params.TrojanLink
		}

		if err := c.DB.Save(&vpn).Error; err != nil {
			return models.Vpn{}, err
		}
	} else {
		return models.Vpn{}, err
	}

	return vpn, nil
}

// GetVpn
func (c *TelegramRepo) GetVpn(tgID int64) (models.Vpn, error) {
	var tg models.Telegram
	if err := c.DB.Where("tg_id = ?", tgID).First(&tg).Error; err != nil {
		return models.Vpn{}, err
	}

	var vpn models.Vpn
	err := c.DB.Where("user_id = ?", tg.UserID).First(&vpn).Error
	return vpn, err
}

func (c *TelegramRepo) UpdateVlessLink(vpnID uint, link string) error {
	return c.DB.Model(&models.Vpn{}).
		Where("id = ?", vpnID).
		Update("vless_link", link).Error
}

func (c *TelegramRepo) UpdateTrojanLink(vpnID uint, link string) error {
	return c.DB.Model(&models.Vpn{}).
		Where("id = ?", vpnID).
		Update("trojan_link", link).Error
}

func (c *TelegramRepo) GetTelegramByTgID(tgID int64) (models.Telegram, error) {
	var tg models.Telegram
	return tg, c.DB.Where("tg_id = ?", tgID).First(&tg).Error
}

// GetAllUsers
func (c *TelegramRepo) GetAllUsers() ([]models.Telegram, error) {
	var users []models.Telegram
	err := c.DB.Find(&users).Error
	return users, err
}

// Create complaint
func (c *TelegramRepo) CreateComplaint(tgID int64, username, text string) (models.Complaint, error) {
	// 1. Ищем user_id через таблицу telegrams
	var telegram models.Telegram
	err := c.DB.Where("tg_id = ?", tgID).First(&telegram).Error
	if err != nil {
		return models.Complaint{}, fmt.Errorf("telegram user not found: %w", err)
	}

	// 2. Заполняем complaint
	complaint := models.Complaint{
		TgID:     tgID,
		Username: username,
		Text:     text,
		Status:   "new",
		Photo:    false,
		UserID:   telegram.UserID,
	}

	if err := c.DB.Create(&complaint).Error; err != nil {
		return models.Complaint{}, err
	}

	return complaint, nil
}

func (c *TelegramRepo) UpdateComplaintPhotoURL(id uint, photoURL string) error {
	return c.DB.Model(&models.Complaint{}).
		Where("id = ?", id).
		Update("photo", true).
		Update("photo_url", photoURL).
		Error
}

// Update complaint
func (c *TelegramRepo) UpdateComplaint(id uint, reply string, status string) (*models.Complaint, error) {
	tx := c.DB.Model(&models.Complaint{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"reply":  reply,
			"status": status,
		})

	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, errors.New("complaint not found")
	}

	var complaint models.Complaint
	if err := c.DB.First(&complaint, id).Error; err != nil {
		return nil, err
	}

	return &complaint, nil
}
