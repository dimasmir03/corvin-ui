package repository

import (
	"errors"
	"time"
	"vpnpanel/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NodeRepo struct {
	DB *gorm.DB
}

var ErrStaleNodeSnapshot = errors.New("stale node snapshot")

type NodeSnapshotUpdate struct {
	ServerID         string
	DisplayName      string
	EndpointGroup    string
	ExpectedProtocol string
	ReportedProtocol string
	ServerRole       string
	AgentVersion     string
	AgentAlive       bool
	XUIAvailable     bool
	InboundID        *int
	InboundRemark    string
	ClientsCount     int
	OnlineCount      int
	TrafficUp        int64
	TrafficDown      int64
	LastError        string
	ReceivedAt       time.Time
	SentAt           time.Time
}

type NodeRecord struct {
	Registry models.ServerRegistry
	Stats    *models.NodeStats
}

func NewNodeRepo(db *gorm.DB) *NodeRepo {
	return &NodeRepo{DB: db}
}

func (r *NodeRepo) ListRecords(includeArchived bool) ([]NodeRecord, error) {
	var registries []models.ServerRegistry
	query := r.DB.Order("endpoint_group ASC, server_id ASC")
	if !includeArchived {
		query = query.Where("archived_at IS NULL AND enabled = ?", true)
	}
	if err := query.Find(&registries).Error; err != nil {
		return nil, err
	}

	records := make([]NodeRecord, 0, len(registries))
	for _, registry := range registries {
		var stats models.NodeStats
		err := r.DB.Where("server_id = ?", registry.ServerID).Take(&stats).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			records = append(records, NodeRecord{Registry: registry})
			continue
		}
		if err != nil {
			return nil, err
		}
		records = append(records, NodeRecord{Registry: registry, Stats: &stats})
	}
	return records, nil
}

func (r *NodeRepo) List() ([]models.NodeState, error) {
	records, err := r.ListRecords(false)
	if err != nil {
		return nil, err
	}
	nodes := make([]models.NodeState, 0, len(records))
	for _, record := range records {
		nodes = append(nodes, nodeStateFromRecord(record))
	}
	return nodes, nil
}

func (r *NodeRepo) GetRecordByServerID(serverID string) (NodeRecord, error) {
	var registry models.ServerRegistry
	if err := r.DB.Where("server_id = ?", serverID).Take(&registry).Error; err != nil {
		return NodeRecord{}, err
	}
	var stats models.NodeStats
	if err := r.DB.Where("server_id = ?", serverID).Take(&stats).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NodeRecord{Registry: registry}, nil
		}
		return NodeRecord{}, err
	}
	return NodeRecord{Registry: registry, Stats: &stats}, nil
}

func (r *NodeRepo) GetByServerID(serverID string) (models.NodeState, error) {
	record, err := r.GetRecordByServerID(serverID)
	if err != nil {
		return models.NodeState{}, err
	}
	return nodeStateFromRecord(record), nil
}

func (r *NodeRepo) GetByNodeID(nodeID string) (models.NodeState, error) {
	return r.GetByServerID(nodeID)
}

