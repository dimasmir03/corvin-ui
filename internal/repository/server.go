// repository/server_repo.go
package repository

import (
	"time"
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

// SaveTotalOnline
func (r *ServerRepo) SaveTotalOnline(totalOnline int) error {
	stat := models.ServerStat{
		ServerID:  0, // 0 = общий онлайн
		Online:    totalOnline,
		CreatedAt: time.Now(),
	}
	return r.DB.Create(&stat).Error
}

// UpdateOnline(serverId, serverCount)
func (r *ServerRepo) UpdateOnline(serverId int, serverCount int) error {
	return r.DB.Model(&models.Server{}).
		Where("server_id = ?", serverId).
		Update("online", serverCount).Error
}

// CreateStat
func (r *ServerRepo) CreateStat(s *models.ServerStat) error {
	tx := r.DB.Create(s)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

// GetAllWithLastStat
func (r *ServerRepo) GetAllWithLastStat() ([]models.Server, int, error) {
	var servers []models.Server
	if err := r.DB.Find(&servers).Error; err != nil {
		return nil, 0, err
	}

	// Подзапрос: для каждого server_id получаем максимальный created_at
	sub := r.DB.
		Model(&models.ServerStat{}).
		Select("server_id, MAX(created_at) AS max_created").
		Group("server_id")

	// Основной запрос: берем полные строки server_stats, которые соответствуют найденным max_created
	var stats []models.ServerStat
	if err := r.DB.
		Table("server_stats AS ss").
		Select("ss.*").
		Joins("JOIN (?) AS latest ON latest.server_id = ss.server_id AND latest.max_created = ss.created_at", sub).
		Scan(&stats).Error; err != nil {
		return nil, 0, err
	}

	// Формируем мапу server_id → stat
	statMap := make(map[int]models.ServerStat, len(stats))
	totalOnline := 0
	for _, st := range stats {
		statMap[st.ServerID] = st
		if st.ServerID == 0 {
			totalOnline = st.Online
		}
	}

	// Присваиваем последнюю статистику каждому серверу
	for i := range servers {
		if st, ok := statMap[servers[i].Id]; ok {
			stCopy := st
			servers[i].LastStat = &stCopy
		}
	}

	return servers, totalOnline, nil
}

// GetOnlineHistory
func (r *ServerRepo) GetOnlineHistory() ([]models.ServerStat, error) {
	var stats []models.ServerStat
	tx := r.DB.Select("created_at", "online").Where("server_id = 0").Find(&stats)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return stats, nil
}

// clear stats
func (r *ServerRepo) ClearStats() error {
	tx := r.DB.Delete(&models.ServerStat{})
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}
