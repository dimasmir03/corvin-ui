package service

import (
	"errors"
	"fmt"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"gorm.io/gorm"
)

type UsersService struct {
	telegramRepo *repository.TelegramRepo
	audit        *audit.Logger
}

type TelegramUserInput struct {
	TgID      int64
	Username  string
	Firstname string
	Lastname  string
}

type AdminTelegramUserView struct {
	UserID    uint
	TgID      int64
	Username  string
	Firstname string
	Lastname  string
}

func NewUsersService(
	telegramRepo *repository.TelegramRepo,
	auditLogger *audit.Logger,
) *UsersService {
	return &UsersService{
		telegramRepo: telegramRepo,
		audit:        auditLogger,
	}
}

func (s *UsersService) EnsureTelegramUser(input TelegramUserInput) (models.Telegram, error) {
	existing, err := s.telegramRepo.FindByTgID(input.TgID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Telegram{}, err
	}

	user := models.User{
		Username: fmt.Sprintf("%s%s(%d)", input.Firstname, input.Lastname, input.TgID),
		Status:   true,
	}
	telegram := models.Telegram{
		TgID:      input.TgID,
		Username:  input.Username,
		Firstname: input.Firstname,
		Lastname:  input.Lastname,
	}

	created, err := s.telegramRepo.CreateTelegramUser(user, telegram)
	if err != nil {
		// If another request created the telegram row after our lookup, keep the
		// service method idempotent and return that row instead of a duplicate error.
		if existing, findErr := s.telegramRepo.FindByTgID(input.TgID); findErr == nil {
			return existing, nil
		}
		return models.Telegram{}, err
	}

	_ = s.audit.Log(audit.Event{
		ActorType:  audit.ActorTelegramUser,
		ActorID:    audit.StringID(input.TgID),
		Action:     "user.created",
		EntityType: "user",
		EntityID:   audit.StringID(created.UserID),
		Status:     audit.StatusSuccess,
		Message:    "telegram user created",
	})

	return created, nil
}

func (s *UsersService) GetTelegramByTgID(tgID int64) (models.Telegram, error) {
	return s.telegramRepo.FindByTgID(tgID)
}

func (s *UsersService) GetAllTelegramUsers() ([]models.Telegram, error) {
	return s.telegramRepo.GetAllUsers()
}

func (s *UsersService) ListTelegramUsers() ([]AdminTelegramUserView, error) {
	users, err := s.telegramRepo.GetAllUsers()
	if err != nil {
		return nil, err
	}

	views := make([]AdminTelegramUserView, 0, len(users))
	for _, user := range users {
		views = append(views, AdminTelegramUserView{
			UserID:    user.UserID,
			TgID:      user.TgID,
			Username:  user.Username,
			Firstname: user.Firstname,
			Lastname:  user.Lastname,
		})
	}

	return views, nil
}
