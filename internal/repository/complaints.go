package repository

import (
	"vpnpanel/internal/models"

	"gorm.io/gorm"
)

type ComplaintRepository struct {
	DB *gorm.DB
}

func NewComplaintRepo(db *gorm.DB) *ComplaintRepository {
	return &ComplaintRepository{DB: db}
}

func (c *ComplaintRepository) GetAll() ([]models.Complaint, error) {
	var complaints []models.Complaint
	return complaints, c.DB.Find(&complaints).Error
}

func (c *ComplaintRepository) GetByID(id uint) (models.Complaint, error) {
	var complaint models.Complaint
	err := c.DB.First(&complaint, id).Error
	return complaint, err
}

func (c *ComplaintRepository) Create(complaint *models.Complaint) error {
	return c.DB.Create(complaint).Error
}

func (c *ComplaintRepository) Update(complaint *models.Complaint) error {
	return c.DB.Save(complaint).Error
}

func (c *ComplaintRepository) UpdateReply(id uint, reply string) error {
	return c.DB.Model(&models.Complaint{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"reply":  reply,
			"status": "resolved",
		}).Error
}

func (c *ComplaintRepository) Delete(id uint) error {
	return c.DB.Delete(&models.Complaint{}, id).Error
}
