package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
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
	logger.Info("manual refresh requested", "component", "node_service", "operation", "request_snapshot", "node_id", nodeID, "requested_by", requestedBy)
	if nodeID == "" {
		logger.Warn("manual refresh failed", "component", "node_service", "operation", "request_snapshot", "node_id", nodeID, "reason", "node_id_required")
		return RequestSnapshotResult{}, fmt.Errorf("node_id is required")
	}
	if requestedBy == "" {
		requestedBy = "admin"
	}
	if _, err := s.nodeRepo.GetByNodeID(nodeID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("manual refresh failed", "component", "node_service", "operation", "request_snapshot", "node_id", nodeID, "requested_by", requestedBy, "reason", "unknown_node")
		} else {
			logger.Error("manual refresh failed", err, "component", "node_service", "operation", "request_snapshot", "node_id", nodeID, "requested_by", requestedBy, "reason", "lookup_failed")
		}
		return RequestSnapshotResult{}, err
	}
	if s.publisher == nil {
		logger.Warn("manual refresh failed", "component", "node_service", "operation", "request_snapshot", "node_id", nodeID, "requested_by", requestedBy, "reason", "publisher_not_initialized")
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

	loggerFields := []any{"component", "node_service", "operation", "request_snapshot", "command_id", commandID, "node_id", nodeID, "requested_by", requestedBy}
	logger.Info("manual refresh command publish started", loggerFields...)
	if err := s.publisher.PublishCollectSnapshotCommand(cmd); err != nil {
		logger.Error("manual refresh failed", err, append(loggerFields, "reason", "publish_failed")...)
		return RequestSnapshotResult{}, err
	}
	logger.Info("manual refresh command published", append(loggerFields, "reason", "command_published")...)

	return RequestSnapshotResult{CommandID: commandID, NodeID: nodeID, Status: "queued"}, nil
}

func (s *NodeService) ApplySnapshot(ctx context.Context, event broker.NodeSnapshotEvent) (models.NodeState, bool, error) {
	_ = ctx
	logger.Info("node snapshot received", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
	if strings.TrimSpace(event.EventType) != NodeSnapshotEventType {
		logger.Warn("node snapshot rejected", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "reason", "unsupported_event_type")
		return models.NodeState{}, false, fmt.Errorf("unsupported event_type %q", event.EventType)
	}
	nodeID := strings.TrimSpace(event.NodeID)
	if nodeID == "" {
		logger.Warn("node snapshot rejected", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "reason", "node_id_required")
		return models.NodeState{}, false, fmt.Errorf("node_id is required")
	}
	endpointGroup := strings.TrimSpace(event.EndpointGroup)
	if endpointGroup == "" {
		endpointGroup = models.ServerStatusUnknown
		logger.Warn("node snapshot endpoint group missing", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "node_id", nodeID, "reason", "endpoint_group_missing", "fallback_endpoint_group", endpointGroup)
	}
	protocol := strings.TrimSpace(event.Protocol)
	if protocol == "" {
		protocol = models.ServerStatusUnknown
		reason := "protocol_missing"
		if !event.XUIAvailable {
			reason = "protocol_missing_xui_unavailable"
		}
		logger.Warn("node snapshot protocol missing", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "node_id", nodeID, "endpoint_group", endpointGroup, "xui_available", event.XUIAvailable, "reason", reason, "fallback_protocol", protocol)
	}

	snapshotAt := s.now().UTC()
	if !event.SentAt.IsZero() {
		snapshotAt = event.SentAt.UTC()
	}

	created := false
	logger.Info("node snapshot lookup started", "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID)
	if existing, err := s.nodeRepo.GetByNodeID(nodeID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created = true
			logger.Info("node state not found", "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID, "reason", "discovered_node_missing")
			logger.Info("discovered node create started", "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID, "endpoint_group", endpointGroup, "protocol", protocol)
		} else {
			logger.Error("node snapshot lookup failed", err, "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID, "reason", "db_error")
			return models.NodeState{}, false, err
		}
	} else {
		logger.Info("node state found", "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID, "source", existing.Source, "status", existing.Status)
		logger.Info("node state update started", "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID, "endpoint_group", endpointGroup, "protocol", protocol)
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
		logger.Error("node snapshot processing failed", err, "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "node_id", nodeID, "endpoint_group", endpointGroup, "protocol", protocol, "reason", "state_update_failed")
		return models.NodeState{}, false, err
	}
	node.Status = s.calculateNodeStatus(node)
	if stale {
		logger.Info("node snapshot processing finished", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "node_id", nodeID, "endpoint_group", endpointGroup, "protocol", protocol, "status", node.Status, "reason", "stale_snapshot")
		return node, stale, nil
	}
	xuiAvailable := false
	if node.XUIAvailable != nil {
		xuiAvailable = *node.XUIAvailable
	}
	logger.Info("node state upserted", "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID, "endpoint_group", endpointGroup, "protocol", protocol, "xui_available", xuiAvailable, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "source", node.Source, "reason", "snapshot_applied")
	if created {
		logger.Info("discovered node created", "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID, "endpoint_group", endpointGroup, "protocol", protocol, "source", node.Source)
		logger.Info("node snapshot processing finished", "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID, "endpoint_group", endpointGroup, "protocol", protocol, "reason", "discovered_created")
		return node, stale, nil
	}
	logger.Info("node state updated", "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID, "xui_available", event.XUIAvailable, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "source", node.Source)
	logger.Info("node snapshot processing finished", "component", "node_service", "operation", "apply_snapshot", "node_id", nodeID, "endpoint_group", endpointGroup, "protocol", protocol, "reason", "state_updated")
	return node, stale, nil
}

func (s *NodeService) ListNodes(ctx context.Context) ([]models.NodeState, error) {
	_ = ctx
	logger.Info("servers page requested", "component", "node_service", "operation", "list_nodes")
	nodes, err := s.nodeRepo.List()
	if err != nil {
		logger.Error("servers page data load failed", err, "component", "node_service", "operation", "list_nodes", "reason", "db_error")
		return nil, err
	}
	discoveredCount := 0
	for i := range nodes {
		nodes[i].Status = s.calculateNodeStatus(nodes[i])
		if strings.TrimSpace(nodes[i].Source) == "" {
			nodes[i].Source = models.NodeSourceKnown
		}
		if nodes[i].Source == models.NodeSourceDiscovered {
			discoveredCount++
		}
	}
	logger.Info("servers page data loaded", "component", "node_service", "operation", "list_nodes", "nodes_count", len(nodes), "discovered_count", discoveredCount)
	return nodes, nil
}

func (s *NodeService) GetNode(ctx context.Context, nodeID string) (models.NodeState, error) {
	_ = ctx
	node, err := s.nodeRepo.GetByNodeID(strings.TrimSpace(nodeID))
	if err != nil {
		return models.NodeState{}, err
	}
	node.Status = s.calculateNodeStatus(node)
	if strings.TrimSpace(node.Source) == "" {
		node.Source = models.NodeSourceKnown
	}
	return node, nil
}

func (s *NodeService) calculateNodeStatus(node models.NodeState) string {
	if node.LastSnapshotAt == nil {
		return models.ServerStatusOffline
	}
	return s.CalculateStatus(*node.LastSnapshotAt)
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
