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
	publisher *rabbitmq.Publisher
	conn      *rabbitmq.Conn
	exchange  string
	queue     string
}

func NewProducer(url, exchange, queue, certfile, keyfile, cafile string) (*Producer, error) {
	// return &Producer{
	// 	queue:    queue,
	// 	exchange: exchange,
	// }, nil
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
		rabbitmq.WithConnectionOptionsConfig(
			rabbitmq.Config{TLSClientConfig: tlsConfig},
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	publisher, err := rabbitmq.NewPublisher(
		conn,
		rabbitmq.WithPublisherOptionsExchangeName(exchange),
		rabbitmq.WithPublisherOptionsExchangeKind("fanout"),
		rabbitmq.WithPublisherOptionsExchangeDeclare,
		rabbitmq.WithPublisherOptionsLogging,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher: %w", err)
	}

	return &Producer{
		conn:      conn,
		publisher: publisher,
		exchange:  exchange,
		queue:     queue,
	}, nil
}

func (p *Producer) Publish(msg any) error {
	// return nil
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	return p.publisher.Publish(
		data,
		[]string{p.queue},
		rabbitmq.WithPublishOptionsContentType("application/json"),
		rabbitmq.WithPublishOptionsExchange(p.exchange),
	)
}

func (p *Producer) Close() {
	// return
	if p.publisher != nil {
		p.publisher.Close()
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
