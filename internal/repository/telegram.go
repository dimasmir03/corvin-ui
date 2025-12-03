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

// Create VPN
func (c *TelegramRepo) CreateVpn(tgID int64, uuid, link string) (models.Vpn, error) {
	var tg models.Telegram
	if err := c.DB.Where("tg_id = ?", tgID).First(&tg).Error; err != nil {
		return models.Vpn{}, err
	}

	vpn := models.Vpn{
		UUID:   uuid,
		UserID: tg.UserID,
		Link:   link,
	}

	if err := c.DB.Create(&vpn).Error; err != nil {
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
