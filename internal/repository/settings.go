package repository

import (
	"errors"
	"strconv"
	"vpnpanel/internal/models"

	"gorm.io/gorm"
)

// func init() {

// }

type SettingsRepo struct {
	db *gorm.DB
}

func NewSettingsRepo(db *gorm.DB) *SettingsRepo {
	return &SettingsRepo{db: db}
}

// Получение всех настроек
func (r *SettingsRepo) GetAll() ([]models.Settings, error) {
	var settings []models.Settings
	if err := r.db.Table("settings").Select("key", "value").Find(&settings).Error; err != nil {
		return nil, err
	}

	return settings, nil
}

// Получение значения по ключу
func (r *SettingsRepo) GetByKey(key string) (string, error) {
	var s models.Settings
	if err := r.db.Where("key = ?", key).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("setting not found")
		}
		return "", err
	}
	return s.Value, nil
}

// Получение нескольких ключей
func (r *SettingsRepo) GetKeys(keys ...string) (map[string]string, error) {
	var settings []models.Settings
	if err := r.db.Where("key IN ?", keys).Find(&settings).Error; err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

// Обновление или добавление ключа
func (r *SettingsRepo) Set(key, value string) error {
	var s models.Settings
	err := r.db.Where("key = ?", key).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s = models.Settings{Key: key, Value: value}
		return r.db.Create(&s).Error
	} else if err != nil {
		return err
	}

	s.Value = value
	return r.db.Save(&s).Error
}

// Массовое обновление ключей
func (r *SettingsRepo) UpdateSettings(updates map[string]string) error {
	for key, value := range updates {
		if err := r.Set(key, value); err != nil {
			return err
		}
	}
	return nil
}

// Пример: получить port как int
func (r *SettingsRepo) GetInt(key string) (int, error) {
	val, err := r.GetByKey(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(val)
}
