// repository/server_repo.go
package repository

import (
	"fmt"
	"time"
	"vpnpanel/internal/models"

	"gorm.io/gorm"
)

type ServerRepo struct {
	DB *gorm.DB
}

type ServerProbeVersions struct {
	PanelVersion *string
	XrayVersion  *string
	AgentVersion *string
}

type NodeStatsUpdate struct {
	OnlineCount   int
	UploadBytes   int64
	DownloadBytes int64
	TotalBytes    int64
	PanelStatus   *string
	XrayStatus    *string
	PanelVersion  *string
	XrayVersion   *string
	AgentVersion  *string
	Error         *string
	RawJSON       []byte
	ObservedAt    time.Time
	Status        string
}

func NewServerRepo(db *gorm.DB) *ServerRepo {
	return &ServerRepo{DB: db}
}

func (r *ServerRepo) GetAll() ([]models.Server, error) {
	return r.GetAllFiltered("")
}

func (r *ServerRepo) GetAllFiltered(role string) ([]models.Server, error) {
	var servers []models.Server
	query := r.DB
	switch role {
	case "", "all":
	case models.NodeRoleRU, models.NodeRoleForeign, models.NodeRoleDirect, models.NodeRoleOther:
		query = query.Where("node_role = ?", role)
	default:
		return nil, fmt.Errorf("invalid role %q", role)
	}
	if err := query.Find(&servers).Error; err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *ServerRepo) GetByID(id int) (*models.Server, error) {
	var s models.Server
	if err := r.DB.Where("id = ?", id).Take(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ServerRepo) Create(s *models.Server) error {
	return r.DB.Create(s).Error
}

func (r *ServerRepo) Update(s *models.Server) error {
	return r.DB.Save(s).Error
}

func (r *ServerRepo) Delete(id int) error {
	return r.DB.Delete(&models.Server{}, id).Error
}

// SaveTotalOnline
func (r *ServerRepo) SaveTotalOnline(totalOnline int) error {
	stat := models.ServerStat{
		ServerID:  0,
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
	return r.DB.Create(s).Error
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
	// start := time.Now()
	var stats []models.ServerStat
	err := r.DB.Select("created_at", "online").Where("server_id = 0").Find(&stats).Error
	// fmt.Println("debug print time for sqlGetOnlineHistory :  ", time.Since(start))
	return stats, err
}

// clear stats
func (r *ServerRepo) ClearStats() error {
	return r.DB.Delete(&models.ServerStat{}).Error
}

func (r *ServerRepo) UpdateProbeSuccess(serverID int, probedAt time.Time, versions ServerProbeVersions) error {
	updates := map[string]any{
		"status":        models.ServerStatusOnline,
		"last_seen_at":  &probedAt,
		"last_probe_at": &probedAt,
		"last_error":    nil,
	}
	if versions.PanelVersion != nil {
		updates["panel_version"] = versions.PanelVersion
	}
	if versions.XrayVersion != nil {
		updates["xray_version"] = versions.XrayVersion
	}
	if versions.AgentVersion != nil {
		updates["agent_version"] = versions.AgentVersion
	}
	return r.DB.Model(&models.Server{}).Where("id = ?", serverID).Updates(updates).Error
}

func (r *ServerRepo) UpdateProbeFailed(serverID int, probedAt time.Time, status, lastError string) error {
	if status == "" {
		status = models.ServerStatusOffline
	}
	return r.DB.Model(&models.Server{}).Where("id = ?", serverID).Updates(map[string]any{
		"status":        status,
		"last_probe_at": &probedAt,
		"last_error":    &lastError,
	}).Error
}

func (r *ServerRepo) ApplyNodeStats(serverID int, stats NodeStatsUpdate) error {
	if stats.ObservedAt.IsZero() {
		stats.ObservedAt = time.Now()
	}
	return r.DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status":              stats.Status,
			"last_stats_at":       &stats.ObservedAt,
			"last_online_count":   stats.OnlineCount,
			"last_upload_bytes":   stats.UploadBytes,
			"last_download_bytes": stats.DownloadBytes,
			"last_total_bytes":    stats.TotalBytes,
			"last_panel_status":   stats.PanelStatus,
			"last_xray_status":    stats.XrayStatus,
			"last_error":          stats.Error,
		}
		if stats.Status == models.ServerStatusOnline || stats.Status == models.ServerStatusDegraded {
			updates["last_seen_at"] = &stats.ObservedAt
		}
		if stats.PanelVersion != nil {
			updates["panel_version"] = stats.PanelVersion
		}
		if stats.XrayVersion != nil {
			updates["xray_version"] = stats.XrayVersion
		}
		if stats.AgentVersion != nil {
			updates["agent_version"] = stats.AgentVersion
		}
		if err := tx.Model(&models.Server{}).Where("id = ?", serverID).Updates(updates).Error; err != nil {
			return err
		}

		snapshot := models.NodeStatsSnapshot{
			ServerID:      uint(serverID),
			OnlineCount:   stats.OnlineCount,
			UploadBytes:   stats.UploadBytes,
			DownloadBytes: stats.DownloadBytes,
			TotalBytes:    stats.TotalBytes,
			PanelStatus:   stats.PanelStatus,
			XrayStatus:    stats.XrayStatus,
			PanelVersion:  stats.PanelVersion,
			XrayVersion:   stats.XrayVersion,
			AgentVersion:  stats.AgentVersion,
			Error:         stats.Error,
			RawJSON:       stats.RawJSON,
			CreatedAt:     stats.ObservedAt,
		}
		return tx.Create(&snapshot).Error
	})
}

func (r *ServerRepo) LatestNodeStats(serverID int) (*models.NodeStatsSnapshot, error) {
	var snapshot models.NodeStatsSnapshot
	if err := r.DB.Where("server_id = ?", serverID).Order("created_at DESC").Take(&snapshot).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *ServerRepo) NodeStatsHistory(serverID int, limit int) ([]models.NodeStatsSnapshot, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var snapshots []models.NodeStatsSnapshot
	if err := r.DB.Where("server_id = ?", serverID).Order("created_at DESC").Limit(limit).Find(&snapshots).Error; err != nil {
		return nil, err
	}
	return snapshots, nil
}

func (r *ServerRepo) OnlineHistory(serverID int, since time.Time, limit int) ([]models.ServerStat, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	var stats []models.ServerStat
	query := r.DB.Where("server_id = ?", serverID)
	if !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}
	if err := query.Order("created_at ASC").Limit(limit).Find(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}
