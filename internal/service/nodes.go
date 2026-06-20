package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"github.com/google/uuid"
)

const (
	NodeSnapshotEventType = "node_snapshot"

	nodeOnlineWindow = 2 * time.Minute
	nodeStaleWindow  = 5 * time.Minute
)

type SnapshotCommandPublisher interface {
	PublishCollectSnapshotCommand(msg broker.CollectSnapshotCommand) error
}

type RequestSnapshotResult struct {
	CommandID string `json:"command_id"`
	NodeID    string `json:"node_id"`
	Status    string `json:"status"`
}

type NodeService struct {
	nodeRepo  *repository.NodeRepo
	publisher SnapshotCommandPublisher
	now       func() time.Time
}

func NewNodeService(nodeRepo *repository.NodeRepo, publisher SnapshotCommandPublisher) *NodeService {
	return &NodeService{nodeRepo: nodeRepo, publisher: publisher, now: time.Now}
}

func (s *NodeService) RequestSnapshot(ctx context.Context, nodeID string, requestedBy string) (RequestSnapshotResult, error) {
	_ = ctx
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return RequestSnapshotResult{}, fmt.Errorf("node_id is required")
	}
	if requestedBy == "" {
		requestedBy = "admin"
	}
	if _, err := s.nodeRepo.GetByNodeID(nodeID); err != nil {
		return RequestSnapshotResult{}, err
	}
	if s.publisher == nil {
		return RequestSnapshotResult{}, fmt.Errorf("snapshot command publisher is not initialized")
	}

	commandID := uuid.NewString()
	cmd := broker.CollectSnapshotCommand{
		EventType:    "collect_snapshot",
		CommandID:    commandID,
		TargetNodeID: nodeID,
		RequestedBy:  requestedBy,
		CreatedAt:    s.now().UTC(),
	}

	loggerFields := []any{"command_id", commandID, "node_id", nodeID, "requested_by", requestedBy}
	logger.Info("node snapshot refresh requested", loggerFields...)
	if err := s.publisher.PublishCollectSnapshotCommand(cmd); err != nil {
		logger.Error("node snapshot refresh publish failed", err, loggerFields...)
		return RequestSnapshotResult{}, err
	}
	logger.Info("node snapshot refresh command published", loggerFields...)

	return RequestSnapshotResult{CommandID: commandID, NodeID: nodeID, Status: "queued"}, nil
}

func (s *NodeService) ApplySnapshot(ctx context.Context, event broker.NodeSnapshotEvent) (models.NodeState, bool, error) {
	_ = ctx
	if strings.TrimSpace(event.EventType) != NodeSnapshotEventType {
		return models.NodeState{}, false, fmt.Errorf("unsupported event_type %q", event.EventType)
	}
	nodeID := strings.TrimSpace(event.NodeID)
	if nodeID == "" {
		return models.NodeState{}, false, fmt.Errorf("node_id is required")
	}
	endpointGroup := strings.TrimSpace(event.EndpointGroup)
	if endpointGroup == "" {
		return models.NodeState{}, false, fmt.Errorf("endpoint_group is required")
	}
	protocol := strings.TrimSpace(event.Protocol)
	if protocol == "" {
		return models.NodeState{}, false, fmt.Errorf("protocol is required")
	}

	snapshotAt := s.now().UTC()
	if !event.SentAt.IsZero() {
		snapshotAt = event.SentAt.UTC()
	}

	node, stale, err := s.nodeRepo.ApplySnapshot(repository.NodeSnapshotUpdate{
		NodeID:         nodeID,
		EndpointGroup:  endpointGroup,
		Protocol:       protocol,
		AgentVersion:   strings.TrimSpace(event.AgentVersion),
		XUIAvailable:   event.XUIAvailable,
		InboundID:      event.InboundID,
		InboundRemark:  strings.TrimSpace(event.InboundRemark),
		ClientsCount:   event.ClientsCount,
		OnlineCount:    event.OnlineCount,
		TrafficUp:      event.TrafficUp,
		TrafficDown:    event.TrafficDown,
		LastError:      event.LastError,
		LastSeenAt:     snapshotAt,
		LastSnapshotAt: snapshotAt,
	})
	if err != nil {
		return models.NodeState{}, false, err
	}
	node.Status = s.CalculateStatus(node.LastSeenAt)
	return node, stale, nil
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
