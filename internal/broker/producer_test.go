package broker

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/wagslane/go-rabbitmq"
)

func TestResultQueueValidJobResultCallsApply(t *testing.T) {
	calls := 0
	body := mustJSON(t, JobResultEvent{EventType: "job_result", JobID: 42, BatchID: 7, NodeID: "01", Status: "success"})
	action := handleResultQueueMessage("corvin.job.results", body, func(event JobResultEvent) error {
		calls++
		if event.JobID != 42 || event.NodeID != "01" || event.Status != "success" {
			t.Fatalf("unexpected event: %#v", event)
		}
		return nil
	}, nil)
	if action != rabbitmq.Ack {
		t.Fatalf("action = %v, want Ack", action)
	}
	if calls != 1 {
		t.Fatalf("job handler calls = %d, want 1", calls)
	}
}

func TestResultQueueInvalidJobResultMissingJobIDIsAckedWithoutApply(t *testing.T) {
	calls := 0
	body := []byte(`{"event_type":"job_result","job_id":0,"node_id":"01","status":"success"}`)
	action := handleResultQueueMessage("corvin.job.results", body, func(event JobResultEvent) error {
		calls++
		return nil
	}, nil)
	if action != rabbitmq.Ack {
		t.Fatalf("action = %v, want Ack", action)
	}
	if calls != 0 {
		t.Fatalf("job handler calls = %d, want 0", calls)
	}
}

func TestResultQueueNodeSnapshotIsRoutedToSnapshotHandler(t *testing.T) {
	jobCalls := 0
	snapshotCalls := 0
	sentAt := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	body := mustJSON(t, NodeSnapshotEvent{EventType: "node_snapshot", NodeID: "01", EndpointGroup: "direct", Protocol: "vless", XUIAvailable: true, ClientsCount: 11, OnlineCount: 3, SentAt: sentAt})
	action := handleResultQueueMessage("corvin.job.results", body, func(event JobResultEvent) error {
		jobCalls++
		return nil
	}, func(event NodeSnapshotEvent) (bool, error) {
		snapshotCalls++
		if event.NodeID != "01" || event.EndpointGroup != "direct" || event.Protocol != "vless" {
			t.Fatalf("unexpected snapshot: %#v", event)
		}
		return false, nil
	})
	if action != rabbitmq.Ack {
		t.Fatalf("action = %v, want Ack", action)
	}
	if jobCalls != 0 {
		t.Fatalf("job handler calls = %d, want 0", jobCalls)
	}
	if snapshotCalls != 1 {
		t.Fatalf("snapshot handler calls = %d, want 1", snapshotCalls)
	}
}

func TestAgentEventQueueNodeSnapshotIsApplied(t *testing.T) {
	snapshotCalls := 0
	body := []byte(`{"event_type":"node_snapshot","server_id":"01","endpoint_group":"ru","protocol":"trojan","xui_available":true,"sent_at":"2026-06-28T10:00:00Z"}`)
	action := handleAgentEventQueueMessage("agent.events", "corvin.agent.events", "node.snapshot", body, func(event NodeSnapshotEvent) (bool, error) {
		snapshotCalls++
		if event.NodeID != "01" || event.ServerID != "01" {
			t.Fatalf("server_id fallback failed: %#v", event)
		}
		return false, nil
	})
	if action != rabbitmq.Ack {
		t.Fatalf("action = %v, want Ack", action)
	}
	if snapshotCalls != 1 {
		t.Fatalf("snapshot handler calls = %d, want 1", snapshotCalls)
	}
}

func TestAgentEventQueueJobResultIsIgnored(t *testing.T) {
	body := mustJSON(t, JobResultEvent{EventType: "job_result", JobID: 42, NodeID: "01", Status: "success"})
	snapshotCalls := 0
	action := handleAgentEventQueueMessage("agent.events", "corvin.agent.events", "node.snapshot", body, func(event NodeSnapshotEvent) (bool, error) {
		snapshotCalls++
		return false, nil
	})
	if action != rabbitmq.Ack {
		t.Fatalf("action = %v, want Ack", action)
	}
	if snapshotCalls != 0 {
		t.Fatalf("snapshot handler calls = %d, want 0", snapshotCalls)
	}
}

func TestJobResultIsNotAppliedTwiceAcrossConsumerFlows(t *testing.T) {
	resultCalls := 0
	agentSnapshotCalls := 0
	body := mustJSON(t, JobResultEvent{EventType: "job_result", JobID: 42, NodeID: "01", Status: "success"})
	if action := handleResultQueueMessage("corvin.job.results", body, func(event JobResultEvent) error {
		resultCalls++
		return nil
	}, nil); action != rabbitmq.Ack {
		t.Fatalf("result action = %v, want Ack", action)
	}
	if action := handleAgentEventQueueMessage("agent.events", "corvin.agent.events", "node.snapshot", body, func(event NodeSnapshotEvent) (bool, error) {
		agentSnapshotCalls++
		return false, nil
	}); action != rabbitmq.Ack {
		t.Fatalf("agent action = %v, want Ack", action)
	}
	if resultCalls != 1 {
		t.Fatalf("result handler calls = %d, want 1", resultCalls)
	}
	if agentSnapshotCalls != 0 {
		t.Fatalf("agent snapshot calls = %d, want 0", agentSnapshotCalls)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}
