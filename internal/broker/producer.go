package broker

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
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

	exchangeJobs := "vpn.jobs"
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

	logger.Info("rabbit command publisher ready", "exchange_jobs", exchangeJobs, "exchange_commands", exchangeCommands)
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
		routingKey = "create." + msg.TargetGroup
		logger.Info("create_client command routing resolved", "component", "rabbitmq", "operation", "publish", "exchange", p.exchangeJobs, "routing_key", routingKey, "job_id", msg.JobID, "batch_id", msg.BatchID, "profile_id", msg.ProfileID, "profile", msg.Profile, "target_group", msg.TargetGroup, "protocol", msg.Protocol, "client_code", msg.ClientCode)
	}
	return p.publishWithRoutingKey(p.publisherJobs, p.exchangeJobs, routingKey, msg)
}

func (p *Producer) PublishCollectSnapshotCommand(msg CollectSnapshotCommand) error {
	routingKey := "collect.node." + msg.TargetNodeID
	if msg.TargetNodeID == "" && msg.TargetGroup != "" {
		routingKey = "collect.group." + msg.TargetGroup
	}
	if msg.TargetNodeID == "" && msg.TargetGroup == "" {
		routingKey = "collect.group.all"
	}
	logger.Info("collect_snapshot command routing resolved", "component", "rabbitmq", "operation", "publish", "exchange", p.exchangeCommands, "routing_key", routingKey, "command_id", msg.CommandID, "target_node_id", msg.TargetNodeID, "target_group", msg.TargetGroup, "requested_by", msg.RequestedBy)
	return p.publishWithRoutingKey(p.publisherCommands, p.exchangeCommands, routingKey, msg)
}

func (p *Producer) StartResultConsumer(queue string, handler func(JobResultEvent) error) (*rabbitmq.Consumer, error) {
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
			logger.Info("rabbit message received", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "payload_size", len(d.Body))
			var event JobResultEvent
			if err := json.Unmarshal(d.Body, &event); err != nil {
				logger.Error("rabbit message invalid", err, "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "payload_size", len(d.Body))
				logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "reason", "invalid_payload")
				return rabbitmq.Ack
			}
			logger.Info("rabbit message parsed", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "job_id", event.JobID, "batch_id", event.BatchID, "profile_id", event.ProfileID, "node_id", event.NodeID)

			if err := handler(event); err != nil {
				logger.Error("rabbit message handler failed", err, "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "job_id", event.JobID, "batch_id", event.BatchID, "profile_id", event.ProfileID, "node_id", event.NodeID)
				logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "job_id", event.JobID, "batch_id", event.BatchID, "reason", "legacy_ack_after_handler_error")
				return rabbitmq.Ack
			}

			logger.Info("rabbit message handler succeeded", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "job_id", event.JobID, "batch_id", event.BatchID, "profile_id", event.ProfileID, "node_id", event.NodeID)
			logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result", "job_id", event.JobID, "batch_id", event.BatchID, "reason", "handler_success")
			return rabbitmq.Ack
		})
		if err != nil {
			logger.Error("job result consumer stopped", err, "component", "rabbitmq", "operation", "consume", "queue", queue, "event_type", "job_result")
		}
	}()

	return consumer, nil
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
			logger.Info("rabbit message received", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "payload_size", len(d.Body))
			var envelope struct {
				EventType string `json:"event_type"`
			}
			if err := json.Unmarshal(d.Body, &envelope); err != nil {
				logger.Error("rabbit message invalid", err, "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "payload_size", len(d.Body))
				logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "reason", "invalid_envelope")
				return rabbitmq.Ack
			}
			logger.Info("rabbit message routed", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "payload_size", len(d.Body))

			switch envelope.EventType {
			case "node_snapshot":
				var event NodeSnapshotEvent
				if err := json.Unmarshal(d.Body, &event); err != nil {
					logger.Error("rabbit message invalid", err, "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "payload_size", len(d.Body))
					logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "reason", "invalid_payload")
					return rabbitmq.Ack
				}

				logger.Info("rabbit message parsed", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
				stale, err := snapshotHandler(event)
				if err != nil {
					logger.Error("rabbit message handler failed", err, "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
					logger.Info("rabbit message nack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "node_id", event.NodeID, "reason", "handler_failed")
					return rabbitmq.NackRequeue
				}
				if stale {
					logger.Info("panel node snapshot ignored", "component", "rabbitmq", "operation", "consume", "event_type", event.EventType, "node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt, "reason", "stale_snapshot")
					logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "node_id", event.NodeID, "reason", "stale_snapshot")
					return rabbitmq.Ack
				}

				logger.Info("rabbit message handler succeeded", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
				logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", event.EventType, "node_id", event.NodeID, "reason", "handler_success")
				return rabbitmq.Ack
			case "job_result":
				if jobResultHandler == nil {
					logger.Info("panel job_result ignored", "component", "rabbitmq", "operation", "consume", "event_type", envelope.EventType, "reason", "handler_is_nil")
					logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "reason", "handler_is_nil")
					return rabbitmq.Ack
				}
				var event JobResultEvent
				if err := json.Unmarshal(d.Body, &event); err != nil {
					logger.Error("rabbit message invalid", err, "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "payload_size", len(d.Body))
					logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "reason", "invalid_payload")
					return rabbitmq.Ack
				}
				logger.Info("rabbit message parsed", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "job_id", event.JobID, "batch_id", event.BatchID, "profile_id", event.ProfileID, "node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
				if err := jobResultHandler(event); err != nil {
					logger.Error("rabbit message handler failed", err, "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "job_id", event.JobID, "profile_id", event.ProfileID, "node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
					logger.Info("rabbit message nack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "job_id", event.JobID, "profile_id", event.ProfileID, "node_id", event.NodeID, "reason", "handler_failed")
					return rabbitmq.NackRequeue
				}
				logger.Info("rabbit message handler succeeded", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "job_id", event.JobID, "profile_id", event.ProfileID, "node_id", event.NodeID, "profile", event.Profile, "target_group", event.TargetGroup, "protocol", event.Protocol, "status", event.Status, "client_code", event.ClientCode)
				logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "job_id", event.JobID, "profile_id", event.ProfileID, "node_id", event.NodeID, "reason", "handler_success")
				return rabbitmq.Ack
			default:
				logger.Info("panel agent event ignored", "component", "rabbitmq", "operation", "consume", "event_type", envelope.EventType, "reason", "unsupported_event_type")
				logger.Info("rabbit message ack", "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey, "event_type", envelope.EventType, "reason", "unsupported_event_type")
				return rabbitmq.Ack
			}
		})
		if err != nil {
			logger.Error("agent events consumer stopped", err, "component", "rabbitmq", "operation", "consume", "exchange", exchange, "queue", queue, "routing_key", routingKey)
		}
	}()

	return consumer, nil
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
		fields = append(fields, "event_type", eventType, "job_id", m.JobID, "batch_id", m.BatchID, "profile_id", m.ProfileID, "profile", m.Profile, "target_group", m.TargetGroup, "protocol", m.Protocol, "client_code", m.ClientCode)
	case CollectSnapshotCommand:
		fields = append(fields, "event_type", m.EventType, "command_id", m.CommandID, "target_node_id", m.TargetNodeID, "target_group", m.TargetGroup, "requested_by", m.RequestedBy)
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
