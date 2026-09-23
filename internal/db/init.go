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
	// Older versions enforced one VPN credential for the whole user. Credentials
	// are now scoped to a device; keep one nullable legacy credential while
	// allowing any number of device-specific credentials.
	if err := DB.Exec("DROP INDEX IF EXISTS idx_vpn_clients_user_id").Error; err != nil {
		return err
	}
	if err := DB.AutoMigrate(
		&models.User{},
		&models.NodeState{},
		&models.ServerRegistry{},
		&models.ServerInbound{},
		&models.NodeStateSnapshot{},
		&models.EndpointGroup{},
		&models.UserSubscription{},
		&models.VPNRoutingSettings{},
		&models.MobileDevice{},
		&models.MobileLoginSession{},
		&models.MobileSession{},
		&models.MobileUsedRefreshToken{},
		&models.MobileOperation{},
		&models.MobileIdempotencyRecord{},
		&models.UserServerAccess{},
		&models.VPNClient{},
		&models.VPNProfile{},
		&models.VPNProfileNode{},
		&models.Telegram{},
		&models.Vpn{},
		&models.Complaint{},
		&models.Settings{},
		&models.AuditLog{},
	); err != nil {
		return err
	}
	if err := DB.Exec("UPDATE node_states SET server_id = node_id WHERE (server_id IS NULL OR server_id = '') AND node_id <> ''").Error; err != nil {
		return err
	}
	if err := DB.Exec("UPDATE vpn_profile_nodes SET server_id = node_id WHERE (server_id IS NULL OR server_id = '') AND node_id <> ''").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_node_states_server_id_unique ON node_states (server_id) WHERE server_id IS NOT NULL AND server_id <> ''").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_vpn_profile_nodes_profile_server_unique ON vpn_profile_nodes (vpn_profile_id, server_id) WHERE server_id IS NOT NULL AND server_id <> ''").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_vpn_clients_user_legacy_unique ON vpn_clients (user_id) WHERE device_id IS NULL").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_vpn_clients_user_device_unique ON vpn_clients (user_id, device_id) WHERE device_id IS NOT NULL AND device_id <> ''").Error; err != nil {
		return err
	}
	if err := DB.Exec("UPDATE vpn_profile_nodes SET desired_state = 'enabled' WHERE desired_state IS NULL OR desired_state = ''").Error; err != nil {
		return err
	}
	return DB.Exec(`
		INSERT INTO server_registry (server_id, display_name, endpoint_group, expected_protocol, source, enabled, first_seen_at, last_seen_at, created_at, updated_at)
		SELECT DISTINCT server_id, server_id, COALESCE(NULLIF(endpoint_group, ''), 'unknown'), COALESCE(NULLIF(expected_protocol, ''), NULLIF(protocol, ''), 'unknown'), 'migrated', enabled, COALESCE(last_seen_at, NOW()), COALESCE(last_seen_at, NOW()), NOW(), NOW()
		FROM node_states
		WHERE server_id IS NOT NULL AND server_id <> ''
		ON CONFLICT (server_id) DO NOTHING
	`).Error
}
