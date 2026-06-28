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

	nodeOnlineWindow = 90 * time.Second
	nodeStaleWindow  = 5 * time.Minute
)

type SnapshotCommandPublisher interface {
	PublishCollectSnapshotCommand(msg broker.CollectSnapshotCommand) error
}

type RequestSnapshotResult struct {
	CommandID string `json:"command_id"`
	ServerID  string `json:"server_id"`
	NodeID    string `json:"node_id,omitempty"`
	Status    string `json:"status"`
}

type NodeView struct {
	ServerID         string     `json:"server_id"`
	NodeID           string     `json:"node_id,omitempty"`
	DisplayName      string     `json:"display_name"`
	EndpointGroup    string     `json:"endpoint_group"`
	ExpectedProtocol string     `json:"expected_protocol"`
	ReportedProtocol string     `json:"reported_protocol"`
	Protocol         string     `json:"protocol"`
	ServerRole       string     `json:"server_role"`
	Source           string     `json:"source"`
	Enabled          bool       `json:"enabled"`
	ArchivedAt       *time.Time `json:"archived_at,omitempty"`
	ArchivedReason   string     `json:"archived_reason,omitempty"`
	AgentVersion     string     `json:"agent_version"`
	AgentAlive       bool       `json:"agent_alive"`
	AgentStatus      string     `json:"agent_status"`
	Status           string     `json:"status"`
	XUIAvailable     *bool      `json:"xui_available,omitempty"`
	XUIStatus        string     `json:"xui_status"`
	InboundID        *int       `json:"inbound_id,omitempty"`
	InboundRemark    string     `json:"inbound_remark"`
	ClientsCount     int        `json:"clients_count"`
	OnlineCount      int        `json:"online_count"`
	TrafficUp        int64      `json:"traffic_up"`
	TrafficDown      int64      `json:"traffic_down"`
	LastError        string     `json:"last_error"`
	FirstSeenAt      time.Time  `json:"first_seen_at"`
	LastSeenAt       time.Time  `json:"last_seen"`
	LastSnapshotAt   *time.Time `json:"last_snapshot_at,omitempty"`
}

type NodeService struct {
	nodeRepo  *repository.NodeRepo
	publisher SnapshotCommandPublisher
	now       func() time.Time
}

func NewNodeService(nodeRepo *repository.NodeRepo, publisher SnapshotCommandPublisher) *NodeService {
	return &NodeService{nodeRepo: nodeRepo, publisher: publisher, now: time.Now}
}

func (s *NodeService) RequestSnapshot(ctx context.Context, serverID string, requestedBy string) (RequestSnapshotResult, error) {
	_ = ctx
	serverID = strings.TrimSpace(serverID)
	logger.Info("manual refresh requested", "component", "node_service", "operation", "request_snapshot", "server_id", serverID, "requested_by", requestedBy)
	if serverID == "" {
		logger.Warn("manual refresh failed", "component", "node_service", "operation", "request_snapshot", "server_id", serverID, "reason", "server_id_required")
		return RequestSnapshotResult{}, fmt.Errorf("server_id is required")
	}
	if requestedBy == "" {
		requestedBy = "admin"
	}
	if _, err := s.nodeRepo.GetRecordByServerID(serverID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("manual refresh failed", "component", "node_service", "operation", "request_snapshot", "server_id", serverID, "requested_by", requestedBy, "reason", "unknown_server")
		} else {
			logger.Error("manual refresh failed", err, "component", "node_service", "operation", "request_snapshot", "server_id", serverID, "requested_by", requestedBy, "reason", "lookup_failed")
		}
		return RequestSnapshotResult{}, err
	}
	if s.publisher == nil {
		logger.Warn("manual refresh failed", "component", "node_service", "operation", "request_snapshot", "server_id", serverID, "requested_by", requestedBy, "reason", "publisher_not_initialized")
		return RequestSnapshotResult{}, fmt.Errorf("snapshot command publisher is not initialized")
	}

	commandID := uuid.NewString()
	cmd := broker.CollectSnapshotCommand{EventType: "collect_snapshot", CommandID: commandID, ServerID: serverID, TargetServerID: serverID, RequestedBy: requestedBy, CreatedAt: s.now().UTC()}
	loggerFields := []any{"component", "node_service", "operation", "request_snapshot", "command_id", commandID, "server_id", serverID, "requested_by", requestedBy}
	logger.Info("manual refresh command publish started", loggerFields...)
	if err := s.publisher.PublishCollectSnapshotCommand(cmd); err != nil {
		logger.Error("manual refresh failed", err, append(loggerFields, "reason", "publish_failed")...)
		return RequestSnapshotResult{}, err
	}
	logger.Info("manual refresh command published", append(loggerFields, "reason", "command_published")...)
	return RequestSnapshotResult{CommandID: commandID, ServerID: serverID, NodeID: serverID, Status: "queued"}, nil
}

