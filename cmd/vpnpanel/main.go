package main

import (
	"log"
	"net/http"

	"vpnpanel/internal/app"
	"vpnpanel/internal/db"
)

func main() {
	db.Init()

	r := app.Routes()

	log.Println("Server started on :8080")
	http.ListenAndServe("localhost:8080", r)
}
