package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const AuthModeNone = "none"

type Config struct {
	HTTP                  HTTPConfig
	Auth                  AuthConfig
	DB                    DBConfig
	RabbitMQ              RabbitMQConfig
	Telegram              TelegramConfig
	MinIO                 MinIOConfig
	Session               SessionConfig
	Defaults              DefaultsConfig
	OnlineCollectInterval string
}

type HTTPConfig struct {
	Addr string
}

type AuthConfig struct {
	Mode string
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type RabbitMQConfig struct {
	URL         string
	ResultQueue string
}

type TelegramConfig struct {
	Enabled bool
	Token   string
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Region    string
	Bucket    string
}

type SessionConfig struct {
	Secret string
}

type DefaultsConfig struct {
	AMQPExchangeComplaints string
	AMQPExchangeUsers      string
	CertFile               string
	KeyFile                string
	CAFile                 string
}

func Load() (Config, error) {
	cfg := Config{
		HTTP: HTTPConfig{
			Addr: getEnv("HTTP_ADDR", "127.0.0.1:8080"),
		},
		Auth: AuthConfig{
			Mode: strings.ToLower(getEnv("AUTH_MODE", AuthModeNone)),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "127.0.0.1"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "corvinvpn"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "corvinvpn"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		RabbitMQ: RabbitMQConfig{
			URL:         getEnv("RABBITMQ_URL", ""),
			ResultQueue: getEnv("RABBITMQ_RESULT_QUEUE", "corvin.job.results"),
		},
		Telegram: TelegramConfig{
			Enabled: getEnvBool("TELEGRAM_ENABLED", false),
			Token:   getEnv("TELEGRAM_BOT_TOKEN", ""),
		},
		MinIO: MinIOConfig{
			Endpoint:  getEnv("MINIO_ENDPOINT", "127.0.0.1:9000"),
			AccessKey: getEnv("MINIO_ACCESS_KEY", "corvinvpn"),
			SecretKey: getEnv("MINIO_SECRET_KEY", ""),
			UseSSL:    getEnvBool("MINIO_USE_SSL", false),
			Region:    getEnv("MINIO_REGION", "us-east-1"),
			Bucket:    getEnv("MINIO_BUCKET", "complaints"),
		},
		Session: SessionConfig{
			Secret: getEnv("SESSION_SECRET", ""),
		},
		Defaults: DefaultsConfig{
			AMQPExchangeComplaints: getEnv("AMQP_EXCHANGE_COMPLAINTS", "vpn.complaints"),
			AMQPExchangeUsers:      getEnv("AMQP_EXCHANGE_USERS", "vpn.users"),
			CertFile:               getEnv("CERT_FILE", "/opt/corvin-ui/cert/cert.pem"),
			KeyFile:                getEnv("KEY_FILE", "/opt/corvin-ui/cert/key.pem"),
			CAFile:                 getEnv("CA_FILE", "/opt/corvin-ui/cert/ca.pem"),
		},
		OnlineCollectInterval: getEnv("ONLINE_COLLECT_INTERVAL", "30s"),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.HTTP.Addr == "" {
		return fmt.Errorf("HTTP_ADDR is required")
	}

	if c.Auth.Mode == AuthModeNone && !isLoopbackAddr(c.HTTP.Addr) {
		return fmt.Errorf("AUTH_MODE=none requires HTTP_ADDR to bind only 127.0.0.1 or localhost, got %q", c.HTTP.Addr)
	}

	if c.Auth.Mode != AuthModeNone && c.Session.Secret == "" {
		return fmt.Errorf("SESSION_SECRET is required when AUTH_MODE=%s", c.Auth.Mode)
	}

	if c.Telegram.Enabled && strings.TrimSpace(c.Telegram.Token) == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required when TELEGRAM_ENABLED=true")
	}

	return nil
}

func (c Config) DefaultSettings() map[string]string {
	return map[string]string{
		"amqp_exchange_complaints": c.Defaults.AMQPExchangeComplaints,
		"amqp_exchange_users":      c.Defaults.AMQPExchangeUsers,
		"minio_access_key":         c.MinIO.AccessKey,
		"minio_bucket":             c.MinIO.Bucket,
		"minio_endpoint":           c.MinIO.Endpoint,
		"minio_ssl":                strconv.FormatBool(c.MinIO.UseSSL),
		"minio_region":             c.MinIO.Region,
		"db_host":                  c.DB.Host,
		"db_port":                  strconv.Itoa(c.DB.Port),
		"db_user":                  c.DB.User,
		"db_name":                  c.DB.Name,
		"db_ssl_mode":              c.DB.SSLMode,
	}
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}