func (s *NodeService) ApplySnapshot(ctx context.Context, event broker.NodeSnapshotEvent) (models.NodeState, bool, error) {
	_ = ctx
	logger.Info("node snapshot received", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "server_id", event.ServerID, "legacy_node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "expected_protocol", event.Protocol, "reported_protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
	if strings.TrimSpace(event.EventType) != NodeSnapshotEventType {
		logger.Warn("node snapshot rejected", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "reason", "unsupported_event_type")
		return models.NodeState{}, false, fmt.Errorf("unsupported event_type %q", event.EventType)
	}
	serverID := strings.TrimSpace(event.ServerID)
	legacyNodeID := strings.TrimSpace(event.NodeID)
	if serverID == "" && legacyNodeID != "" {
		serverID = legacyNodeID
		logger.Warn("node snapshot server_id fallback", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "server_id", serverID, "legacy_node_id", legacyNodeID, "reason", "legacy_node_id_fallback")
	}
	if serverID == "" {
		logger.Warn("node snapshot rejected", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "reason", "server_id_required")
		return models.NodeState{}, false, fmt.Errorf("server_id is required")
	}

	endpointGroup := fallbackString(event.EndpointGroup, models.ServerStatusUnknown)
	reportedProtocol := fallbackString(event.Protocol, models.ServerStatusUnknown)
	expectedProtocol := reportedProtocol
	serverRole := fallbackString(endpointGroup, "other")
	displayName := serverID
	if event.Protocol == "" {
		reason := "protocol_missing"
		if !event.XUIAvailable {
			reason = "protocol_missing_xui_unavailable"
		}
		logger.Warn("node snapshot protocol missing", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "server_id", serverID, "endpoint_group", endpointGroup, "xui_available", event.XUIAvailable, "reason", reason, "fallback_protocol", reportedProtocol)
	}
	receivedAt := s.now().UTC()
	sentAt := receivedAt
	if !event.SentAt.IsZero() {
		sentAt = event.SentAt.UTC()
	}

	logger.Info("node snapshot lookup started", "component", "node_service", "operation", "apply_snapshot", "server_id", serverID)
	_, existingErr := s.nodeRepo.GetRecordByServerID(serverID)
	created := errors.Is(existingErr, gorm.ErrRecordNotFound)
	if created {
		logger.Info("server registry create started", "component", "node_service", "operation", "apply_snapshot", "server_id", serverID, "endpoint_group", endpointGroup, "expected_protocol", expectedProtocol)
	} else if existingErr != nil {
		logger.Error("node snapshot lookup failed", existingErr, "component", "node_service", "operation", "apply_snapshot", "server_id", serverID, "reason", "db_error")
		return models.NodeState{}, false, existingErr
	}

	node, stale, err := s.nodeRepo.ApplySnapshot(repository.NodeSnapshotUpdate{
		ServerID:         serverID,
		DisplayName:      displayName,
		EndpointGroup:    endpointGroup,
		ExpectedProtocol: expectedProtocol,
		ReportedProtocol: reportedProtocol,
		ServerRole:       serverRole,
		AgentVersion:     strings.TrimSpace(event.AgentVersion),
		AgentAlive:       true,
		XUIAvailable:     event.XUIAvailable,
		InboundID:        event.InboundID,
		InboundRemark:    strings.TrimSpace(event.InboundRemark),
		ClientsCount:     event.ClientsCount,
		OnlineCount:      event.OnlineCount,
		TrafficUp:        event.TrafficUp,
		TrafficDown:      event.TrafficDown,
		LastError:        event.LastError,
		ReceivedAt:       receivedAt,
		SentAt:           sentAt,
	})
	if err != nil {
		logger.Error("node snapshot processing failed", err, "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "server_id", serverID, "endpoint_group", endpointGroup, "reported_protocol", reportedProtocol, "reason", "state_update_failed")
		return models.NodeState{}, false, err
	}
	node.Status = s.calculateNodeStatus(node)
	if stale {
		logger.Info("node snapshot processing finished", "component", "node_service", "operation", "apply_snapshot", "event_type", event.EventType, "server_id", serverID, "endpoint_group", endpointGroup, "reported_protocol", reportedProtocol, "status", node.Status, "reason", "stale_snapshot")
		return node, stale, nil
	}
	xuiAvailable := false
	if node.XUIAvailable != nil {
		xuiAvailable = *node.XUIAvailable
	}
	logger.Info("server registry upserted", "component", "node_service", "operation", "apply_snapshot", "server_id", serverID, "source", models.NodeSourceDiscovered, "reason", "snapshot_received")
	logger.Info("node stats latest upserted", "component", "node_service", "operation", "apply_snapshot", "server_id", serverID, "xui_available", xuiAvailable)
	logger.Info("node stats snapshot inserted", "component", "node_service", "operation", "apply_snapshot", "server_id", serverID)
	logger.Info("node snapshot processing finished", "component", "node_service", "operation", "apply_snapshot", "server_id", serverID, "endpoint_group", endpointGroup, "reported_protocol", reportedProtocol, "reason", map[bool]string{true: "discovered_created", false: "state_updated"}[created])
	return node, stale, nil
}

func (s *NodeService) ListNodes(ctx context.Context) ([]NodeView, error) {
	_ = ctx
	logger.Info("servers page requested", "component", "node_service", "operation", "list_nodes")
	records, err := s.nodeRepo.ListRecords(false)
	if err != nil {
		logger.Error("servers page data load failed", err, "component", "node_service", "operation", "list_nodes", "reason", "db_error")
		return nil, err
	}
	views := make([]NodeView, 0, len(records))	
	discoveredCount := 0
	for _, record := range records {
		view := s.nodeView(record)
		if view.Source == models.NodeSourceDiscovered {
			discoveredCount++
		}
		views = append(views, view)
	}
	logger.Info("servers page data loaded", "component", "node_service", "operation", "list_nodes", "total", len(views), "active", len(views), "archived", 0, "discovered", discoveredCount)
	return views, nil
}

func (s *NodeService) GetNode(ctx context.Context, serverID string) (NodeView, error) {
	_ = ctx
	record, err := s.nodeRepo.GetRecordByServerID(strings.TrimSpace(serverID))
	if err != nil {
		return NodeView{}, err
	}
	return s.nodeView(record), nil
}

func (s *NodeService) ArchiveServer(ctx context.Context, serverID string, reason string) error {
	_ = ctx
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return fmt.Errorf("server_id is required")
	}
	if reason == "" {
		reason = "manual"
	}
	err := s.nodeRepo.ArchiveServer(serverID, reason, s.now().UTC())
	if err != nil {
		logger.Error("server archive failed", err, "component", "node_service", "operation", "archive_server", "server_id", serverID, "reason", reason)
		return err
	}
	logger.Info("server archived", "component", "node_service", "operation", "archive_server", "server_id", serverID, "reason", reason)
	return nil
}

func (s *NodeService) RestoreServer(ctx context.Context, serverID string) error {
	_ = ctx
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return fmt.Errorf("server_id is required")
	}
	err := s.nodeRepo.RestoreServer(serverID)
	if err != nil {
		return err
	}
	logger.Info("server restored", "component", "node_service", "operation", "restore_server", "server_id", serverID)
	return nil
}

