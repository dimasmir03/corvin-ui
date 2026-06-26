package service

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
)

type SupportService struct {
	telegramRepo  *repository.TelegramRepo
	complaintRepo *repository.ComplaintRepository
	storageRepo   *repository.StorageRepo
}

type CreateComplaintInput struct {
	TgID        int64
	Text        string
	PhotoFile   string
	PhotoFileID string
	Photo       *ComplaintPhotoInput
}

type ComplaintPhotoInput struct {
	FileName       string
	MimeType       string
	Data           []byte
	TelegramFileID string
	TelegramUnique string
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

func NewSupportService(telegramRepo *repository.TelegramRepo, complaintRepo *repository.ComplaintRepository, storageRepo *repository.StorageRepo) *SupportService {
	return &SupportService{
		telegramRepo:  telegramRepo,
		complaintRepo: complaintRepo,
		storageRepo:   storageRepo,
	}
}

func (s *SupportService) CreateComplaint(input CreateComplaintInput) (models.Complaint, error) {
	logger.Info("support complaint create requested", "component", "support_service", "operation", "create_complaint", "tg_id", input.TgID, "text_len", len(strings.TrimSpace(input.Text)), "has_photo", input.Photo != nil || input.PhotoFile != "" || input.PhotoFileID != "")
	telegram, err := s.telegramRepo.GetTelegramByTgID(input.TgID)
	if err != nil {
		logger.Error("support complaint user lookup failed", err, "component", "support_service", "operation", "create_complaint", "tg_id", input.TgID)
		return models.Complaint{}, err
	}
	logger.Info("support complaint user found", "component", "support_service", "operation", "create_complaint", "tg_id", input.TgID, "user_id", telegram.UserID)

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
		complaint.PhotoObjectKey = input.PhotoFile
	}
	if input.PhotoFileID != "" {
		complaint.Photo = true
		complaint.PhotoFileID = input.PhotoFileID
	}

	if err := s.complaintRepo.Create(complaint); err != nil {
		logger.Error("support complaint create failed", err, "component", "support_service", "operation", "create_complaint", "tg_id", input.TgID, "user_id", telegram.UserID)
		return models.Complaint{}, err
	}
	logger.Info("support complaint created", "component", "support_service", "operation", "create_complaint", "tg_id", input.TgID, "user_id", telegram.UserID, "complaint_id", complaint.ID, "has_photo", complaint.Photo)

	if input.Photo != nil {
		if s.storageRepo == nil {
			logger.Error("support complaint attachment upload failed", nil, "component", "support_service", "operation", "create_complaint", "tg_id", input.TgID, "complaint_id", complaint.ID, "reason", "storage_not_configured")
			return models.Complaint{}, errors.New("storage is not configured")
		}

		objectKey := complaintPhotoObjectKey(complaint.ID, input.Photo)
		contentType := strings.TrimSpace(input.Photo.MimeType)
		if contentType == "" {
			contentType = "image/jpeg"
		}

		logger.Info("support complaint attachment upload started", "component", "support_service", "operation", "create_complaint", "tg_id", input.TgID, "complaint_id", complaint.ID, "content_type", contentType)
		if _, err := s.storageRepo.UploadFile(bytes.NewReader(input.Photo.Data), objectKey, contentType); err != nil {
			logger.Error("support complaint attachment upload failed", err, "component", "support_service", "operation", "create_complaint", "tg_id", input.TgID, "complaint_id", complaint.ID, "content_type", contentType)
			return models.Complaint{}, err
		}
		logger.Info("support complaint attachment uploaded", "component", "support_service", "operation", "create_complaint", "tg_id", input.TgID, "complaint_id", complaint.ID, "content_type", contentType)

		updated, err := s.complaintRepo.SetPhoto(complaint.ID, objectKey, objectKey, input.Photo.TelegramFileID)
		if err != nil {
			logger.Error("support complaint photo update failed", err, "component", "support_service", "operation", "create_complaint", "tg_id", input.TgID, "complaint_id", complaint.ID)
			return models.Complaint{}, err
		}
		return updated, nil
	}

	return *complaint, nil
}

func (s *SupportService) ReplyToComplaint(input ReplyToComplaintInput) (*SupportReplyResult, error) {
	logger.Info("support reply requested", "component", "support_service", "operation", "reply_to_complaint", "admin_tg_id", input.AdminTgID, "complaint_id", input.ComplaintID, "text_len", len(strings.TrimSpace(input.Text)))
	replyText := strings.TrimSpace(input.Text)
	if replyText == "" {
		logger.Warn("support reply rejected", "component", "support_service", "operation", "reply_to_complaint", "admin_tg_id", input.AdminTgID, "complaint_id", input.ComplaintID, "reason", "reply_text_empty")
		return nil, errors.New("reply text is empty")
	}

	complaint, err := s.complaintRepo.GetByID(input.ComplaintID)
	if err != nil {
		logger.Error("support reply complaint lookup failed", err, "component", "support_service", "operation", "reply_to_complaint", "admin_tg_id", input.AdminTgID, "complaint_id", input.ComplaintID)
		return nil, err
	}

	complaint, err = s.complaintRepo.SetReply(complaint.ID, replyText, input.AdminTgID)
	if err != nil {
		logger.Error("support reply save failed", err, "component", "support_service", "operation", "reply_to_complaint", "admin_tg_id", input.AdminTgID, "complaint_id", input.ComplaintID)
		return nil, err
	}
	logger.Info("support reply saved", "component", "support_service", "operation", "reply_to_complaint", "admin_tg_id", input.AdminTgID, "complaint_id", complaint.ID, "text_len", len(replyText))

	userTgID := complaint.TgID
	if complaint.UserID != 0 {
		telegram, err := s.telegramRepo.GetByUserID(complaint.UserID)
		if err != nil {
			logger.Error("support reply telegram user lookup failed", err, "component", "support_service", "operation", "reply_to_complaint", "admin_tg_id", input.AdminTgID, "complaint_id", complaint.ID, "user_id", complaint.UserID)
			return nil, err
		}
		userTgID = telegram.TgID
	}

	logger.Info("support reply result ready", "component", "support_service", "operation", "reply_to_complaint", "admin_tg_id", input.AdminTgID, "complaint_id", complaint.ID, "user_tg_id", userTgID, "text_len", len(replyText))
	return &SupportReplyResult{
		ComplaintID: complaint.ID,
		UserTgID:    userTgID,
		Text:        replyText,
	}, nil
}

func complaintPhotoObjectKey(complaintID uint, photo *ComplaintPhotoInput) string {
	base := strings.TrimSpace(photo.TelegramUnique)
	if base == "" {
		base = strings.TrimSpace(photo.FileName)
	}
	if base == "" {
		base = fmt.Sprintf("photo_%d", complaintID)
	}

	ext := strings.ToLower(filepath.Ext(photo.FileName))
	if ext == "" {
		ext = ".jpg"
	}

	name := safeObjectKeyPart(strings.TrimSuffix(base, filepath.Ext(base)))
	return fmt.Sprintf("complaints/%d/%d_%s%s", complaintID, time.Now().UnixNano(), name, ext)
}

func safeObjectKeyPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "photo"
	}

	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "photo"
	}
	return b.String()
}
