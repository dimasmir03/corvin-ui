package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultEnvPath = ".env"

type Config struct {
	App   AppConfig
	DB    DBConfig
	AMQP  AMQPConfig
	MinIO MinIOConfig
	Certs CertConfig
}

type AppConfig struct {
	Addr          string
	SessionSecret string
}

type DBConfig struct {
	Host    string
	Port    int
	User    string
	Pass    string
	Name    string
	SSLMode string
}

type AMQPConfig struct {
	URL                string
	ExchangeComplaints string
	ExchangeUsers      string
}

type MinIOConfig struct {
	AccessKey string
	SecretKey string
	Bucket    string
	Endpoint  string
	SSL       string
	Region    string
}

type CertConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

func Load() (Config, error) {
	return LoadFile(defaultEnvPath)
}

func LoadFile(path string) (Config, error) {
	if err := loadDotEnv(path); err != nil {
		return Config{}, err
	}

	return Config{
		App: AppConfig{
			Addr:          getEnv("APP_ADDR", ":8080"),
			SessionSecret: getEnv("SESSION_SECRET", "change-me"),
		},
		DB: DBConfig{
			Host:    getEnv("DB_HOST", "localhost"),
			Port:    getEnvInt("DB_PORT", 5432),
			User:    getEnv("DB_USER", "corvinvpn"),
			Pass:    getEnv("DB_PASS", "corvinvpn"),
			Name:    getEnv("DB_NAME", "corvinvpn"),
			SSLMode: getEnv("DB_SSL_MODE", "disable"),
		},
		AMQP: AMQPConfig{
			URL:                getEnv("AMQP_URL", "amqps://corvinvpn:corvinvpn@localhost:5671/"),
			ExchangeComplaints: getEnv("AMQP_EXCHANGE_COMPLAINTS", "vpn.complaints"),
			ExchangeUsers:      getEnv("AMQP_EXCHANGE_USERS", "vpn.users"),
		},
		MinIO: MinIOConfig{
			AccessKey: getEnv("MINIO_ACCESS_KEY", "corvinvpn"),
			SecretKey: getEnv("MINIO_SECRET_KEY", "corvinvpn"),
			Bucket:    getEnv("MINIO_BUCKET", "vpn"),
			Endpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
			SSL:       getEnv("MINIO_SSL", "true"),
			Region:    getEnv("MINIO_REGION", "us-east-1"),
		},
		Certs: CertConfig{
			CertFile: getEnv("CERT_FILE", "/opt/corvin-ui/cert/cert.pem"),
			KeyFile:  getEnv("KEY_FILE", "/opt/corvin-ui/cert/key.pem"),
			CAFile:   getEnv("CA_FILE", "/opt/corvin-ui/cert/ca.pem"),
		},
	}, nil
}

func (c Config) DefaultSettings() map[string]string {
	return map[string]string{
		"amqp_url":                 c.AMQP.URL,
		"amqp_exchange_complaints": c.AMQP.ExchangeComplaints,
		"amqp_exchange_users":      c.AMQP.ExchangeUsers,
		"cert_file":                c.Certs.CertFile,
		"key_file":                 c.Certs.KeyFile,
		"ca_file":                  c.Certs.CAFile,
		"minio_access_key":         c.MinIO.AccessKey,
		"minio_secret_key":         c.MinIO.SecretKey,
		"minio_bucket":             c.MinIO.Bucket,
		"minio_endpoint":           c.MinIO.Endpoint,
		"minio_ssl":                c.MinIO.SSL,
		"minio_region":             c.MinIO.Region,
		"db_host":                  c.DB.Host,
		"db_port":                  strconv.Itoa(c.DB.Port),
		"db_user":                  c.DB.User,
		"db_pass":                  c.DB.Pass,
		"db_name":                  c.DB.Name,
		"db_ssl_mode":              c.DB.SSLMode,
	}
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		value = strings.TrimSpace(stripInlineComment(value))
		value = strings.Trim(value, `"'`)

		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set %s: %w", key, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	return nil
}

func stripInlineComment(value string) string {
	inSingleQuote := false
	inDoubleQuote := false

	for i, r := range value {
		switch r {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case '#':
			if !inSingleQuote && !inDoubleQuote {
				return value[:i]
			}
		}
	}

	return value
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