func (s *NodeService) ArchiveStaleDiscovered(ctx context.Context, days int) (int64, error) {
	_ = ctx
	if days <= 0 {
		days = 7
	}
	count, err := s.nodeRepo.ArchiveStaleDiscovered(s.now().UTC().AddDate(0, 0, -days), fmt.Sprintf("stale_discovered_%d_days", days), s.now().UTC())
	if err != nil {
		return 0, err
	}
	logger.Info("stale discovered servers archived", "component", "node_service", "operation", "archive_stale_discovered", "count", count, "days", days)
	return count, nil
}

func (s *NodeService) nodeView(record repository.NodeRecord) NodeView {
	registry := record.Registry
	view := NodeView{
		ServerID:         registry.ServerID,
		NodeID:           registry.ServerID,
		DisplayName:      fallbackString(registry.DisplayName, registry.ServerID),
		EndpointGroup:    fallbackString(registry.EndpointGroup, models.ServerStatusUnknown),
		ExpectedProtocol: fallbackString(registry.ExpectedProtocol, models.ServerStatusUnknown),
		ReportedProtocol: models.ServerStatusUnknown,
		Protocol:         models.ServerStatusUnknown,
		ServerRole:       fallbackString(registry.ServerRole, "other"),
		Source:           fallbackString(registry.Source, models.NodeSourceDiscovered),
		Enabled:          registry.Enabled,
		ArchivedAt:       registry.ArchivedAt,
		ArchivedReason:   registry.ArchivedReason,
		AgentStatus:      models.ServerStatusOffline,
		Status:           models.ServerStatusOffline,
		XUIStatus:        models.ServerStatusUnknown,
		FirstSeenAt:      registry.FirstSeenAt,
		LastSeenAt:       registry.LastSeenAt,
	}
	if record.Stats == nil {
		view.AgentStatus = "never_seen"
		view.Status = "never_seen"
		return view
	}
	stats := record.Stats
	view.EndpointGroup = fallbackString(stats.EndpointGroup, view.EndpointGroup)
	view.ExpectedProtocol = fallbackString(stats.ExpectedProtocol, view.ExpectedProtocol)
	view.ReportedProtocol = fallbackString(stats.ReportedProtocol, models.ServerStatusUnknown)
	view.Protocol = view.ReportedProtocol
	view.ServerRole = fallbackString(stats.ServerRole, view.ServerRole)
	view.DisplayName = fallbackString(stats.DisplayName, view.DisplayName)
	view.AgentVersion = stats.AgentVersion
	view.AgentAlive = stats.AgentAlive
	view.AgentStatus = s.calculateAgentStatus(stats.LastSnapshotAt)
	view.Status = view.AgentStatus
	view.XUIAvailable = stats.XUIAvailable
	view.XUIStatus = xuiStatus(stats.XUIAvailable)
	view.InboundID = stats.InboundID
	view.InboundRemark = stats.InboundRemark
	view.ClientsCount = stats.ClientsCount
	view.OnlineCount = stats.OnlineCount
	view.TrafficUp = stats.TrafficUp
	view.TrafficDown = stats.TrafficDown
	view.LastError = stats.LastError
	view.LastSnapshotAt = stats.LastSnapshotAt
	return view
}

func fallbackString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func xuiStatus(value *bool) string {
	if value == nil {
		return models.ServerStatusUnknown
	}
	if *value {
		return "ok"
	}
	return "error"
}

func (s *NodeService) calculateNodeStatus(node models.NodeState) string {
	return s.calculateAgentStatus(node.LastSnapshotAt)
}

func (s *NodeService) calculateAgentStatus(lastSnapshotAt *time.Time) string {
	if lastSnapshotAt == nil || lastSnapshotAt.IsZero() {
		return "never_seen"
	}
	return s.CalculateStatus(*lastSnapshotAt)
}

func (s *NodeService) CalculateStatus(lastSeen time.Time) string {
	if lastSeen.IsZero() {
		return "never_seen"
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
