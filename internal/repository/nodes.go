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
	RawJSON          []byte
}

type NodeRecord struct {
	Registry models.ServerRegistry
	State    *models.NodeState
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
		var state models.NodeState
		err := r.DB.Where("server_id = ?", registry.ServerID).Take(&state).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			records = append(records, NodeRecord{Registry: registry})
			continue
		}
		if err != nil {
			return nil, err
		}
		records = append(records, NodeRecord{Registry: registry, State: &state})
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
	var state models.NodeState
	if err := r.DB.Where("server_id = ?", serverID).Take(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NodeRecord{Registry: registry}, nil
		}
		return NodeRecord{}, err
	}
	return NodeRecord{Registry: registry, State: &state}, nil
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
	var state models.NodeState
	var registry models.ServerRegistry
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("server_id = ?", update.ServerID).Take(&registry).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			firstSeenAt := update.ReceivedAt
			lastSeenAt := update.ReceivedAt
			registry = models.ServerRegistry{
				ServerID:         update.ServerID,
				DisplayName:      update.DisplayName,
				EndpointGroup:    update.EndpointGroup,
				ExpectedProtocol: update.ExpectedProtocol,
				Source:           models.NodeSourceDiscovered,
				Enabled:          true,
				FirstSeenAt:      &firstSeenAt,
				LastSeenAt:       &lastSeenAt,
			}
			if err := tx.Create(&registry).Error; err != nil {
				return err
			}
		} else {
			updates := map[string]any{"last_seen_at": &update.ReceivedAt}
			if registry.DisplayName == "" || registry.Source != "registered" {
				updates["display_name"] = update.DisplayName
			}
			if registry.EndpointGroup == "" || registry.Source != "registered" {
				updates["endpoint_group"] = update.EndpointGroup
			}
			if registry.ExpectedProtocol == "" || registry.ExpectedProtocol == models.ServerStatusUnknown || registry.Source != "registered" {
				updates["expected_protocol"] = update.ExpectedProtocol
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

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("server_id = ?", update.ServerID).Take(&state).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			xuiAvailable := update.XUIAvailable
			state = models.NodeState{
				ServerID:         update.ServerID,
				NodeID:           update.ServerID,
				DisplayName:      update.DisplayName,
				EndpointGroup:    update.EndpointGroup,
				ExpectedProtocol: update.ExpectedProtocol,
				ReportedProtocol: update.ReportedProtocol,
				Protocol:         update.ReportedProtocol,
				AgentVersion:     update.AgentVersion,
				AgentAlive:       update.AgentAlive,
				Status:           models.ServerStatusOnline,
				Source:           registry.Source,
				LastSeenAt:       update.ReceivedAt,
				LastSnapshotAt:   &update.SentAt,
				XUIAvailable:     &xuiAvailable,
				InboundID:        update.InboundID,
				InboundRemark:    update.InboundRemark,
				ClientsCount:     update.ClientsCount,
				OnlineCount:      update.OnlineCount,
				TrafficUp:        update.TrafficUp,
				TrafficDown:      update.TrafficDown,
				LastError:        update.LastError,
				Enabled:          registry.Enabled && registry.ArchivedAt == nil,
				SentAt:           &update.SentAt,
			}
			if err := tx.Create(&state).Error; err != nil {
				return err
			}
		} else {
			if state.LastSnapshotAt != nil && update.SentAt.Before(state.LastSnapshotAt.UTC()) {
				return ErrStaleNodeSnapshot
			}
			xuiAvailable := update.XUIAvailable
			updates := map[string]any{
				"server_id":         update.ServerID,
				"node_id":           update.ServerID,
				"display_name":      update.DisplayName,
				"endpoint_group":    update.EndpointGroup,
				"expected_protocol": update.ExpectedProtocol,
				"reported_protocol": update.ReportedProtocol,
				"protocol":          update.ReportedProtocol,
				"agent_version":     update.AgentVersion,
				"agent_alive":       update.AgentAlive,
				"status":            models.ServerStatusOnline,
				"source":            registry.Source,
				"last_seen_at":      update.ReceivedAt,
				"last_snapshot_at":  update.SentAt,
				"x_ui_available":    &xuiAvailable,
				"inbound_id":        update.InboundID,
				"inbound_remark":    update.InboundRemark,
				"clients_count":     update.ClientsCount,
				"online_count":      update.OnlineCount,
				"traffic_up":        update.TrafficUp,
				"traffic_down":      update.TrafficDown,
				"last_error":        update.LastError,
				"enabled":           registry.Enabled && registry.ArchivedAt == nil,
				"sent_at":           update.SentAt,
			}
			if err := tx.Model(&models.NodeState{}).Where("id = ?", state.ID).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.Where("server_id = ?", update.ServerID).Take(&state).Error; err != nil {
				return err
			}
		}

		xuiAvailable := update.XUIAvailable
		snapshot := models.NodeStateSnapshot{
			ServerID:         update.ServerID,
			DisplayName:      update.DisplayName,
			EndpointGroup:    update.EndpointGroup,
			ExpectedProtocol: update.ExpectedProtocol,
			ReportedProtocol: update.ReportedProtocol,
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
			SentAt:           update.SentAt,
			ReceivedAt:       update.ReceivedAt,
			RawJSON:          update.RawJSON,
		}
		return tx.Create(&snapshot).Error
	})
	if errors.Is(err, ErrStaleNodeSnapshot) {
		return nodeStateFromRecord(NodeRecord{Registry: registry, State: &state}), true, nil
	}
	if err != nil {
		return models.NodeState{}, false, err
	}
	return nodeStateFromRecord(NodeRecord{Registry: registry, State: &state}), false, nil
}

func (r *NodeRepo) ArchiveServer(serverID, reason string, archivedAt time.Time) error {
	return r.DB.Model(&models.ServerRegistry{}).Where("server_id = ?", serverID).Updates(map[string]any{
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
		ServerID:         registry.ServerID,
		NodeID:           registry.ServerID,
		DisplayName:      registry.DisplayName,
		EndpointGroup:    registry.EndpointGroup,
		ExpectedProtocol: registry.ExpectedProtocol,
		ReportedProtocol: models.ServerStatusUnknown,
		Protocol:         models.ServerStatusUnknown,
		Source:           registry.Source,
		Enabled:          registry.Enabled && registry.ArchivedAt == nil,
	}
	if registry.LastSeenAt != nil {
		node.LastSeenAt = *registry.LastSeenAt
	}
	if record.State != nil {
		state := *record.State
		state.ServerID = registry.ServerID
		state.NodeID = registry.ServerID
		return state
	}
	return node
}
