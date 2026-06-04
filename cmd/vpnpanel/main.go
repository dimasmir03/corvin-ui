package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"

	"vpnpanel/internal/app"
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
	initRabbitMQ(settingsRepo)

	// Server init
	server := app.NewServer(cfg)
	server.CronStart()

	defer server.Cron.Stop()

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

func initRabbitMQ(settings *repository.SettingsRepo) {
	keys := []string{
		"amqp_url",
		"amqp_exchange_complaints",
		"amqp_exchange_users",
		"amqp_queue",
		"cert_file",
		"key_file",
		"ca_file",
	}

	values, err := settings.GetKeys(keys...)
	if err != nil {
		log.Fatalf("Failed to get settings: %v", err)
	}
	p, err := broker.NewProducer(
		values["amqp_url"],
		values["amqp_exchange_complaints"],
		values["amqp_exchange_users"],
		values["cert_file"],
		values["key_file"],
		values["ca_file"],
	)

	if err != nil && runtime.GOOS != "windows" {
		log.Fatalf("Failed to init RabbitMQ producer: %v", err)
	}

	broker.GlobalProducer = p
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
