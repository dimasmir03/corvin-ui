package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"vpnpanel/internal/app"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/db"
	"vpnpanel/internal/repository"
)

func main() {
	f, err := os.OpenFile("vpnpanel.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	multi := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multi)

	db.Init()

	settingsRepo := repository.NewSettingsRepo(db.DB)

	// Инициализация дефолтных настроек
	if err := InitDefaultSettings(settingsRepo); err != nil {
		log.Fatalf("Failed to initialize default settings: %v", err)
	}
	// fmt.Println(os.Args)
	if len(os.Args) > 1 {
		runCLI(os.Args[1:])
		return
	}

	// var url = "amqps://ravenvpn:ravenvpn@localhost:5671/"
	// var exchange = "vpn.complaints"
	// var queue = "complaints.reply"
	// var certFile = "/etc/corvin/cert.pem"
	// var keyFile = "/etc/corvin/key.pem"
	// var caFile = "/etc/corvin/ca.pem"

	keys := []string{"amqp_url", "amqp_exchange", "amqp_queue", "cert_file", "key_file", "ca_file"}
	values, err := settingsRepo.GetKeys(keys...)
	if err != nil {
		log.Fatalf("Failed to get settings: %v", err)
	}
	// p, err := broker.NewProducer(url, exchange, queue, certFile, keyFile, caFile)
	p, err := broker.NewProducer(
		values["amqp_url"],
		values["amqp_exchange"],
		values["amqp_queue"],
		values["cert_file"],
		values["key_file"],
		values["ca_file"],
	)

	if err != nil {
		log.Fatalf("Failed to init RabbitMQ producer: %v", err)
	}

	broker.GlobalProducer = p

	server := app.NewServer()
	server.CronStart()
	server.Cron.Start()

	defer func() {
		server.Cron.Stop()
	}()

	log.Println("Server started on :8080")
	http.ListenAndServe("localhost:8080", server.Router)
}

func InitDefaultSettings(repo *repository.SettingsRepo) error {
	defaults := map[string]string{
		"amqp_url":      "amqps://ravenvpn:ravenvpn@localhost:5671/",
		"amqp_exchange": "vpn.complaints",
		"amqp_queue":    "complaints.reply",
		"cert_file":     "/etc/corvin/cert.pem",
		"key_file":      "/etc/corvin/key.pem",
		"ca_file":       "/etc/corvin/ca.pem",
	}

	for key, value := range defaults {
		_, err := repo.GetByKey(key)
		if err != nil {
			// ключа нет — создаем
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
	// case "global":
	// 	handleGlobalCLI(args[1:])
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

		err := repo.UpdateSettings(updates)
		if err != nil {
			fmt.Printf("Failed to update settings: %v\n", err)
			return
		}
		fmt.Println("Settings updated successfully")

	default:
		fmt.Println("Unknown settings CLI command. Available commands: show, update")
	}
}

func ShowSettings(repo *repository.SettingsRepo) {
	s, err := repo.GetAll()
	if err != nil {
		fmt.Printf("Failed to get settings: %v\n", err)
		return
	}

	fmt.Println("Panel Settings:")
	for _, value := range s {
		fmt.Printf("%s: %s\n", value.Key, value.Value)
	}

	// fmt.Printf("  Listen: %s\n", s.Listen)
	// fmt.Printf("  Port: %d\n", s.Port)
	// fmt.Printf("  Telegram Token: %s\n", s.TelegramToken)
	// fmt.Printf("  Other fields: ...\n") // по необходимости

}
