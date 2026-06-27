package service

import (
	"context"
	"testing"
	"time"
	"vpnpanel/internal/broker"
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
	if result.NodeID != "direct-1" || result.Status != "queued" || result.CommandID == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(publisher.commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(publisher.commands))
	}
	cmd := publisher.commands[0]
	if cmd.EventType != "collect_snapshot" || cmd.TargetNodeID != "direct-1" || cmd.RequestedBy != "admin" {
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
