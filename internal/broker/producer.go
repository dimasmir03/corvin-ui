package broker

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"vpnpanel/internal/logger"

	"github.com/wagslane/go-rabbitmq"
)

type Producer struct {
	conn *rabbitmq.Conn

	publisherComplaints *rabbitmq.Publisher
	publisherUsers      *rabbitmq.Publisher
	publisherJobs       *rabbitmq.Publisher
	publisherCommands   *rabbitmq.Publisher

	exchangeComplaints string
	exchangeUsers      string
	exchangeJobs       string
	exchangeCommands   string
}

func NewProducer(url, exchangeComplaints, exchangeUsers, exchangeCommands, certfile, keyfile, cafile string) (*Producer, error) {
	rootCAs, err := loadRootCAs(cafile)
	if err != nil {
		return nil, fmt.Errorf("failed to load root CAs: %w", err)
	}

	cert, err := tls.LoadX509KeyPair(certfile, keyfile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		RootCAs:      rootCAs,
		Certificates: []tls.Certificate{cert},
		ServerName:   "rabbitmq", // Optional
	}

	conn, err := rabbitmq.NewConn(
		url,
		rabbitmq.WithConnectionOptionsLogging,
		rabbitmq.WithConnectionOptionsConfig(rabbitmq.Config{
			TLSClientConfig: tlsConfig,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	publisherComplaints, err := rabbitmq.NewPublisher(
		conn,
		rabbitmq.WithPublisherOptionsExchangeName(exchangeComplaints),
		rabbitmq.WithPublisherOptionsExchangeKind("fanout"),
		rabbitmq.WithPublisherOptionsExchangeDeclare,
		rabbitmq.WithPublisherOptionsLogging,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher for complaints: %w", err)
	}

	publisherUsers, err := rabbitmq.NewPublisher(
		conn,
		rabbitmq.WithPublisherOptionsExchangeName(exchangeUsers),
		rabbitmq.WithPublisherOptionsExchangeKind("fanout"),
		rabbitmq.WithPublisherOptionsExchangeDeclare,
		rabbitmq.WithPublisherOptionsLogging,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher: %w", err)
	}

	exchangeJobs := exchangeCommands
	publisherJobs, err := rabbitmq.NewPublisher(
		conn,
		rabbitmq.WithPublisherOptionsExchangeName(exchangeJobs),
		rabbitmq.WithPublisherOptionsExchangeKind("topic"),
		rabbitmq.WithPublisherOptionsExchangeDeclare,
		rabbitmq.WithPublisherOptionsLogging,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher for jobs: %w", err)
	}

	publisherCommands, err := rabbitmq.NewPublisher(
		conn,
		rabbitmq.WithPublisherOptionsExchangeName(exchangeCommands),
		rabbitmq.WithPublisherOptionsExchangeKind("topic"),
		rabbitmq.WithPublisherOptionsExchangeDeclare,
		rabbitmq.WithPublisherOptionsLogging,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher for commands: %w", err)
	}

	logger.Info("rabbit command publisher ready", "exchange", exchangeCommands, "create_exchange", exchangeJobs, "collect_exchange", exchangeCommands)
	logger.Info("rabbit legacy publishers ready", "exchange_complaints", exchangeComplaints, "exchange_users", exchangeUsers)

	return &Producer{
		conn:                conn,
		publisherComplaints: publisherComplaints,
		publisherUsers:      publisherUsers,
		publisherJobs:       publisherJobs,
		publisherCommands:   publisherCommands,
		exchangeComplaints:  exchangeComplaints,
		exchangeUsers:       exchangeUsers,
		exchangeJobs:        exchangeJobs,
		exchangeCommands:    exchangeCommands,
	}, nil
}

func (p *Producer) PublishComplaintReply(msg any) error {
	return p.publish(p.publisherComplaints, p.exchangeComplaints, msg)
}

func (p *Producer) PublishCreateUser(msg any) error {
	return p.publish(p.publisherUsers, p.exchangeUsers, msg)
}

func (p *Producer) PublishJob(msg JobTask) error {
	routingKey := ""
	if msg.EventType == "create_client" || msg.Action == "create_client" {
		serverID := effectiveJobTaskServerID(msg)
		routingKey = createClientRoutingKey(msg)
		logger.Info("create_client command routing resolved", "component", "rabbitmq", "operation", "publish", "exchange", p.exchangeJobs, "routing_key", routingKey, "job_id", msg.JobID, "batch_id", msg.BatchID, "profile_id", msg.ProfileID, "server_id", serverID, "profile", msg.Profile, "target_group", msg.TargetGroup, "protocol", msg.Protocol, "client_code", msg.ClientCode)
	}
	return p.publishWithRoutingKey(p.publisherJobs, p.exchangeJobs, routingKey, msg)
}

func (p *Producer) PublishCollectSnapshotCommand(msg CollectSnapshotCommand) error {
	serverID := effectiveCollectSnapshotServerID(msg)
	routingKey := collectSnapshotRoutingKey(msg)
	logger.Info("collect_snapshot command routing resolved", "component", "rabbitmq", "operation", "publish", "exchange", p.exchangeCommands, "routing_key", routingKey, "command_id", msg.CommandID, "server_id", serverID, "target_group", msg.TargetGroup, "requested_by", msg.RequestedBy)
	return p.publishWithRoutingKey(p.publisherCommands, p.exchangeCommands, routingKey, msg)
}

func createClientRoutingKey(msg JobTask) string {
	serverID := effectiveJobTaskServerID(msg)
	if serverID != "" {
		return "create.server." + serverID
	}
	if msg.TargetGroup != "" {
		return "create.group." + msg.TargetGroup
	}
	return ""
}

func collectSnapshotRoutingKey(msg CollectSnapshotCommand) string {
	serverID := effectiveCollectSnapshotServerID(msg)
	if serverID != "" {
		return "collect.server." + serverID
	}
	if msg.TargetGroup != "" {
		return "collect.group." + msg.TargetGroup
	}
	return "collect.group.all"
}

func effectiveJobTaskServerID(msg JobTask) string {
	if strings.TrimSpace(msg.TargetServerID) != "" {
		return strings.TrimSpace(msg.TargetServerID)
	}
	if strings.TrimSpace(msg.ServerID) != "" {
		return strings.TrimSpace(msg.ServerID)
	}
	return strings.TrimSpace(msg.NodeID)
}

func effectiveCollectSnapshotServerID(msg CollectSnapshotCommand) string {
	if strings.TrimSpace(msg.TargetServerID) != "" {
		return strings.TrimSpace(msg.TargetServerID)
	}
	if strings.TrimSpace(msg.ServerID) != "" {
		return strings.TrimSpace(msg.ServerID)
	}
	return strings.TrimSpace(msg.TargetNodeID)
}

type JobResultHandler func(JobResultEvent) error
type NodeSnapshotHandler func(NodeSnapshotEvent) (bool, error)

func (p *Producer) StartResultConsumer(queue string, jobHandler JobResultHandler, snapshotHandler NodeSnapshotHandler) (*rabbitmq.Consumer, error) {
	if p == nil || p.conn == nil {
		return nil, fmt.Errorf("rabbitmq connection is not initialized")
	}
	if queue == "" {
		return nil, fmt.Errorf("result queue is required")
	}

	consumer, err := rabbitmq.NewConsumer(
		p.conn,
		queue,
		rabbitmq.WithConsumerOptionsQueueDurable,
		rabbitmq.WithConsumerOptionsConcurrency(1),
		rabbitmq.WithConsumerOptionsQOSPrefetch(10),
		rabbitmq.WithConsumerOptionsLogging,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create result consumer: %w", err)
	}

	logger.Info("rabbit result consumer started", "queue", queue)

	go func() {
		err := consumer.Run(func(d rabbitmq.Delivery) rabbitmq.Action {
			return handleResultQueueMessage(queue, d.Body, jobHandler, snapshotHandler)
		})
		if err != nil {
			logger.Error("job result consumer stopped", err, "component", "rabbitmq", "operation", "consume", "queue", queue)
		}
	}()

	return consumer, nil
}

func handleResultQueueMessage(queue string, body []byte, jobHandler JobResultHandler, snapshotHandler NodeSnapshotHandler) rabbitmq.Action {
	logger.Info("rabbit message received", "component", "rabbitmq", "operation", "consume", "queue", queue, "payload_size", len(body))
	envelope, ok := parseEventEnvelope(queue, body, "result_queue")
	if !ok {
		return rabbitmq.Ack
	}

	switch envelope.EventType {
	case "job_result":
		return handleResultQueueJobResult(queue, body, jobHandler)
	case "node_snapshot":
		logger.Warn("rabbit message routed", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", envelope.EventType, "reason", "node_snapshot_in_result_queue")
		return handleResultQueueNodeSnapshot(queue, body, snapshotHandler)
	default:
		logger.Warn("rabbit message ignored", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", envelope.EventType, "reason", "unsupported_event_type")
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", envelope.EventType, "reason", "unsupported_event_type")
		return rabbitmq.Ack
	}
}

func handleResultQueueJobResult(queue string, body []byte, handler JobResultHandler) rabbitmq.Action {
	var event JobResultEvent
	if err := json.Unmarshal(body, &event); err != nil {
		logger.Error("rabbit message invalid", err, "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "payload_size", len(body))
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "reason", "invalid_payload")
		return rabbitmq.Ack
	}
	logger.Info("rabbit message parsed", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "job_id", event.JobID, "batch_id", event.BatchID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "status", event.Status)
	if strings.TrimSpace(event.ServerID) == "" && strings.TrimSpace(event.NodeID) != "" {
		logger.Warn("job_result server_id fallback", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "job_id", event.JobID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "legacy_node_id_fallback")
	}

	if reason := validateJobResultForApply(event); reason != "" {
		logger.Warn("job_result ignored", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "job_id", event.JobID, "batch_id", event.BatchID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "status", event.Status, "reason", reason)
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "job_id", event.JobID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", reason)
		return rabbitmq.Ack
	}
	if handler == nil {
		logger.Warn("job_result ignored", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "job_id", event.JobID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "handler_is_nil")
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "job_id", event.JobID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "handler_is_nil")
		return rabbitmq.Ack
	}

	if err := handler(event); err != nil {
		logger.Error("rabbit message handler failed", err, "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "job_id", event.JobID, "batch_id", event.BatchID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID)
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "job_id", event.JobID, "batch_id", event.BatchID, "reason", "legacy_ack_after_handler_error")
		return rabbitmq.Ack
	}

	logger.Info("rabbit message handler succeeded", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "job_id", event.JobID, "batch_id", event.BatchID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID)
	logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "job_id", event.JobID, "batch_id", event.BatchID, "reason", "handler_success")
	return rabbitmq.Ack
}

func handleResultQueueNodeSnapshot(queue string, body []byte, handler NodeSnapshotHandler) rabbitmq.Action {
	var event NodeSnapshotEvent
	if err := json.Unmarshal(body, &event); err != nil {
		logger.Error("rabbit message invalid", err, "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "node_snapshot", "payload_size", len(body))
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "node_snapshot", "reason", "invalid_payload")
		return rabbitmq.Ack
	}
	logger.Info("rabbit message parsed", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
	if handler == nil {
		logger.Warn("node_snapshot ignored", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "handler_is_nil")
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "handler_is_nil")
		return rabbitmq.Ack
	}
	stale, err := handler(event)
	if err != nil {
		logger.Error("rabbit message handler failed", err, "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol)
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "node_snapshot_in_result_queue_handler_failed")
		return rabbitmq.Ack
	}
	reason := "node_snapshot_in_result_queue"
	if stale {
		reason = "stale_snapshot"
	}
	logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", reason)
	return rabbitmq.Ack
}

func validateJobResultForApply(event JobResultEvent) string {
	if event.EventType != "job_result" {
		return "invalid_job_result_event_type"
	}
	if event.JobID == 0 {
		return "invalid_job_result_missing_job_id"
	}
	if strings.TrimSpace(event.Status) == "" {
		return "invalid_job_result_missing_status"
	}
	if strings.TrimSpace(event.EffectiveServerID()) == "" {
		return "invalid_job_result_missing_server_id"
	}
	return ""
}

type eventEnvelope struct {
	EventType string `json:"event_type"`
}

func parseEventEnvelope(queue string, body []byte, source string) (eventEnvelope, bool) {
	var envelope eventEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		logger.Error("rabbit message invalid", err, "component", "rabbitmq", "operation", "consume", "queue", queue, "source", source, "payload_size", len(body))
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "source", source, "reason", "invalid_envelope")
		return envelope, false
	}
	envelope.EventType = strings.TrimSpace(envelope.EventType)
	if envelope.EventType == "" {
		logger.Warn("rabbit message ignored", "component", "rabbitmq", "operation", "consume", "queue", queue, "source", source, "reason", "missing_event_type")
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "source", source, "reason", "missing_event_type")
		return envelope, false
	}
	logger.Info("rabbit message routed", "component", "rabbitmq", "operation", "consume", "queue", queue, "source", source, "event_type", envelope.EventType, "payload_size", len(body))
	return envelope, true
}

