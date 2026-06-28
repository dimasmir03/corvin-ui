package service

import (
	"context"
	"testing"
	"time"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/jobsvc"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
)

type captureSnapshotPublisher struct {
	commands []broker.CollectSnapshotCommand
}

func (p *captureSnapshotPublisher) PublishCollectSnapshotCommand(msg broker.CollectSnapshotCommand) error {
	p.commands = append(p.commands, msg)
	return nil
}

func newTestNodeService(dbRepo *repository.NodeRepo, publisher SnapshotCommandPublisher, now time.Time) *NodeService {
	svc := NewNodeService(dbRepo, publisher)
	svc.now = func() time.Time { return now }
	return svc
}

func TestApplySnapshotCreatesDiscoveredNode(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	svc := newTestNodeService(repository.NewNodeRepo(db), &captureSnapshotPublisher{}, now)
	inboundID := 7

	node, stale, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{
		EventType:     NodeSnapshotEventType,
		NodeID:        "direct-1",
		EndpointGroup: "direct",
		Protocol:      "vless",
		AgentVersion:  "1.2.3",
		InboundID:     &inboundID,
		InboundRemark: "direct-vless",
		XUIAvailable:  true,
		ClientsCount:  10,
		OnlineCount:   3,
		TrafficUp:     100,
		TrafficDown:   200,
		SentAt:        now,
	})
	if err != nil {
		t.Fatalf("ApplySnapshot: %v", err)
	}
	if stale {
		t.Fatalf("stale = true, want false")
	}
	if node.Source != models.NodeSourceDiscovered {
		t.Fatalf("source = %q", node.Source)
	}
	if node.Status != models.ServerStatusOnline || node.ClientsCount != 10 || node.OnlineCount != 3 || node.InboundID == nil || *node.InboundID != inboundID {
		t.Fatalf("unexpected node: %#v", node)
	}
}

func TestApplySnapshotUpdatesExistingDiscoveredNode(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	repo := repository.NewNodeRepo(db)
	svc := newTestNodeService(repo, &captureSnapshotPublisher{}, now)

	if _, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, NodeID: "ru-1", EndpointGroup: "ru", Protocol: "trojan", XUIAvailable: true, ClientsCount: 1, OnlineCount: 1, SentAt: now}); err != nil {
		t.Fatalf("first ApplySnapshot: %v", err)
	}
	updatedAt := now.Add(time.Minute)
	node, stale, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, NodeID: "ru-1", EndpointGroup: "ru", Protocol: "trojan", XUIAvailable: false, ClientsCount: 5, OnlineCount: 0, LastError: "xui unavailable", SentAt: updatedAt})
	if err != nil {
		t.Fatalf("second ApplySnapshot: %v", err)
	}
	if stale {
		t.Fatalf("stale = true, want false")
	}
	if node.Source != models.NodeSourceDiscovered || node.ClientsCount != 5 || node.OnlineCount != 0 || node.LastError != "xui unavailable" || node.XUIAvailable == nil || *node.XUIAvailable {
		t.Fatalf("unexpected updated node: %#v", node)
	}
}

func TestApplySnapshotXUIUnavailableWithEmptyProtocolCreatesUnknownProtocolNode(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	svc := newTestNodeService(repository.NewNodeRepo(db), &captureSnapshotPublisher{}, now)

	node, stale, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{
		EventType:     NodeSnapshotEventType,
		NodeID:        "02",
		EndpointGroup: "foreign",
		Protocol:      "",
		XUIAvailable:  false,
		ClientsCount:  0,
		OnlineCount:   0,
		LastError:     "3x-ui unavailable",
		SentAt:        now,
	})
	if err != nil {
		t.Fatalf("ApplySnapshot: %v", err)
	}
	if stale {
		t.Fatalf("stale = true, want false")
	}
	if node.EndpointGroup != "foreign" || node.Protocol != models.ServerStatusUnknown {
		t.Fatalf("endpoint/protocol = %q/%q", node.EndpointGroup, node.Protocol)
	}
	if node.XUIAvailable == nil || *node.XUIAvailable {
		t.Fatalf("xui_available = %#v, want false", node.XUIAvailable)
	}
	if node.Status != models.ServerStatusOnline {
		t.Fatalf("status = %q, want online", node.Status)
	}
}

