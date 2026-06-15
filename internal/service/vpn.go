package service

import (
	"errors"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/jobsvc"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/utils"

	"gorm.io/gorm"
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
		return "", errors.New("unsupported protocol")
	}
}
