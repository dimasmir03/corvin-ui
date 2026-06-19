package service

import (
	"errors"
	"strings"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
)

type SupportService struct {
	telegramRepo  *repository.TelegramRepo
	complaintRepo *repository.ComplaintRepository
}

type CreateComplaintInput struct {
	TgID        int64
	Text        string
	PhotoFile   string
	PhotoFileID string
}

type ReplyToComplaintInput struct {
	AdminTgID   int64
	ComplaintID uint
	Text        string
}

type SupportReplyResult struct {
	ComplaintID uint
	UserTgID    int64
	Text        string
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
	if input.PhotoFileID != "" {
		complaint.Photo = true
		complaint.PhotoFileID = input.PhotoFileID
	}

	if err := s.complaintRepo.Create(complaint); err != nil {
		return models.Complaint{}, err
	}
	return *complaint, nil
}

func (s *SupportService) ReplyToComplaint(input ReplyToComplaintInput) (*SupportReplyResult, error) {
	replyText := strings.TrimSpace(input.Text)
	if replyText == "" {
		return nil, errors.New("reply text is empty")
	}

	complaint, err := s.complaintRepo.GetByID(input.ComplaintID)
	if err != nil {
		return nil, err
	}

	complaint, err = s.complaintRepo.SetReply(complaint.ID, replyText, input.AdminTgID)
	if err != nil {
		return nil, err
	}

	userTgID := complaint.TgID
	if complaint.UserID != 0 {
		telegram, err := s.telegramRepo.GetByUserID(complaint.UserID)
		if err != nil {
			return nil, err
		}
		userTgID = telegram.TgID
	}

	return &SupportReplyResult{
		ComplaintID: complaint.ID,
		UserTgID:    userTgID,
		Text:        replyText,
	}, nil
}
