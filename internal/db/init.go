package db

import (
	"log"
	"runtime"
	"vpnpanel/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() {
	var err error

	DB, err = gorm.Open(sqlite.Open(initdbpath() + "data.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database:", err)
	}

	err = DB.AutoMigrate(
		&models.User{},
		&models.Server{},
		&models.ServerStat{},
		&models.TelegramInfo{},
		&models.VpnInfo{},
	)
	if err != nil {
		log.Fatal("failed migration:", err)
	}
}

func initdbpath() string {
	if runtime.GOOS == "windows" {
		return "./"
	}
	return "/etc/corvin-ui/"
}
