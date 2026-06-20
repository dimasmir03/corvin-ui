package audit

import (
	"encoding/json"
	"fmt"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
)

const (
	ActorAdmin        = "admin"
	ActorSystem       = "system"
	ActorTelegramUser = "telegram_user"
	ActorJob          = "job"
	ActorAgent        = "agent"

	StatusSuccess = "success"
	StatusFailed  = "failed"
)

type Logger struct {
	repo *repository.AuditRepo
}

type Event struct {
	ActorType  string
	ActorID    *string
	Action     string
	EntityType string
	EntityID   *string
	Status     string
	Message    string
	OldValue   any
	NewValue   any
	Metadata   any
	IP         string
	UserAgent  string
}

func NewLogger(repo *repository.AuditRepo) *Logger {
	return &Logger{repo: repo}
}

func (l *Logger) Log(event Event) error {
	if l == nil || l.repo == nil {
		return nil
	}

	auditLog := &models.AuditLog{
		ActorType:  event.ActorType,
		ActorID:    event.ActorID,
		Action:     event.Action,
		EntityType: event.EntityType,
		EntityID:   event.EntityID,
		Status:     event.Status,
		Message:    event.Message,
		IP:         event.IP,
		UserAgent:  event.UserAgent,
	}

	var err error
	if auditLog.OldValueJSON, err = marshalOptional(event.OldValue); err != nil {
		return err
	}
	if auditLog.NewValueJSON, err = marshalOptional(event.NewValue); err != nil {
		return err
	}
	if auditLog.MetadataJSON, err = marshalOptional(event.Metadata); err != nil {
		return err
	}

	return l.repo.Create(auditLog)
}

func StringID(value any) *string {
	if value == nil {
		return nil
	}
	id := fmt.Sprint(value)
	return &id
}

func marshalOptional(value any) (*string, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	encoded := string(data)
	return &encoded, nil
}
