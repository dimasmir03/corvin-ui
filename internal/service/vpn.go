package service

import (
	"errors"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
)

type VPNService struct {
	vpnRepo      *repository.VpnRepo
	telegramRepo *repository.TelegramRepo
}

func NewVPNService(
	vpnRepo *repository.VpnRepo,
	telegramRepo *repository.TelegramRepo,
) *VPNService {
	return &VPNService{
		vpnRepo:      vpnRepo,
		telegramRepo: telegramRepo,
	}
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
