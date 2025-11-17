package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"vpnpanel/internal/app"
	"vpnpanel/internal/broker"
)

func main() {
	f, err := os.OpenFile("vpnpanel.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	multi := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multi)

	
	var url = "amqps://ravenvpn:ravenvpn@localhost:5671/"
	var exchange = "vpn.complaints"
	var queue = "complaints.reply"
	var certFile = "/etc/corvin/cert.pem"
	var keyFile = "/etc/corvin/key.pem"
	var caFile = "/etc/corvin/ca.pem"

	p, err := broker.NewProducer(url, exchange, queue, certFile, keyFile, caFile)

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
