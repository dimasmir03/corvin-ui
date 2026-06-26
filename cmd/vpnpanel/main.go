package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"

	"vpnpanel/internal/app"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/config"
	"vpnpanel/internal/db"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/repository"
)

func main() {
	mw := initLogger()

	logger.Info("app starting", "component", "startup", "operation", "app_start")
	logger.Info("config loading started", "component", "startup", "operation", "config_load")
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}
	logger.Info("startup config loaded", "component", "startup", "operation", "config_load", "http_addr", cfg.HTTP.Addr, "db_host", cfg.DB.Host, "db_port", cfg.DB.Port, "db_name", cfg.DB.Name, "rabbit_host", rabbitHostForLog(cfg.RabbitMQ.URL), "telegram_enabled", cfg.Telegram.Enabled, "telegram_proxy_enabled", cfg.Telegram.ProxyURL != "")

	dbOptions := db.DBOptions{
		Host:    cfg.DB.Host,
		Port:    cfg.DB.Port,
		User:    cfg.DB.User,
		Pass:    cfg.DB.Password,
		DBName:  cfg.DB.Name,
		SSLMode: cfg.DB.SSLMode,
	}

	db.Init(dbOptions, mw)
	logger.Info("database initialized", "component", "startup", "operation", "db_init", "db_host", cfg.DB.Host, "db_port", cfg.DB.Port, "db_name", cfg.DB.Name)

	settingsRepo := repository.NewSettingsRepo(db.DB)

	if err := InitDefaultSettings(settingsRepo, cfg); err != nil {
		logger.Fatalf("Failed to initialize default settings: %v", err)
	}

	// CLI mode
	if len(os.Args) > 1 {
		runCLI(os.Args[1:])
		return
	}

	// RabbitMQ init
	initRabbitMQ(cfg, settingsRepo)

	logger.Info("rabbitmq initialization completed", "component", "startup", "operation", "rabbitmq_init", "rabbit_host", rabbitHostForLog(cfg.RabbitMQ.URL), "producer_ready", broker.GlobalProducer != nil && broker.GlobalProducer.IsReady())

	logger.Info("application server init started", "component", "startup", "operation", "server_init", "telegram_enabled", cfg.Telegram.Enabled)
	// Server init
	server, err := app.NewServer(cfg)
	if err != nil {
		logger.Fatalf("Failed to initialize server: %v", err)
	}
	server.CronStart()
	logger.Info("background workers started", "component", "startup", "operation", "background_workers")

	defer server.Close()

	logger.Info("http server starting", "component", "startup", "operation", "http_start", "http_addr", cfg.HTTP.Addr)
	logger.Printf("Server Started on %s", cfg.HTTP.Addr)
	if err := http.ListenAndServe(cfg.HTTP.Addr, server.Router); err != nil {
		logger.Fatalf("HTTP server error: %v", err)
	}
}

func initLogger() io.Writer {
	path := initLogPath()

	f, err := os.OpenFile(
		path+"vpnpanel.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		logger.Fatal(err)
	}

	mw := io.MultiWriter(os.Stdout, f)
	logger.Configure(mw, "info", "text")

	return mw
}

func initLogPath() string {
	if runtime.GOOS == "windows" {
		return "./"
	}
	return "/var/log/corvin-ui/"
}

