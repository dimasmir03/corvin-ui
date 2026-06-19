package service

import (
	"strings"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
)

type SupportService struct {
	telegramRepo  *repository.TelegramRepo
	complaintRepo *repository.ComplaintRepository
}

type CreateComplaintInput struct {
	TgID      int64
	Text      string
	PhotoFile string
}

func NewSupportService(telegramRepo *repository.TelegramRepo, complaintRepo *repository.ComplaintRepository) *SupportService {
	return &SupportService{
		telegramRepo:  telegramRepo,
		complaintRepo: complaintRepo,
	}
}

func (s *SupportService) CreateComplaint(input CreateComplaintInput) (models.Complaint, error) {
	telegram, err := s.telegramRepo.GetTelegramByTgID(input.TgID)
	if err != nil {
		return models.Complaint{}, err
	}

	text := strings.TrimSpace(input.Text)
	if text == "" {
		text = "Фото без описания"
	}

	complaint := &models.Complaint{
		TgID:     input.TgID,
		Username: telegram.Username,
		Text:     text,
		Status:   "new",
		UserID:   telegram.UserID,
	}
	if input.PhotoFile != "" {
		complaint.Photo = true
		complaint.PhotoURL = input.PhotoFile
	}

	if err := s.complaintRepo.Create(complaint); err != nil {
		return models.Complaint{}, err
	}
	return *complaint, nil
}
