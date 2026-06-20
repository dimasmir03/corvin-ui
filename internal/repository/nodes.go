package repository

import (
	"time"
	"vpnpanel/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NodeRepo struct {
	DB *gorm.DB
}

type NodeHeartbeatUpdate struct {
	NodeID        string
	EndpointGroup string
	Protocol      string
	AgentVersion  string
	Status        string
	LastSeenAt    time.Time
	LastError     string
	SentAt        *time.Time
}

func NewNodeRepo(db *gorm.DB) *NodeRepo {
	return &NodeRepo{DB: db}
}

func (r *NodeRepo) UpsertHeartbeat(update NodeHeartbeatUpdate) (models.NodeState, error) {
	node := models.NodeState{
		NodeID:        update.NodeID,
		EndpointGroup: update.EndpointGroup,
		Protocol:      update.Protocol,
		AgentVersion:  update.AgentVersion,
		Status:        update.Status,
		LastSeenAt:    update.LastSeenAt,
		LastError:     update.LastError,
		SentAt:        update.SentAt,
	}

	err := r.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"endpoint_group",
			"protocol",
			"agent_version",
			"status",
			"last_seen_at",
			"last_error",
			"sent_at",
			"updated_at",
		}),
	}).Create(&node).Error
	if err != nil {
		return models.NodeState{}, err
	}

	if err := r.DB.Where("node_id = ?", update.NodeID).Take(&node).Error; err != nil {
		return models.NodeState{}, err
	}
	return node, nil
}

func (r *NodeRepo) List() ([]models.NodeState, error) {
	var nodes []models.NodeState
	if err := r.DB.Order("node_id ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *NodeRepo) GetByNodeID(nodeID string) (models.NodeState, error) {
	var node models.NodeState
	if err := r.DB.Where("node_id = ?", nodeID).Take(&node).Error; err != nil {
		return models.NodeState{}, err
	}
	return node, nil
}
