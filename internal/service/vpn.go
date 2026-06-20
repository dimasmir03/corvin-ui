package service

import (
	"encoding/json"
	"errors"
	"strings"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/jobsvc"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/utils"

	"gorm.io/gorm"
)

var (
	ErrVPNAlreadyExists    = errors.New("vpn already exists")
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
)

type VPNErrorKind string

const (
	VPNErrorKindJobs   VPNErrorKind = "jobs"
	VPNErrorKindBroker VPNErrorKind = "broker"
)

type VPNFlowError struct {
	Kind VPNErrorKind
	Err  error
}

func (e *VPNFlowError) Error() string {
	return e.Err.Error()
}

func (e *VPNFlowError) Unwrap() error {
	return e.Err
}

func IsVPNFlowError(err error, kind VPNErrorKind) bool {
	var flowErr *VPNFlowError
	return errors.As(err, &flowErr) && flowErr.Kind == kind
}

type CreateVPNInput struct {
	TgID int64
}

type CreateVPNProtocolInput struct {
	TgID     int64
	Protocol string
}

type RequestCreateVPNInput struct {
	TgID     int64
	Protocol string
}

type RequestCreateVPNResult struct {
	TgID     int64
	Protocol string
	BatchID  uint
	JobID    uint
}

type VPNReadyNotification struct {
	TgID     int64
	Protocol string
	Link     string
}

type VPNService struct {
	vpnRepo      *repository.VpnRepo
	telegramRepo *repository.TelegramRepo
	jobs         *jobsvc.Service
	audit        *audit.Logger
}

func NewVPNService(
	vpnRepo *repository.VpnRepo,
	telegramRepo *repository.TelegramRepo,
	jobs *jobsvc.Service,
	auditLogger *audit.Logger,
) *VPNService {
	return &VPNService{
		vpnRepo:      vpnRepo,
		telegramRepo: telegramRepo,
		jobs:         jobs,
		audit:        auditLogger,
	}
}

func (s *VPNService) RequestCreateVPN(input RequestCreateVPNInput) (*RequestCreateVPNResult, error) {
	protocol := strings.ToLower(strings.TrimSpace(input.Protocol))
	if protocol == "" {
		protocol = "vless"
	}
	if protocol != "vless" && protocol != "trojan" {
		return nil, ErrUnsupportedProtocol
	}

	telegram, err := s.telegramRepo.GetTelegramByTgID(input.TgID)
	if err != nil {
		return nil, err
	}

	vpn, err := s.vpnRepo.GetByUserID(telegram.UserID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err == nil {
		switch protocol {
		case "vless":
			if strings.TrimSpace(vpn.VlessLink) != "" {
				return nil, ErrVPNAlreadyExists
			}
		case "trojan":
			if strings.TrimSpace(vpn.TrojanLink) != "" {
				return nil, ErrVPNAlreadyExists
			}
		}
	}

	if s.jobs == nil {
		return nil, errors.New("jobs service is not configured")
	}

	vlessParams := utils.GenVlessLink(input.TgID)
	trojanParams := utils.GenTrojanLink(input.TgID)
	clientCode := vlessParams.Name
	technicalClientID := vlessParams.UID
	if protocol == "trojan" {
		technicalClientID = trojanParams.Password
		clientCode = trojanParams.Name
	}

	batch, jobs, err := s.jobs.CreateUserConfig(jobsvc.CreateUserConfigInput{
		UserID:            telegram.UserID,
		TelegramID:        input.TgID,
		ClientCode:        clientCode,
		Email:             clientCode,
		VlessUUID:         vlessParams.UID,
		VlessFlow:         vlessParams.Flow,
		TrojanPassword:    trojanParams.Password,
		Enable:            true,
		TechnicalClientID: technicalClientID,
		Protocols:         []string{protocol},
	})
	if err != nil {
		return nil, &VPNFlowError{Kind: VPNErrorKindJobs, Err: err}
	}

	var jobID uint
	if len(jobs) > 0 {
		jobID = jobs[0].ID
	}

	return &RequestCreateVPNResult{
		TgID:     input.TgID,
		Protocol: protocol,
		BatchID:  batch.ID,
		JobID:    jobID,
	}, nil
}

func (s *VPNService) ApplyAgentCreateResult(job *models.Job, event broker.JobResultEvent) (*VPNReadyNotification, error) {
	if job == nil || job.Action != jobsvc.ActionCreateClient {
		return nil, nil
	}
	if job.Status != jobsvc.JobStatusSuccess {
		return nil, nil
	}

	var payload broker.JobTask
	if len(job.PayloadJSON) > 0 {
		if err := json.Unmarshal(job.PayloadJSON, &payload); err != nil {
			return nil, err
		}
	}

	protocol := strings.ToLower(strings.TrimSpace(payload.Protocol))
	if protocol == "" {
		protocol = strings.ToLower(strings.TrimSpace(job.Protocol))
	}
	if protocol != "vless" && protocol != "trojan" {
		return nil, ErrUnsupportedProtocol
	}

	userID := payload.UserID
	if userID == 0 {
		return nil, errors.New("job payload user_id is empty")
	}

	link := strings.TrimSpace(valueOrEmptyString(event.ConfigLink))
	if link == "" {
		link = strings.TrimSpace(configLinkFromResult(event.ResultJSON))
	}
	if link == "" {
		return nil, errors.New("agent result config link is empty")
	}

	if _, err := s.vpnRepo.UpsertLinkByUserID(userID, protocol, link); err != nil {
		return nil, err
	}

	telegram, err := s.telegramRepo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}

	return &VPNReadyNotification{
		TgID:     telegram.TgID,
		Protocol: protocol,
		Link:     link,
	}, nil
}

