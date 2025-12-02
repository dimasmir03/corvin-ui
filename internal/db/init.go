package db

import (
	"fmt"
	"log"
	"vpnpanel/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

type DBOptions struct {
	Host    string
	Port    int
	User    string
	Pass    string
	DBName  string
	SSLMode string
}

func Init(options DBOptions) {
	if options.Host == "" || options.Port == 0 || options.User == "" || options.Pass == "" || options.DBName == "" {
		log.Fatal("database options are empty")
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		options.Host,
		options.Port,
		options.User,
		options.Pass,
		options.DBName,
		options.SSLMode,
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database:", err)
	}

	if err := migrate(); err != nil {
		log.Fatal("failed migration:", err)
	}
}

func migrate() error {
	return DB.AutoMigrate(
		&models.User{},
		&models.Server{},
		&models.ServerStat{},
		&models.Telegram{},
		&models.Vpn{},
		&models.Complaint{},
		&models.Settings{},
	)
}