func (r *NodeRepo) ApplySnapshot(update NodeSnapshotUpdate) (models.NodeState, bool, error) {
	var stats models.NodeStats
	var registry models.ServerRegistry
	created := false
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("server_id = ?", update.ServerID).Take(&registry).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			created = true
			registry = models.ServerRegistry{
				ServerID:         update.ServerID,
				DisplayName:      update.DisplayName,
				EndpointGroup:    update.EndpointGroup,
				ExpectedProtocol: update.ExpectedProtocol,
				ServerRole:       update.ServerRole,
				Source:           models.NodeSourceDiscovered,
				Enabled:          true,
				FirstSeenAt:      update.ReceivedAt,
				LastSeenAt:       update.ReceivedAt,
			}
			if err := tx.Create(&registry).Error; err != nil {
				return err
			}
		} else {
			updates := map[string]any{"last_seen_at": update.ReceivedAt}
			if registry.DisplayName == "" || registry.Source != "registered" {
				updates["display_name"] = update.DisplayName
			}
			if registry.EndpointGroup == "" || registry.Source != "registered" {
				updates["endpoint_group"] = update.EndpointGroup
			}
			if registry.ExpectedProtocol == "" || registry.ExpectedProtocol == models.ServerStatusUnknown || registry.Source != "registered" {
				updates["expected_protocol"] = update.ExpectedProtocol
			}
			if registry.ServerRole == "" || registry.ServerRole == "other" || registry.Source != "registered" {
				updates["server_role"] = update.ServerRole
			}
			if registry.Source == "" {
				updates["source"] = models.NodeSourceDiscovered
			}
			if registry.ArchivedAt != nil {
				updates["archived_at"] = nil
				updates["archived_reason"] = ""
			}
			if err := tx.Model(&models.ServerRegistry{}).Where("id = ?", registry.ID).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.Where("server_id = ?", update.ServerID).Take(&registry).Error; err != nil {
				return err
			}
		}

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("server_id = ?", update.ServerID).Take(&stats).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			xuiAvailable := update.XUIAvailable
			stats = models.NodeStats{
				ServerID:         update.ServerID,
				EndpointGroup:    update.EndpointGroup,
				ExpectedProtocol: update.ExpectedProtocol,
				ReportedProtocol: update.ReportedProtocol,
				ServerRole:       update.ServerRole,
				DisplayName:      update.DisplayName,
				AgentVersion:     update.AgentVersion,
				AgentAlive:       update.AgentAlive,
				XUIAvailable:     &xuiAvailable,
				InboundID:        update.InboundID,
				InboundRemark:    update.InboundRemark,
				ClientsCount:     update.ClientsCount,
				OnlineCount:      update.OnlineCount,
				TrafficUp:        update.TrafficUp,
				TrafficDown:      update.TrafficDown,
				LastError:        update.LastError,
				LastSnapshotAt:   &update.SentAt,
			}
			if err := tx.Create(&stats).Error; err != nil {
				return err
			}
		} else {
			if stats.LastSnapshotAt != nil && update.SentAt.Before(stats.LastSnapshotAt.UTC()) {
				return ErrStaleNodeSnapshot
			}
			xuiAvailable := update.XUIAvailable
			updates := map[string]any{
				"endpoint_group":     update.EndpointGroup,
				"expected_protocol":  update.ExpectedProtocol,
				"reported_protocol":  update.ReportedProtocol,
				"server_role":        update.ServerRole,
				"display_name":       update.DisplayName,
				"agent_version":      update.AgentVersion,
				"agent_alive":        update.AgentAlive,
				"x_ui_available":     &xuiAvailable,
				"inbound_id":         update.InboundID,
				"inbound_remark":     update.InboundRemark,
				"clients_count":      update.ClientsCount,
				"online_count":       update.OnlineCount,
				"traffic_up":         update.TrafficUp,
				"traffic_down":       update.TrafficDown,
				"last_error":         update.LastError,
				"last_snapshot_at":   update.SentAt,
			}
			if err := tx.Model(&models.NodeStats{}).Where("id = ?", stats.ID).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.Where("server_id = ?", update.ServerID).Take(&stats).Error; err != nil {
				return err
			}
		}

		xuiAvailable := update.XUIAvailable
		snapshot := models.NodeStatsHistory{
			ServerID:         update.ServerID,
			EndpointGroup:    update.EndpointGroup,
			ExpectedProtocol: update.ExpectedProtocol,
			ReportedProtocol: update.ReportedProtocol,
			ServerRole:       update.ServerRole,
			AgentVersion:     update.AgentVersion,
			AgentAlive:       update.AgentAlive,
			XUIAvailable:     &xuiAvailable,
			ClientsCount:     update.ClientsCount,
			OnlineCount:      update.OnlineCount,
			TrafficUp:        update.TrafficUp,
			TrafficDown:      update.TrafficDown,
			LastError:        update.LastError,
			SentAt:           update.SentAt,
			ReceivedAt:       update.ReceivedAt,
		}
		return tx.Create(&snapshot).Error
	})
	if errors.Is(err, ErrStaleNodeSnapshot) {
		return nodeStateFromRecord(NodeRecord{Registry: registry, Stats: &stats}), true, nil
	}
	if err != nil {
		return models.NodeState{}, false, err
	}
	return nodeStateFromRecord(NodeRecord{Registry: registry, Stats: &stats}), false, nil
}

func (r *NodeRepo) ArchiveServer(serverID, reason string, archivedAt time.Time) error {
	return r.DB.Model(&models.ServerRegistry{}).Where("server_id = ? AND source <> ?", serverID, "registered-hard-lock").Updates(map[string]any{
		"archived_at":     &archivedAt,
		"archived_reason": reason,
	}).Error
}

func (r *NodeRepo) RestoreServer(serverID string) error {
	return r.DB.Model(&models.ServerRegistry{}).Where("server_id = ?", serverID).Updates(map[string]any{
		"archived_at":     nil,
		"archived_reason": "",
		"enabled":         true,
	}).Error
}

func (r *NodeRepo) ArchiveStaleDiscovered(cutoff time.Time, reason string, archivedAt time.Time) (int64, error) {
	result := r.DB.Model(&models.ServerRegistry{}).Where("source = ? AND archived_at IS NULL AND last_seen_at < ?", models.NodeSourceDiscovered, cutoff).Updates(map[string]any{
		"archived_at":     &archivedAt,
		"archived_reason": reason,
	})
	return result.RowsAffected, result.Error
}

func nodeStateFromRecord(record NodeRecord) models.NodeState {
	registry := record.Registry
	node := models.NodeState{
		ServerID:      registry.ServerID,
		NodeID:        registry.ServerID,
		EndpointGroup: registry.EndpointGroup,
		Protocol:      registry.ExpectedProtocol,
		Source:        registry.Source,
		Enabled:       registry.Enabled && registry.ArchivedAt == nil,
		LastSeenAt:    registry.LastSeenAt,
	}
	if record.Stats != nil {
		stats := *record.Stats
		node.EndpointGroup = stats.EndpointGroup
		node.Protocol = stats.ReportedProtocol
		node.AgentVersion = stats.AgentVersion
		node.LastSnapshotAt = stats.LastSnapshotAt
		node.XUIAvailable = stats.XUIAvailable
		node.InboundID = stats.InboundID
		node.InboundRemark = stats.InboundRemark
		node.ClientsCount = stats.ClientsCount
		node.OnlineCount = stats.OnlineCount
		node.TrafficUp = stats.TrafficUp
		node.TrafficDown = stats.TrafficDown
		node.LastError = stats.LastError
	}
	return node
}