func configLinkFromResult(raw *json.RawMessage) string {
	if raw == nil || len(*raw) == 0 {
		return ""
	}
	var payload struct {
		ConfigLink string `json:"config_link"`
		Link       string `json:"link"`
	}
	if err := json.Unmarshal(*raw, &payload); err != nil {
		return ""
	}
	if payload.ConfigLink != "" {
		return payload.ConfigLink
	}
	return payload.Link
}

func valueOrEmptyString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *VPNService) CreateVPN(input CreateVPNInput) (models.Vpn, error) {
	vlessParams := utils.GenVlessLink(input.TgID)
	trojanParams := utils.GenTrojanLink(input.TgID)

	telegram, err := s.telegramRepo.GetTelegramByTgID(input.TgID)
	if err != nil {
		return models.Vpn{}, err
	}

	vpn, err := s.vpnRepo.Create(models.Vpn{
		UUID:       vlessParams.UID,
		UserID:     telegram.UserID,
		Status:     "active",
		Link:       vlessParams.Link,
		VlessLink:  vlessParams.Link,
		TrojanLink: trojanParams.Link,
	})
	if err != nil {
		return models.Vpn{}, err
	}

	if s.jobs != nil {
		batch, jobs, err := s.jobs.CreateUserConfig(jobsvc.CreateUserConfigInput{
			UserID:            telegram.UserID,
			TelegramID:        input.TgID,
			ClientCode:        vlessParams.Name,
			Email:             vlessParams.Name,
			VlessUUID:         vlessParams.UID,
			VlessFlow:         vlessParams.Flow,
			TrojanPassword:    trojanParams.Password,
			Enable:            true,
			TechnicalClientID: vlessParams.UID,
			Protocols:         []string{"vless", "trojan"},
		})
		if err != nil {
			return models.Vpn{}, &VPNFlowError{Kind: VPNErrorKindJobs, Err: err}
		}
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorTelegramUser,
			ActorID:    audit.StringID(input.TgID),
			Action:     "vpn.client.created",
			EntityType: "job_batch",
			EntityID:   audit.StringID(batch.ID),
			Status:     audit.StatusSuccess,
			Message:    "vpn create jobs queued",
			Metadata: map[string]any{
				"jobs_count": len(jobs),
				"user_id":    telegram.UserID,
			},
		})
	}

	task := broker.CreateUserTask{
		UserID:     input.TgID,
		Username:   vlessParams.Name,
		UUID:       vlessParams.UID,
		PBK:        vlessParams.PBK,
		SID:        vlessParams.SID,
		SPX:        vlessParams.SPX,
		Flow:       vlessParams.Flow,
		Encryption: vlessParams.Encryption,

		Type:     trojanParams.Type,
		Security: trojanParams.Security,
		Fp:       trojanParams.Fp,
		Alpn:     trojanParams.Alpn,
		Sni:      trojanParams.Sni,
		Password: trojanParams.Password,
	}

	if err := broker.GlobalProducer.PublishCreateUser(task); err != nil {
		return models.Vpn{}, &VPNFlowError{Kind: VPNErrorKindBroker, Err: err}
	}

	return vpn, nil
}

