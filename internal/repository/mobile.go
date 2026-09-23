package repository

import (
	"errors"
	"time"
	"vpnpanel/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MobileRepo struct{ DB *gorm.DB }

func NewMobileRepo(db *gorm.DB) *MobileRepo { return &MobileRepo{DB: db} }

func (r *MobileRepo) CreateLogin(login *models.MobileLoginSession) error {
	return r.DB.Create(login).Error
}

func (r *MobileRepo) GetLogin(id string) (models.MobileLoginSession, error) {
	var login models.MobileLoginSession
	err := r.DB.Where("id = ?", id).Take(&login).Error
	return login, err
}

func (r *MobileRepo) ApproveLogin(tokenHash string, userID uint, now time.Time) error {
	result := r.DB.Model(&models.MobileLoginSession{}).
		Where("approval_token_hash = ? AND status = ? AND expires_at > ?", tokenHash, "pending", now).
		Updates(map[string]any{"status": "approved", "user_id": userID, "approved_at": &now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *MobileRepo) ExchangeLogin(loginID string, now time.Time, fn func(*gorm.DB, models.MobileLoginSession) error) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		var login models.MobileLoginSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", loginID).Take(&login).Error; err != nil {
			return err
		}
		if login.ExchangedAt != nil {
			return ErrMobileLoginReplayed
		}
		if !login.ExpiresAt.After(now) {
			return ErrMobileLoginExpired
		}
		if login.Status != "approved" || login.UserID == nil {
			return ErrMobileLoginPending
		}
		if err := fn(tx, login); err != nil {
			return err
		}
		return tx.Model(&models.MobileLoginSession{}).Where("id = ? AND exchanged_at IS NULL", loginID).Updates(map[string]any{"exchanged_at": &now, "updated_at": now}).Error
	})
}

var (
	ErrMobileLoginReplayed = errors.New("mobile login already exchanged")
	ErrMobileLoginExpired  = errors.New("mobile login expired")
	ErrMobileLoginPending  = errors.New("mobile login is not approved")
	ErrRefreshReused       = errors.New("refresh token reused")
)

func (r *MobileRepo) CreateDeviceAndSession(tx *gorm.DB, device models.MobileDevice, session models.MobileSession) error {
	var existing models.MobileDevice
	err := tx.Where("install_id = ?", device.InstallID).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := tx.Create(&device).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if existing.UserID != device.UserID {
			return errors.New("install already belongs to another user")
		}
		if existing.Status == models.MobileDeviceStatusRevoked {
			return errors.New("device is revoked")
		}
		device.ID = existing.ID
		if err := tx.Model(&existing).Updates(map[string]any{"device_name": device.DeviceName, "platform": device.Platform, "last_seen_at": device.LastSeenAt, "updated_at": device.UpdatedAt}).Error; err != nil {
			return err
		}
	}
	session.DeviceID = device.ID
	return tx.Create(&session).Error
}

func (r *MobileRepo) GetSession(id string) (models.MobileSession, error) {
	var session models.MobileSession
	err := r.DB.Where("id = ?", id).Take(&session).Error
	return session, err
}

func (r *MobileRepo) FindSessionByRefreshHash(hash string) (models.MobileSession, error) {
	var session models.MobileSession
	err := r.DB.Where("refresh_token_hash = ?", hash).Take(&session).Error
	return session, err
}