func TestApplySnapshotXUIUnavailableSavesLastErrorAndListsNode(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	svc := newTestNodeService(repository.NewNodeRepo(db), &captureSnapshotPublisher{}, now)

	if _, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{
		EventType:     NodeSnapshotEventType,
		NodeID:        "02",
		EndpointGroup: "foreign",
		Protocol:      "",
		XUIAvailable:  false,
		LastError:     "connection refused",
		SentAt:        now,
	}); err != nil {
		t.Fatalf("ApplySnapshot: %v", err)
	}

	nodes, err := svc.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(nodes))
	}
	node := nodes[0]
	if node.NodeID != "02" || node.EndpointGroup != "foreign" || node.Protocol != models.ServerStatusUnknown || node.LastError != "connection refused" {
		t.Fatalf("unexpected node from list: %#v", node)
	}
	if node.XUIAvailable == nil || *node.XUIAvailable {
		t.Fatalf("xui_available = %#v, want false", node.XUIAvailable)
	}
}

func TestApplySnapshotEmptyEndpointGroupFallsBackToUnknown(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	svc := newTestNodeService(repository.NewNodeRepo(db), &captureSnapshotPublisher{}, now)

	node, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, NodeID: "03", EndpointGroup: "", Protocol: "", XUIAvailable: false, SentAt: now})
	if err != nil {
		t.Fatalf("ApplySnapshot: %v", err)
	}
	if node.EndpointGroup != models.ServerStatusUnknown || node.Protocol != models.ServerStatusUnknown {
		t.Fatalf("endpoint/protocol = %q/%q, want unknown/unknown", node.EndpointGroup, node.Protocol)
	}
}

func TestRequestSnapshotPublishesCollectSnapshot(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	repo := repository.NewNodeRepo(db)
	publisher := &captureSnapshotPublisher{}
	svc := newTestNodeService(repo, publisher, now)
	if _, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, NodeID: "direct-1", EndpointGroup: "direct", Protocol: "vless", XUIAvailable: true, SentAt: now}); err != nil {
		t.Fatalf("ApplySnapshot: %v", err)
	}

	result, err := svc.RequestSnapshot(context.Background(), "direct-1", "admin")
	if err != nil {
		t.Fatalf("RequestSnapshot: %v", err)
	}
	if result.ServerID != "direct-1" || result.Status != "queued" || result.CommandID == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(publisher.commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(publisher.commands))
	}
	cmd := publisher.commands[0]
	if cmd.EventType != "collect_snapshot" || cmd.TargetServerID != "direct-1" || cmd.ServerID != "direct-1" || cmd.RequestedBy != "admin" {
		t.Fatalf("unexpected command: %#v", cmd)
	}
}

func TestRequestSnapshotUnknownNodeDoesNotPublish(t *testing.T) {
	db := newVPNServiceTestDB(t)
	publisher := &captureSnapshotPublisher{}
	svc := newTestNodeService(repository.NewNodeRepo(db), publisher, time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RequestSnapshot(context.Background(), "missing-node", "admin"); err == nil {
		t.Fatalf("RequestSnapshot error = nil, want error")
	}
	if len(publisher.commands) != 0 {
		t.Fatalf("commands = %d, want 0", len(publisher.commands))
	}
}

func TestApplySnapshotUsesServerIDAsPrimaryKey(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC)
	svc := newTestNodeService(repository.NewNodeRepo(db), nil, now)

	node, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, ServerID: "foreign-01", NodeID: "legacy-different", EndpointGroup: "foreign", Protocol: "vless", XUIAvailable: true, SentAt: now})
	if err != nil {
		t.Fatalf("ApplySnapshot: %v", err)
	}
	if node.ServerID != "foreign-01" || node.NodeID != "foreign-01" {
		t.Fatalf("unexpected identity: %#v", node)
	}
}

func TestApplySnapshotRejectsWhenServerAndLegacyNodeIDMissing(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC)
	svc := newTestNodeService(repository.NewNodeRepo(db), nil, now)

	if _, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, EndpointGroup: "foreign", Protocol: "vless", XUIAvailable: true, SentAt: now}); err == nil {
		t.Fatalf("ApplySnapshot error = nil, want server_id required")
	}
}