func (p *Producer) StartAgentEventConsumer(exchange, queue, routingKey string, snapshotHandler func(NodeSnapshotEvent) (bool, error), jobResultHandler func(JobResultEvent) error) (*rabbitmq.Consumer, error) {
	if p == nil || p.conn == nil {
		return nil, fmt.Errorf("rabbitmq connection is not initialized")
	}
	if exchange == "" {
		return nil, fmt.Errorf("events exchange is required")
	}
	if queue == "" {
		return nil, fmt.Errorf("events queue is required")
	}
	if routingKey == "" {
		routingKey = "node.snapshot"
	}

	consumer, err := rabbitmq.NewConsumer(
		p.conn,
		queue,
		rabbitmq.WithConsumerOptionsExchangeName(exchange),
		rabbitmq.WithConsumerOptionsExchangeKind("topic"),
		rabbitmq.WithConsumerOptionsExchangeDeclare,
		rabbitmq.WithConsumerOptionsExchangeDurable,
		rabbitmq.WithConsumerOptionsRoutingKey(routingKey),
		rabbitmq.WithConsumerOptionsQueueDurable,
		rabbitmq.WithConsumerOptionsConcurrency(1),
		rabbitmq.WithConsumerOptionsQOSPrefetch(10),
		rabbitmq.WithConsumerOptionsLogging,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent events consumer: %w", err)
	}

	logger.Info("rabbit events consumer started", "exchange", exchange, "queue", queue, "routing_key", routingKey)

	go func() {
		err := consumer.Run(func(d rabbitmq.Delivery) rabbitmq.Action {
			return handleAgentEventQueueMessage(exchange, queue, routingKey, d.Body, snapshotHandler)
		})
		if err != nil {
			logger.Error("agent events consumer stopped", err, "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey)
		}
	}()

	return consumer, nil
}

func handleAgentEventQueueMessage(exchange string, queue string, routingKey string, body []byte, snapshotHandler NodeSnapshotHandler) rabbitmq.Action {
	logger.Info("rabbit message received", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "payload_size", len(body))
	envelope, ok := parseEventEnvelope(queue, body, "agent_events_queue")
	if !ok {
		return rabbitmq.Ack
	}

	switch envelope.EventType {
	case "node_snapshot":
		var event NodeSnapshotEvent
		if err := json.Unmarshal(body, &event); err != nil {
			logger.Error("rabbit message invalid", err, "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "payload_size", len(body))
			logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "reason", "invalid_payload")
			return rabbitmq.Ack
		}
		logger.Info("rabbit message parsed", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
		if snapshotHandler == nil {
			logger.Warn("node_snapshot ignored", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "handler_is_nil")
			logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "handler_is_nil")
			return rabbitmq.Ack
		}
		stale, err := snapshotHandler(event)
		if err != nil {
			logger.Error("rabbit message handler failed", err, "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
			logger.Info("rabbit message nack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "handler_failed")
			return rabbitmq.NackRequeue
		}
		reason := "handler_success"
		if stale {
			reason = "stale_snapshot"
			logger.Info("panel node snapshot ignored", "component", "rabbitmq", "operation", "consume", "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt, "reason", reason)
		} else {
			logger.Info("rabbit message handler succeeded", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
		}
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", reason)
		return rabbitmq.Ack
	case "job_result":
		var event JobResultEvent
		if err := json.Unmarshal(body, &event); err != nil {
			logger.Error("rabbit message invalid", err, "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "payload_size", len(body))
			logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "reason", "invalid_payload")
			return rabbitmq.Ack
		}
		logger.Warn("panel job_result ignored", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "job_id", event.JobID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "job_result_not_expected_here")
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "job_id", event.JobID, "profile_id", event.ProfileID, "server_id", event.EffectiveServerID(), "legacy_node_id", event.NodeID, "reason", "job_result_not_expected_here")
		return rabbitmq.Ack
	default:
		logger.Info("panel agent event ignored", "component", "rabbitmq", "operation", "consume", "event_type", envelope.EventType, "reason", "unsupported_event_type")
		logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "reason", "unsupported_event_type")
		return rabbitmq.Ack
	}
}

func (p *Producer) publish(pub *rabbitmq.Publisher, exchange string, msg any) error {
	return p.publishWithRoutingKey(pub, exchange, "", msg)
}

func (p *Producer) publishWithRoutingKey(pub *rabbitmq.Publisher, exchange, routingKey string, msg any) error {
	if p == nil || pub == nil {
		return fmt.Errorf("rabbitmq publisher is not initialized")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	fields := publishLogFields(exchange, routingKey, len(data), msg)
	logger.Info("rabbit publish started", fields...)
	err = pub.Publish(
		data,
		[]string{routingKey},
		rabbitmq.WithPublishOptionsContentType("application/json"),
		rabbitmq.WithPublishOptionsExchange(exchange),
	)
	if err != nil {
		logger.Error("rabbit publish failed", err, fields...)
		return err
	}
	logger.Info("rabbit publish succeeded", fields...)
	return nil
}

func publishLogFields(exchange string, routingKey string, payloadSize int, msg any) []any {
	fields := []any{"component", "rabbitmq", "operation", "publish", "exchange", exchange, "routing_key", routingKey, "payload_size", payloadSize}
	switch m := msg.(type) {
	case JobTask:
		eventType := m.EventType
		if eventType == "" {
			eventType = m.Action
		}
		fields = append(fields, "event_type", eventType, "job_id", m.JobID, "batch_id", m.BatchID, "profile_id", m.ProfileID, "server_id", effectiveJobTaskServerID(m), "profile", m.Profile, "target_group", m.TargetGroup, "protocol", m.Protocol, "client_code", m.ClientCode)
	case CollectSnapshotCommand:
		fields = append(fields, "event_type", m.EventType, "command_id", m.CommandID, "server_id", effectiveCollectSnapshotServerID(m), "target_group", m.TargetGroup, "requested_by", m.RequestedBy)
	case CreateUserTask:
		fields = append(fields, "event_type", "create_user_legacy", "tg_id", m.UserID, "username", m.Username)
	default:
		fields = append(fields, "event_type", fmt.Sprintf("%T", msg))
	}
	return fields
}

func (p *Producer) Close() {
	if p.publisherComplaints != nil {
		p.publisherComplaints.Close()
	}
	if p.publisherUsers != nil {
		p.publisherUsers.Close()
	}
	if p.publisherJobs != nil {
		p.publisherJobs.Close()
	}
	if p.publisherCommands != nil {
		p.publisherCommands.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

func loadRootCAs(cafile string) (*x509.CertPool, error) {
	rootCAs := x509.NewCertPool()

	caCert, err := os.ReadFile(cafile)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	if !rootCAs.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate to pool")
	}

	return rootCAs, nil
}

func (p *Producer) IsReady() bool {
	return p != nil && p.conn != nil && p.publisherComplaints != nil && p.publisherUsers != nil && p.publisherJobs != nil && p.publisherCommands != nil
}

func IsReady() bool {
	return GlobalProducer != nil && GlobalProducer.IsReady()
}
