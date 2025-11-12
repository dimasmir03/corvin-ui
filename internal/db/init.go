package db

import (
	"log"
	"vpnpanel/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() {
	var err error
	DB, err = gorm.Open(sqlite.Open("data.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database:", err)
	}

	err = DB.AutoMigrate(
		&models.Server{},
		&models.User{},
		&models.UserServer{},
		&models.ServerStat{},
	)
	if err != nil {
		log.Fatal("failed migration:", err)
	} 
}
