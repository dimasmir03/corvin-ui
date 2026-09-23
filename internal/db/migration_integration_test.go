//go:build integration_postgres

package db

import (
	"fmt"
	"os"
	"testing"
	"vpnpanel/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresMigrationsAreRepeatableAndAllowDeviceCredentials(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not configured")
	}
	testDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect PostgreSQL: %v", err)
	}
	previous := DB
	DB = testDB
	defer func() { DB = previous }()
	if err := migrate(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := migrate(); err != nil {
		t.Fatalf("repeat migrate: %v", err)
	}
	for _, model := range []any{&models.ServerRegistry{}, &models.UserSubscription{}, &models.MobileLoginSession{}, &models.MobileSession{}, &models.MobileOperation{}, &models.MobileIdempotencyRecord{}} {
		if !testDB.Migrator().HasTable(model) {
			t.Fatalf("missing migrated table for %T", model)
		}
	}
	var partialIndexes int64
	if err := testDB.Raw("SELECT count(*) FROM pg_indexes WHERE indexname IN ('idx_vpn_clients_user_legacy_unique','idx_vpn_clients_user_device_unique')").Scan(&partialIndexes).Error; err != nil || partialIndexes != 2 {
		t.Fatalf("partial indexes=%d err=%v", partialIndexes, err)
	}
	user := models.User{Username: "migration-user", Status: true}
	if err := testDB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	for i, deviceID := range []string{"device-a", "device-b"} {
		id := deviceID
		client := models.VPNClient{UserID: user.ID, DeviceID: &id, TelegramID: 100, ClientCode: fmt.Sprintf("migration_%d", i), Email: fmt.Sprintf("migration_%d", i), VlessUUID: fmt.Sprintf("uuid-%d", i), TrojanPassword: fmt.Sprintf("password-%d", i)}
		if err := testDB.Create(&client).Error; err != nil {
			t.Fatalf("create device client: %v", err)
		}
	}
	duplicateID := "device-a"
	duplicate := models.VPNClient{UserID: user.ID, DeviceID: &duplicateID, TelegramID: 100, ClientCode: "migration_duplicate", Email: "migration_duplicate", VlessUUID: "uuid-duplicate", TrojanPassword: "password-duplicate"}
	if err := testDB.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate user/device credentials were accepted")
	}
}
