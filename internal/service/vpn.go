package service

import (
	"context"
	"errors"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/jobsvc"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
)

var errVPNServiceNotImplemented = errors.New("vpn service is not implemented")

type VPNService struct {
	telegramRepo *repository.TelegramRepo
	vpnRepo      *repository.VpnRepo
	jobs         *jobsvc.Service
}

type VPNReadyNotification struct{}

func (s *VPNService) GetConfigByTelegramID(ctx context.Context, tgID int64) (*models.Vpn, error) {
	return nil, errVPNServiceNotImplemented
}

func (s *VPNService) RequestCreateConfig(ctx context.Context, tgID int64, protocol string) (*models.JobBatch, error) {
	return nil, errVPNServiceNotImplemented
}

func (s *VPNService) ApplyAgentCreateResult(ctx context.Context, event broker.JobResultEvent) (*VPNReadyNotification, error) {
	return nil, errVPNServiceNotImplemented
}
