package db

import (
	"fmt"
	"io"
	"time"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
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

func Init(options DBOptions, w io.Writer) {
	logger.Info("database connect started", "component", "startup", "operation", "db_connect", "db_host", options.Host, "db_port", options.Port, "db_name", options.DBName)
	if options.Host == "" || options.Port == 0 || options.User == "" || options.Pass == "" || options.DBName == "" {
		logger.Error("database config invalid", nil, "component", "startup", "operation", "db_connect", "db_host", options.Host, "db_port", options.Port, "db_name", options.DBName, "reason", "database_options_empty")
		logger.Fatal("database options are empty")
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

	newLogger := gormlogger.New(
		logger.NewWithWriter(w, "info", "text"),
		gormlogger.Config{
			SlowThreshold: time.Second,
			LogLevel:      gormlogger.Info,
			Colorful:      false,
		},
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		logger.Error("database connect failed", err, "component", "startup", "operation", "db_connect", "db_host", options.Host, "db_port", options.Port, "db_name", options.DBName)
		logger.Fatal("failed to connect database:", err)
	}
	logger.Info("database connected", "component", "startup", "operation", "db_connect", "db_host", options.Host, "db_port", options.Port, "db_name", options.DBName)

	logger.Info("database migrations started", "component", "startup", "operation", "migration")
	if err := migrate(); err != nil {
		logger.Error("database migrations failed", err, "component", "startup", "operation", "migration")
		logger.Fatal("failed migration:", err)
	}
	logger.Info("database migrations completed", "component", "startup", "operation", "migration")
}

func migrate() error {
	if err := DB.AutoMigrate(
		&models.User{},
		&models.Server{},
		&models.ServerStat{},
		&models.NodeStatsSnapshot{},
		&models.NodeState{},
		&models.EndpointGroup{},
		&models.VPNClient{},
		&models.VPNProfile{},
		&models.VPNProfileNode{},
		&models.Telegram{},
		&models.Vpn{},
		&models.Complaint{},
		&models.Settings{},
		&models.JobBatch{},
		&models.Job{},
		&models.AuditLog{},
	); err != nil {
		return err
	}
	if err := DB.Exec("UPDATE node_states SET server_id = node_id WHERE (server_id IS NULL OR server_id = '') AND node_id <> ''").Error; err != nil {
		return err
	}
	return DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_node_states_server_id_unique ON node_states (server_id) WHERE server_id IS NOT NULL AND server_id <> ''").Error
}
