package repository

import (
	"strconv"
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
	var user models.User
	err := c.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Create(&m).Error
		if err != nil {
			return err
		}
		user.Username = m.Firstname + m.Lastname + "(" + strconv.FormatInt(m.TgID, 10) + ")"
		user.Status = true
		user.Telegram = m
		return tx.Create(&user).Error
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
	err := c.DB.Where("tg_id = ?", tgID).First(&tg).Error
	if err != nil {
		return models.Vpn{}, err
	}

	var vpn = models.Vpn{
		UUID:   uuid,
		UserID: uint(tg.UserID),
		Link:   link,
	}
	err = c.DB.Create(&vpn).Error
	return vpn, err
}

// GetVpn
func (c *TelegramRepo) GetVpn(tgID int64) (models.Vpn, error) {
	var tg models.Telegram
	err := c.DB.Where("tg_id = ?", tgID).First(&tg).Error
	if err != nil {
		return models.Vpn{}, err
	}

	var vpn models.Vpn
	err = c.DB.Where("user_id = ?", tg.UserID).First(&vpn).Error
	return vpn, err
}

// GetAllUsers
func (c *TelegramRepo) GetAllUsers() ([]models.Telegram, error) {
	var users []models.Telegram
	err := c.DB.Find(&users).Error
	return users, err
}

// Create complaint
func (c *TelegramRepo) CreateComplaint(tgID int64, username string, text string) (models.Complaint, error) {
	var complaint = models.Complaint{
		TgID:     tgID,
		Username: username,
		Text:     text,
		Status:   "new",
	}
	err := c.DB.Create(&complaint).Error
	return complaint, err
}

// Update complaint
func (c *TelegramRepo) UpdateComplaint(id uint, reply string, status string) error {
	return c.DB.Model(&models.Complaint{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"admin_reply": reply,
			"status":      status,
		}).Error
}
