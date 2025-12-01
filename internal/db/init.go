package db

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"vpnpanel/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() {
	dbPath := resolveDBpath()

	if err := ensureDir(filepath.Dir(dbPath)); err != nil {
		log.Fatal("failed to create db dir:", err)
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database:", err)
	}

	if err := migrate();err != nil {
		log.Fatal("failed migration:", err)
	}
}

func resolveDBpath() string {
	if runtime.GOOS == "windows" {
		return "./data.db"
	}
	return "/etc/corvin-ui/data.db"
}

func ensureDir(dir string) error {
	if dir == "" || dir == "," {
		return nil
	}
	return os.MkdirAll(dir, 0755)
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
