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

	exchangeComplaints string
	exchangeUsers      string
	exchangeJobs       string
}

func NewProducer(url, exchangeComplaints, exchangeUsers, certfile, keyfile, cafile string) (*Producer, error) {
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
		rabbitmq.WithPublisherOptionsExchangeKind("fanout"),
		rabbitmq.WithPublisherOptionsExchangeDeclare,
		rabbitmq.WithPublisherOptionsLogging,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher for jobs: %w", err)
	}

	return &Producer{
		conn:                conn,
		publisherComplaints: publisherComplaints,
		publisherUsers:      publisherUsers,
		publisherJobs:       publisherJobs,
		exchangeComplaints:  exchangeComplaints,
		exchangeUsers:       exchangeUsers,
		exchangeJobs:        exchangeJobs,
	}, nil
}

func (p *Producer) PublishComplaintReply(msg any) error {
	return p.publish(p.publisherComplaints, p.exchangeComplaints, msg)
}

func (p *Producer) PublishCreateUser(msg any) error {
	return p.publish(p.publisherUsers, p.exchangeUsers, msg)
}

func (p *Producer) PublishJob(msg JobTask) error {
	return p.publish(p.publisherJobs, p.exchangeJobs, msg)
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

	go func() {
		err := consumer.Run(func(d rabbitmq.Delivery) rabbitmq.Action {
			var event JobResultEvent
			if err := json.Unmarshal(d.Body, &event); err != nil {
				logger.Printf("invalid job result event: %v", err)
				return rabbitmq.Ack
			}

			if err := handler(event); err != nil {
				logger.Printf("failed to apply job result event job_id=%d batch_id=%d: %v", event.JobID, event.BatchID, err)
				return rabbitmq.Ack
			}

			return rabbitmq.Ack
		})
		if err != nil {
			logger.Printf("job result consumer stopped: %v", err)
		}
	}()

	return consumer, nil
}

func (p *Producer) StartAgentEventConsumer(exchange, queue, routingKey string, snapshotHandler func(NodeSnapshotEvent) (bool, error)) (*rabbitmq.Consumer, error) {
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

	go func() {
		err := consumer.Run(func(d rabbitmq.Delivery) rabbitmq.Action {
			var envelope struct {
				EventType string `json:"event_type"`
			}
			if err := json.Unmarshal(d.Body, &envelope); err != nil {
				logger.Error("panel node snapshot invalid", err)
				return rabbitmq.Ack
			}
			if envelope.EventType != "node_snapshot" {
				logger.Info("panel agent event ignored", "event_type", envelope.EventType)
				return rabbitmq.Ack
			}

			var event NodeSnapshotEvent
			if err := json.Unmarshal(d.Body, &event); err != nil {
				logger.Error("panel node snapshot invalid", err, "event_type", envelope.EventType)
				return rabbitmq.Ack
			}

			logger.Info("panel node snapshot received", "event_type", event.EventType, "node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
			stale, err := snapshotHandler(event)
			if err != nil {
				logger.Error("panel node snapshot apply failed", err, "event_type", event.EventType, "node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
				return rabbitmq.NackRequeue
			}
			if stale {
				logger.Info("panel node snapshot ignored as stale", "event_type", event.EventType, "node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
				return rabbitmq.Ack
			}

			logger.Info("panel node snapshot applied", "event_type", event.EventType, "node_id", event.NodeID, "endpoint_group", event.EndpointGroup, "protocol", event.Protocol, "clients_count", event.ClientsCount, "online_count", event.OnlineCount, "xui_available", event.XUIAvailable, "sent_at", event.SentAt)
			return rabbitmq.Ack
		})
		if err != nil {
			logger.Error("agent events consumer stopped", err)
		}
	}()

	return consumer, nil
}

func (p *Producer) publish(pub *rabbitmq.Publisher, exchange string, msg any) error {
	if p == nil || pub == nil {
		return fmt.Errorf("rabbitmq publisher is not initialized")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	return pub.Publish(
		data,
		[]string{""},
		rabbitmq.WithPublishOptionsContentType("application/json"),
		rabbitmq.WithPublishOptionsExchange(exchange),
	)
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
	return p != nil && p.conn != nil && p.publisherComplaints != nil && p.publisherUsers != nil && p.publisherJobs != nil
}

func IsReady() bool {
	return GlobalProducer != nil && GlobalProducer.IsReady()
}
