package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"vpnpanel/internal/app"
)

func main() {
	f, err := os.OpenFile("vpnpanel.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	multi := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multi)

	server := app.NewServer()
	server.CronStart()
	server.Cron.Start()

	defer func() {
		server.Cron.Stop()
	}()

	log.Println("Server started on :8080")
	http.ListenAndServe("localhost:8080", server.Router)
}
