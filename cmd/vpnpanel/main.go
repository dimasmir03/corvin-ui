package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"

	"vpnpanel/internal/app"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/config"
	"vpnpanel/internal/db"
	"vpnpanel/internal/repository"
)

func main() {
	mw := initLogger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbOptions := db.DBOptions{
		Host:    cfg.DB.Host,
		Port:    cfg.DB.Port,
		User:    cfg.DB.User,
		Pass:    cfg.DB.Password,
		DBName:  cfg.DB.Name,
		SSLMode: cfg.DB.SSLMode,
	}

	db.Init(dbOptions, mw)

	settingsRepo := repository.NewSettingsRepo(db.DB)

	if err := InitDefaultSettings(settingsRepo, cfg); err != nil {
		log.Fatalf("Failed to initialize default settings: %v", err)
	}

	// CLI mode
	if len(os.Args) > 1 {
		runCLI(os.Args[1:])
		return
	}

	// RabbitMQ init
	initRabbitMQ(cfg, settingsRepo)

	// Server init
	server := app.NewServer(cfg)
	server.CronStart()

	defer server.Close()

	log.Printf("Server Started on %s", cfg.HTTP.Addr)
	if err := http.ListenAndServe(cfg.HTTP.Addr, server.Router); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func initLogger() io.Writer {
	path := initLogPath()

	f, err := os.OpenFile(
		path+"vpnpanel.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

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
		"cert_file",
		"key_file",
		"ca_file",
	}

	values, err := settings.GetKeys(keys...)
	if err != nil {
		log.Fatalf("Failed to get settings: %v", err)
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
		certFile,
		keyFile,
		caFile,
	)

	if err != nil && runtime.GOOS != "windows" {
		log.Fatalf("Failed to init RabbitMQ producer: %v", err)
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

func logAMQPConfig(source, rawURL string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		log.Printf("AMQP config source=%s url_user= host= port= password_len=0 parse_error=%v", source, err)
		return
	}

	passwordLen := 0
	if password, ok := parsed.User.Password(); ok {
		passwordLen = len(password)
	}

	log.Printf(
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
