package service

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
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
		complaint.PhotoObjectKey = input.PhotoFile
	}
	if input.PhotoFileID != "" {
		complaint.Photo = true
		complaint.PhotoFileID = input.PhotoFileID
	}

	if err := s.complaintRepo.Create(complaint); err != nil {
		return models.Complaint{}, err
	}

	if input.Photo != nil {
		if s.storageRepo == nil {
			return models.Complaint{}, errors.New("storage is not configured")
		}

		objectKey := complaintPhotoObjectKey(complaint.ID, input.Photo)
		contentType := strings.TrimSpace(input.Photo.MimeType)
		if contentType == "" {
			contentType = "image/jpeg"
		}

		if _, err := s.storageRepo.UploadFile(bytes.NewReader(input.Photo.Data), objectKey, contentType); err != nil {
			return models.Complaint{}, err
		}

		updated, err := s.complaintRepo.SetPhoto(complaint.ID, objectKey, objectKey, input.Photo.TelegramFileID)
		if err != nil {
			return models.Complaint{}, err
		}
		return updated, nil
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