func TestApplySnapshotDifferentServerIDsCreateDifferentRecords(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC)
	svc := newTestNodeService(repository.NewNodeRepo(db), nil, now)

	for _, id := range []string{"foreign-01", "foreign-02"} {
		if _, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, ServerID: id, EndpointGroup: "foreign", Protocol: "vless", XUIAvailable: true, SentAt: now}); err != nil {
			t.Fatalf("ApplySnapshot %s: %v", id, err)
		}
	}
	nodes, err := svc.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(nodes))
	}
}

func TestApplySnapshotSameServerIDUpdatesOneRecord(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC)
	svc := newTestNodeService(repository.NewNodeRepo(db), nil, now)

	if _, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, ServerID: "foreign-01", EndpointGroup: "foreign", Protocol: "vless", XUIAvailable: true, ClientsCount: 1, SentAt: now}); err != nil {
		t.Fatalf("first ApplySnapshot: %v", err)
	}
	if _, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, ServerID: "foreign-01", EndpointGroup: "foreign", Protocol: "vless", XUIAvailable: true, ClientsCount: 2, SentAt: now.Add(time.Second)}); err != nil {
		t.Fatalf("second ApplySnapshot: %v", err)
	}
	nodes, err := svc.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ClientsCount != 2 {
		t.Fatalf("unexpected nodes: %#v", nodes)
	}
}

func TestApplySnapshotWritesRegistryLatestAndHistory(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	svc := newTestNodeService(repository.NewNodeRepo(db), nil, now)

	if _, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, ServerID: "foreign-01", EndpointGroup: "foreign", Protocol: "vless", XUIAvailable: false, LastError: "xui unavailable", SentAt: now}); err != nil {
		t.Fatalf("ApplySnapshot: %v", err)
	}

	var registry models.ServerRegistry
	if err := db.Where("server_id = ?", "foreign-01").Take(&registry).Error; err != nil {
		t.Fatalf("server_registry row missing: %v", err)
	}
	if registry.Source != models.NodeSourceDiscovered || registry.EndpointGroup != "foreign" || registry.ExpectedProtocol != jobsvc.VPNProfileVLESS {
		t.Fatalf("unexpected registry: %#v", registry)
	}

	var state models.NodeState
	if err := db.Where("server_id = ?", "foreign-01").Take(&state).Error; err != nil {
		t.Fatalf("node_states row missing: %v", err)
	}
	if state.ReportedProtocol != jobsvc.VPNProfileVLESS || state.LastError != "xui unavailable" || state.XUIAvailable == nil || *state.XUIAvailable {
		t.Fatalf("unexpected latest state: %#v", state)
	}

	var historyCount int64
	if err := db.Model(&models.NodeStateSnapshot{}).Where("server_id = ?", "foreign-01").Count(&historyCount).Error; err != nil {
		t.Fatalf("count node_state_snapshots: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("history rows = %d, want 1", historyCount)
	}
}

func TestArchiveAndRestoreServerAffectsListNodes(t *testing.T) {
	db := newVPNServiceTestDB(t)
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	svc := newTestNodeService(repository.NewNodeRepo(db), nil, now)

	if _, _, err := svc.ApplySnapshot(context.Background(), broker.NodeSnapshotEvent{EventType: NodeSnapshotEventType, ServerID: "foreign-01", EndpointGroup: "foreign", Protocol: "vless", XUIAvailable: true, SentAt: now}); err != nil {
		t.Fatalf("ApplySnapshot: %v", err)
	}
	if err := svc.ArchiveServer(context.Background(), "foreign-01", "test archive"); err != nil {
		t.Fatalf("ArchiveServer: %v", err)
	}
	nodes, err := svc.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("ListNodes after archive: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("nodes after archive = %d, want 0", len(nodes))
	}
	if err := svc.RestoreServer(context.Background(), "foreign-01"); err != nil {
		t.Fatalf("RestoreServer: %v", err)
	}
	nodes, err = svc.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("ListNodes after restore: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ServerID != "foreign-01" {
		t.Fatalf("unexpected nodes after restore: %#v", nodes)
	}
}
