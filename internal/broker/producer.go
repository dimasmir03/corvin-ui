package broker

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"

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