func (s *VPNService) CreateVPNProtocol(input CreateVPNProtocolInput) (models.Vpn, error) {
	var vlessParams utils.VlessParams
	var trojanParams utils.TrojanParams
	switch input.Protocol {
	case "vless":
		vlessParams = utils.GenVlessLink(input.TgID)
	case "trojan":
		trojanParams = utils.GenTrojanLink(input.TgID)
	}

	telegram, err := s.telegramRepo.GetTelegramByTgID(input.TgID)
	if err != nil {
		return models.Vpn{}, err
	}

	var vpn models.Vpn
	switch input.Protocol {
	case "vless":
		vpn, err = s.upsertVPNProtocol(telegram.UserID, input.Protocol, vlessParams.Link)
		if err != nil {
			return models.Vpn{}, err
		}
	case "trojan":
		vpn, err = s.upsertVPNProtocol(telegram.UserID, input.Protocol, trojanParams.Link)
		if err != nil {
			return models.Vpn{}, err
		}
	}

	var username string
	switch input.Protocol {
	case "vless":
		username = vlessParams.Name
	case "trojan":
		username = trojanParams.Name
	}

	if s.jobs != nil {
		technicalClientID := vlessParams.UID
		if input.Protocol == "trojan" {
			technicalClientID = trojanParams.Password
		}
		batch, jobs, err := s.jobs.CreateUserConfig(jobsvc.CreateUserConfigInput{
			UserID:            telegram.UserID,
			TelegramID:        input.TgID,
			ClientCode:        username,
			Email:             username,
			VlessUUID:         vlessParams.UID,
			VlessFlow:         vlessParams.Flow,
			TrojanPassword:    trojanParams.Password,
			Enable:            true,
			TechnicalClientID: technicalClientID,
			Protocols:         []string{input.Protocol},
		})
		if err != nil {
			return models.Vpn{}, &VPNFlowError{Kind: VPNErrorKindJobs, Err: err}
		}
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorTelegramUser,
			ActorID:    audit.StringID(input.TgID),
			Action:     "vpn.client.created",
			EntityType: "job_batch",
			EntityID:   audit.StringID(batch.ID),
			Status:     audit.StatusSuccess,
			Message:    "vpn protocol create jobs queued",
			Metadata: map[string]any{
				"jobs_count": len(jobs),
				"protocol":   input.Protocol,
				"user_id":    telegram.UserID,
			},
		})
	}

	task := broker.CreateUserTask{
		UserID:     input.TgID,
		Username:   username,
		UUID:       vlessParams.UID,
		PBK:        vlessParams.PBK,
		SID:        vlessParams.SID,
		SPX:        vlessParams.SPX,
		Flow:       vlessParams.Flow,
		Encryption: vlessParams.Encryption,

		Type:     trojanParams.Type,
		Security: trojanParams.Security,
		Fp:       trojanParams.Fp,
		Alpn:     trojanParams.Alpn,
		Sni:      trojanParams.Sni,
		Password: trojanParams.Password,
	}

	if err := broker.GlobalProducer.PublishCreateUser(task); err != nil {
		return models.Vpn{}, &VPNFlowError{Kind: VPNErrorKindBroker, Err: err}
	}

	return vpn, nil
}

func (s *VPNService) upsertVPNProtocol(userID uint, protocol string, link string) (models.Vpn, error) {
	vpn, err := s.vpnRepo.GetByUserID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		vpn = models.Vpn{
			UserID: userID,
		}
		switch protocol {
		case "vless":
			vpn.VlessLink = link
		case "trojan":
			vpn.TrojanLink = link
		}
		return s.vpnRepo.Create(vpn)
	}
	if err != nil {
		return models.Vpn{}, err
	}

	switch protocol {
	case "vless":
		vpn.VlessLink = link
	case "trojan":
		vpn.TrojanLink = link
	}
	return s.vpnRepo.Save(vpn)
}

func (s *VPNService) GetVPNByTelegramID(tgID int64) (models.Vpn, error) {
	telegram, err := s.telegramRepo.FindByTgID(tgID)
	if err != nil {
		return models.Vpn{}, err
	}

	return s.vpnRepo.GetByUserID(telegram.UserID)
}

func (s *VPNService) GetVPNLinkByProtocol(tgID int64, protocol string) (string, error) {
	vpn, err := s.GetVPNByTelegramID(tgID)
	if err != nil {
		return "", err
	}

	switch protocol {
	case "vless":
		return vpn.VlessLink, nil
	case "trojan":
		return vpn.TrojanLink, nil
	default:
		return "", ErrUnsupportedProtocol
	}
}