func initRabbitMQ(cfg config.Config, settings *repository.SettingsRepo) {
	keys := []string{
		"amqp_url",
		"amqp_exchange_complaints",
		"amqp_exchange_users",
		"amqp_exchange_commands",
		"cert_file",
		"key_file",
		"ca_file",
	}

	values, err := settings.GetKeys(keys...)
	if err != nil {
		logger.Fatalf("Failed to get settings: %v", err)
	}

	amqpURL, amqpSource := envFirst(cfg.RabbitMQ.URL, values["amqp_url"])
	certFile := envSettingsFallback("CERT_FILE", cfg.Defaults.CertFile, values["cert_file"])
	keyFile := envSettingsFallback("KEY_FILE", cfg.Defaults.KeyFile, values["key_file"])
	caFile := envSettingsFallback("CA_FILE", cfg.Defaults.CAFile, values["ca_file"])

	logAMQPConfig(amqpSource, amqpURL)

	p, err := broker.NewProducer(
		amqpURL,
		fallback(values["amqp_exchange_complaints"], cfg.Defaults.AMQPExchangeComplaints),
		fallback(values["amqp_exchange_users"], cfg.Defaults.AMQPExchangeUsers),
		fallback(values["amqp_exchange_commands"], cfg.Defaults.AMQPExchangeCommands),
		certFile,
		keyFile,
		caFile,
	)

	if err != nil && runtime.GOOS != "windows" {
		logger.Fatalf("Failed to init RabbitMQ producer: %v", err)
	}

	broker.GlobalProducer = p
}

func envFirst(envValue, settingsValue string) (string, string) {
	if envValue != "" {
		return envValue, "env"
	}
	return settingsValue, "settings"
}

func fallback(value, fallbackValue string) string {
	if value != "" {
		return value
	}
	return fallbackValue
}

func envSettingsFallback(envKey, defaultValue, settingsValue string) string {
	if value, ok := os.LookupEnv(envKey); ok && value != "" {
		return value
	}
	if settingsValue != "" {
		return settingsValue
	}
	return defaultValue
}

func rabbitHostForLog(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func logAMQPConfig(source, rawURL string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		logger.Printf("AMQP config source=%s url_user= host= port= password_len=0 parse_error=%v", source, err)
		return
	}

	passwordLen := 0
	if password, ok := parsed.User.Password(); ok {
		passwordLen = len(password)
	}

	logger.Printf(
		"AMQP config source=%s url_user=%s host=%s port=%s password_len=%d",
		source,
		parsed.User.Username(),
		parsed.Hostname(),
		parsed.Port(),
		passwordLen,
	)
}

func InitDefaultSettings(repo *repository.SettingsRepo, cfg config.Config) error {
	defaults := cfg.DefaultSettings()
	for key, value := range defaults {
		_, err := repo.GetByKey(key)
		if err != nil {
			if err := repo.Set(key, value); err != nil {
				return err
			}
		}
	}

	return nil
}

func runCLI(args []string) {
	switch args[0] {
	case "settings":
		handleSettingCLI(args[1:])
	default:
		fmt.Println("Unknown CLI command")
	}
}

func handleSettingCLI(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: settings <list|show|update> [options]")
		return
	}

	repo := repository.NewSettingsRepo(db.DB)

	switch args[0] {
	case "show":
		ShowSettings(repo)
	case "update":
		if len(args) < 3 || len(args)%2 == 0 {
			fmt.Println("Usage: settings update <field> <value> [<field> <value>...]")
			return
		}

		updates := make(map[string]string)
		for i := 1; i < len(args); i += 2 {
			updates[args[i]] = args[i+1]
		}

		if err := repo.UpdateSettings(updates); err != nil {
			fmt.Printf("Failed to update settings: %v\n", err)
			return
		}

		auditLogger := audit.NewLogger(repository.NewAuditRepo(db.DB))
		_ = auditLogger.Log(audit.Event{
			ActorType:  audit.ActorAdmin,
			Action:     "settings.changed",
			EntityType: "settings",
			Status:     audit.StatusSuccess,
			Message:    "settings updated from CLI",
			NewValue:   updates,
		})

		fmt.Println("Settings updated successfully")

	default:
		fmt.Println("Unknown settings CLI command. Available commands: show, update")
	}
}

func ShowSettings(repo *repository.SettingsRepo) {
	settings, err := repo.GetAll()
	if err != nil {
		fmt.Printf("Failed to get settings: %v\n", err)
		return
	}

	fmt.Println("Panel Settings:")
	for _, value := range settings {
		fmt.Printf("%s: %s\n", value.Key, value.Value)
	}
}