func (r *MobileRepo) RotateRefresh(oldHash, newHash, installID string, expiresAt, now time.Time) (models.MobileSession, error) {
	var rotated models.MobileSession
	reused := false
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		var used models.MobileUsedRefreshToken
		if err := tx.Where("token_hash = ?", oldHash).Take(&used).Error; err == nil {
			reused = true
			return tx.Model(&models.MobileSession{}).Where("id = ?", used.SessionID).Updates(map[string]any{"revoked_at": &now, "updated_at": now}).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var session models.MobileSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("refresh_token_hash = ?", oldHash).Take(&session).Error; err != nil {
			return err
		}
		if session.RevokedAt != nil || !session.ExpiresAt.After(now) || session.InstallID != installID {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Create(&models.MobileUsedRefreshToken{SessionID: session.ID, TokenHash: oldHash, UsedAt: now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&session).Updates(map[string]any{"refresh_token_hash": newHash, "expires_at": expiresAt, "last_used_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", session.ID).Take(&rotated).Error
	})
	if err == nil && reused {
		return models.MobileSession{}, ErrRefreshReused
	}
	return rotated, err
}

func (r *MobileRepo) RevokeSession(id string, now time.Time) error {
	return r.DB.Model(&models.MobileSession{}).Where("id = ? AND revoked_at IS NULL", id).Updates(map[string]any{"revoked_at": &now, "updated_at": now}).Error
}

func (r *MobileRepo) GetDevice(id string) (models.MobileDevice, error) {
	var device models.MobileDevice
	err := r.DB.Where("id = ?", id).Take(&device).Error
	return device, err
}

func (r *MobileRepo) TouchDevice(id string, now time.Time) error {
	return r.DB.Model(&models.MobileDevice{}).Where("id = ?", id).Updates(map[string]any{"last_seen_at": now, "updated_at": now}).Error
}

func (r *MobileRepo) ListDevices(userID uint) ([]models.MobileDevice, error) {
	var devices []models.MobileDevice
	err := r.DB.Where("user_id = ?", userID).Order("created_at ASC").Find(&devices).Error
	return devices, err
}

func (r *MobileRepo) CountActiveDevices(userID uint) (int64, error) {
	var count int64
	err := r.DB.Model(&models.MobileDevice{}).Where("user_id = ? AND status = ?", userID, models.MobileDeviceStatusActive).Count(&count).Error
	return count, err
}

func (r *MobileRepo) RevokeDevice(userID uint, deviceID string, now time.Time) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.MobileDevice{}).Where("id = ? AND user_id = ?", deviceID, userID).Updates(map[string]any{"status": models.MobileDeviceStatusRevoked, "revoked_at": &now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&models.MobileSession{}).Where("device_id = ? AND revoked_at IS NULL", deviceID).Updates(map[string]any{"revoked_at": &now, "updated_at": now}).Error
	})
}

func (r *MobileRepo) GetUser(userID uint) (models.User, error) {
	var user models.User
	err := r.DB.Where("id = ?", userID).Take(&user).Error
	return user, err
}

func (r *MobileRepo) GetTelegram(userID uint) (models.Telegram, error) {
	var telegram models.Telegram
	err := r.DB.Where("user_id = ?", userID).Take(&telegram).Error
	return telegram, err
}

func (r *MobileRepo) CreateOperationWithIdempotency(operation models.MobileOperation, record models.MobileIdempotencyRecord) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&operation).Error; err != nil {
			return err
		}
		return tx.Create(&record).Error
	})
}

func (r *MobileRepo) GetIdempotency(deviceID, key string) (models.MobileIdempotencyRecord, error) {
	var record models.MobileIdempotencyRecord
	err := r.DB.Where("device_id = ? AND key = ?", deviceID, key).Take(&record).Error
	return record, err
}

func (r *MobileRepo) GetOperation(id string) (models.MobileOperation, error) {
	var operation models.MobileOperation
	err := r.DB.Where("id = ?", id).Take(&operation).Error
	return operation, err
}

func (r *MobileRepo) UpdateOperation(id string, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	return r.DB.Model(&models.MobileOperation{}).Where("id = ?", id).Updates(updates).Error
}

func (r *MobileRepo) Cleanup(now time.Time) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at < ?", now).Delete(&models.MobileLoginSession{}).Error; err != nil {
			return err
		}
		if err := tx.Where("expires_at < ?", now).Delete(&models.MobileIdempotencyRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("expires_at < ?", now.Add(-24*time.Hour)).Delete(&models.MobileOperation{}).Error; err != nil {
			return err
		}
		if err := tx.Where("expires_at < ? OR (revoked_at IS NOT NULL AND revoked_at < ?)", now.Add(-24*time.Hour), now.Add(-30*24*time.Hour)).Delete(&models.MobileSession{}).Error; err != nil {
			return err
		}
		return tx.Where("used_at < ?", now.Add(-30*24*time.Hour)).Delete(&models.MobileUsedRefreshToken{}).Error
	})
}
