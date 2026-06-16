package telegrambot

import (
	"errors"
	"strings"
	"vpnpanel/internal/service"

	telebot "gopkg.in/telebot.v4"
	"gorm.io/gorm"
)

func (b *Bot) handleVPN(c telebot.Context) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram vpn handler failed", nil, "reason", "sender is nil")
		return c.Send(msgVPNFetchFailed)
	}

	vpn, err := b.deps.VPN.GetVPNByTelegramID(sender.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Send(msgVPNMissing, vpnCreateMenu())
	}
	if err != nil {
		b.logger.Error("telegram vpn handler failed", err, "tg_id", sender.ID)
		return c.Send(msgVPNFetchFailed)
	}
	if strings.TrimSpace(vpn.VlessLink) == "" && strings.TrimSpace(vpn.TrojanLink) == "" {
		return c.Send(msgVPNMissing, vpnCreateMenu())
	}

	return c.Send(msgVPNReady, vpnMenu())
}

func (b *Bot) handleVPNVLESS(c telebot.Context) error {
	return b.sendVPNLink(c, "vless", msgVLESSMissing)
}

func (b *Bot) handleVPNTrojan(c telebot.Context) error {
	return b.sendVPNLink(c, "trojan", msgTrojanMissing)
}

func (b *Bot) handleCreateVPN(c telebot.Context) error {
	return b.requestCreateVPN(c, "vless")
}

func (b *Bot) handleCreateVLESS(c telebot.Context) error {
	return b.requestCreateVPN(c, "vless")
}

func (b *Bot) handleCreateTrojan(c telebot.Context) error {
	return b.requestCreateVPN(c, "trojan")
}

func (b *Bot) handleVPNBack(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		b.logger.Error("telegram callback failed", err)
	}
	return c.Send(msgStartMenu, startMenu())
}

func (b *Bot) sendVPNLink(c telebot.Context, protocol string, missingMessage string) error {
	if err := c.Respond(); err != nil {
		b.logger.Error("telegram callback failed", err)
	}

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram vpn link handler failed", nil, "reason", "sender is nil", "protocol", protocol)
		return c.Send(msgVPNFetchFailed)
	}

	link, err := b.deps.VPN.GetVPNLinkByProtocol(sender.ID, protocol)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Send(missingMessage)
	}
	if err != nil {
		b.logger.Error("telegram vpn link handler failed", err, "tg_id", sender.ID, "protocol", protocol)
		return c.Send(msgVPNFetchFailed)
	}
	if strings.TrimSpace(link) == "" {
		return c.Send(missingMessage)
	}

	return c.Send(link)
}

func (b *Bot) requestCreateVPN(c telebot.Context, protocol string) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram vpn create request failed", nil, "reason", "sender is nil", "protocol", protocol)
		return c.Send(msgVPNCreateFailed)
	}

	b.logger.Info("telegram vpn create requested", "tg_id", sender.ID, "protocol", protocol)
	result, err := b.deps.VPN.RequestCreateVPN(service.RequestCreateVPNInput{
		TgID:     sender.ID,
		Protocol: protocol,
	})
	if errors.Is(err, service.ErrVPNAlreadyExists) {
		b.logger.Info("telegram vpn already exists", "tg_id", sender.ID, "protocol", protocol)
		return c.Send(msgVPNAlreadyExists)
	}
	if errors.Is(err, service.ErrUnsupportedProtocol) {
		b.logger.Error("telegram vpn create request failed", err, "tg_id", sender.ID, "protocol", protocol)
		return c.Send(msgVPNUnsupportedProtocol)
	}
	if err != nil {
		b.logger.Error("telegram vpn create request failed", err, "tg_id", sender.ID, "protocol", protocol)
		return c.Send(msgVPNCreateFailed)
	}

	b.logger.Info("telegram vpn create request queued", "tg_id", sender.ID, "protocol", result.Protocol, "batch_id", result.BatchID, "job_id", result.JobID)
	return c.Send(msgVPNCreateRequested)
}

func (b *Bot) respondToCallback(c telebot.Context) {
	if c.Callback() == nil {
		return
	}
	if err := c.Respond(); err != nil {
		b.logger.Error("telegram callback failed", err)
	}
}
