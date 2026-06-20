package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
)

const (
	NodeHeartbeatEventType = "node_heartbeat"

	nodeOnlineWindow = 2 * time.Minute
	nodeStaleWindow  = 5 * time.Minute
)

type NodeService struct {
	nodeRepo *repository.NodeRepo
	now      func() time.Time
}

func NewNodeService(nodeRepo *repository.NodeRepo) *NodeService {
	return &NodeService{nodeRepo: nodeRepo, now: time.Now}
}

func (s *NodeService) ApplyHeartbeat(ctx context.Context, event broker.NodeHeartbeatEvent) (models.NodeState, error) {
	_ = ctx
	if strings.TrimSpace(event.EventType) != NodeHeartbeatEventType {
		return models.NodeState{}, fmt.Errorf("unsupported event_type %q", event.EventType)
	}
	nodeID := strings.TrimSpace(event.NodeID)
	if nodeID == "" {
		return models.NodeState{}, fmt.Errorf("node_id is required")
	}
	endpointGroup := strings.TrimSpace(event.EndpointGroup)
	if endpointGroup == "" {
		return models.NodeState{}, fmt.Errorf("endpoint_group is required")
	}
	protocol := strings.TrimSpace(event.Protocol)
	if protocol == "" {
		return models.NodeState{}, fmt.Errorf("protocol is required")
	}

	status := strings.TrimSpace(event.Status)
	if status == "" {
		status = models.ServerStatusOnline
	}

	now := s.now().UTC()
	var sentAt *time.Time
	if !event.SentAt.IsZero() {
		value := event.SentAt.UTC()
		sentAt = &value
	}

	node, err := s.nodeRepo.UpsertHeartbeat(repository.NodeHeartbeatUpdate{
		NodeID:        nodeID,
		EndpointGroup: endpointGroup,
		Protocol:      protocol,
		AgentVersion:  strings.TrimSpace(event.AgentVersion),
		Status:        status,
		LastSeenAt:    now,
		LastError:     event.LastError,
		SentAt:        sentAt,
	})
	if err != nil {
		return models.NodeState{}, err
	}
	node.Status = s.CalculateStatus(node.LastSeenAt)
	return node, nil
}

func (s *NodeService) ListNodes(ctx context.Context) ([]models.NodeState, error) {
	_ = ctx
	nodes, err := s.nodeRepo.List()
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		nodes[i].Status = s.CalculateStatus(nodes[i].LastSeenAt)
	}
	return nodes, nil
}

func (s *NodeService) GetNode(ctx context.Context, nodeID string) (models.NodeState, error) {
	_ = ctx
	node, err := s.nodeRepo.GetByNodeID(strings.TrimSpace(nodeID))
	if err != nil {
		return models.NodeState{}, err
	}
	node.Status = s.CalculateStatus(node.LastSeenAt)
	return node, nil
}

func (s *NodeService) CalculateStatus(lastSeen time.Time) string {
	if lastSeen.IsZero() {
		return models.ServerStatusOffline
	}
	age := s.now().UTC().Sub(lastSeen.UTC())
	if age <= nodeOnlineWindow {
		return models.ServerStatusOnline
	}
	if age <= nodeStaleWindow {
		return models.ServerStatusStale
	}
	return models.ServerStatusOffline
}
