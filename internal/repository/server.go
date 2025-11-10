// repository/server_repo.go
package repository

import (
	"vpnpanel/internal/models"

	"gorm.io/gorm"
)

type ServerRepo struct {
	DB *gorm.DB
}

func NewServerRepo(db *gorm.DB) *ServerRepo {
	return &ServerRepo{DB: db}
}

func (r *ServerRepo) GetAll() ([]models.Server, error) {
	var servers []models.Server
	tx := r.DB.Find(&servers)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return servers, nil
}

func (r *ServerRepo) GetByID(id int) (*models.Server, error) {
	var s models.Server
	tx := r.DB.Where("id = ?", id).Take(&s)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &s, nil
}

func (r *ServerRepo) Create(s *models.Server) error {
	tx := r.DB.Create(s)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (r *ServerRepo) Update(s *models.Server) error {
	tx := r.DB.Save(s)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (r *ServerRepo) Delete(id int) error {
	var s models.Server
	tx := r.DB.Where("id = ?", id).Delete(&s)
	return tx.Error
}

func (r *ServerRepo) OnlineUsersServers() ([]models.Server, error) {
	var servers []models.Server
	tx := r.DB.Where("online = ?", true).Find(&servers)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return servers, nil
}
