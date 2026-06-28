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
	ServerID       string
	EndpointGroup  string
	Protocol       string
	AgentVersion   string
	XUIAvailable   bool
	InboundID      *int
	InboundRemark  string
	ClientsCount   int
	OnlineCount    int
	TrafficUp      int64
	TrafficDown    int64
	LastError      string
	LastSeenAt     time.Time
	LastSnapshotAt time.Time
}

func NewNodeRepo(db *gorm.DB) *NodeRepo {
	return &NodeRepo{DB: db}
}

func (r *NodeRepo) List() ([]models.NodeState, error) {
	var nodes []models.NodeState
	if err := r.DB.Order("server_id ASC, node_id ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *NodeRepo) GetByServerID(serverID string) (models.NodeState, error) {
	var node models.NodeState
	if err := r.DB.Where("server_id = ? OR node_id = ?", serverID, serverID).Take(&node).Error; err != nil {
		return models.NodeState{}, err
	}
	return normalizeNodeStateIdentity(node), nil
}

func (r *NodeRepo) GetByNodeID(nodeID string) (models.NodeState, error) {
	return r.GetByServerID(nodeID)
}

func normalizeNodeStateIdentity(node models.NodeState) models.NodeState {
	if node.ServerID == "" {
		node.ServerID = node.NodeID
	}
	if node.NodeID == "" {
		node.NodeID = node.ServerID
	}
	return node
}

func (r *NodeRepo) ApplySnapshot(update NodeSnapshotUpdate) (models.NodeState, bool, error) {
	var node models.NodeState
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("server_id = ? OR node_id = ?", update.ServerID, update.ServerID).Take(&node).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			xuiAvailable := update.XUIAvailable
			node = models.NodeState{
				ServerID:       update.ServerID,
				NodeID:         update.ServerID,
				EndpointGroup:  update.EndpointGroup,
				Protocol:       update.Protocol,
				AgentVersion:   update.AgentVersion,
				Status:         models.ServerStatusOnline,
				Source:         models.NodeSourceDiscovered,
				LastSeenAt:     update.LastSeenAt,
				LastSnapshotAt: &update.LastSnapshotAt,
				XUIAvailable:   &xuiAvailable,
				InboundID:      update.InboundID,
				InboundRemark:  update.InboundRemark,
				ClientsCount:   update.ClientsCount,
				OnlineCount:    update.OnlineCount,
				TrafficUp:      update.TrafficUp,
				TrafficDown:    update.TrafficDown,
				LastError:      update.LastError,
			}
			return tx.Create(&node).Error
		}

		if node.LastSnapshotAt != nil && update.LastSnapshotAt.Before(node.LastSnapshotAt.UTC()) {
			return ErrStaleNodeSnapshot
		}

		xuiAvailable := update.XUIAvailable
		source := node.Source
		if source == "" {
			source = models.NodeSourceDiscovered
		}
		updates := map[string]any{
			"server_id":        update.ServerID,
			"node_id":          update.ServerID,
			"endpoint_group":   update.EndpointGroup,
			"protocol":         update.Protocol,
			"agent_version":    update.AgentVersion,
			"status":           models.ServerStatusOnline,
			"source":           source,
			"last_seen_at":     update.LastSeenAt,
			"last_snapshot_at": update.LastSnapshotAt,
			"x_ui_available":   &xuiAvailable,
			"inbound_id":       update.InboundID,
			"inbound_remark":   update.InboundRemark,
			"clients_count":    update.ClientsCount,
			"online_count":     update.OnlineCount,
			"traffic_up":       update.TrafficUp,
			"traffic_down":     update.TrafficDown,
			"last_error":       update.LastError,
		}
		if err := tx.Model(&models.NodeState{}).Where("id = ?", node.ID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("server_id = ? OR node_id = ?", update.ServerID, update.ServerID).Take(&node).Error
	})
	if errors.Is(err, ErrStaleNodeSnapshot) {
		return node, true, nil
	}
	if err != nil {
		return models.NodeState{}, false, err
	}
	return normalizeNodeStateIdentity(node), false, nil
}
