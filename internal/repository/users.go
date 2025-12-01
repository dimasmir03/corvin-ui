package repository

import (
	"vpnpanel/internal/models"

	"gorm.io/gorm"
)

type UserRepo struct {
	DB *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{DB: db}
}

// GetAllUSers
func (c *UserRepo) GetAllUsers() ([]models.User, error) {
	var users []models.User
	err := c.DB.Find(&users).Error
	return users, err
}
