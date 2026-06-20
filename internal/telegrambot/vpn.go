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
		return b.send(c, msgVPNFetchFailed)
	}

	vpn, err := b.deps.VPN.GetVPNByTelegramID(sender.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return b.send(c, msgVPNMissing, vpnCreateMenu())
	}
	if err != nil {
		b.logger.Error("telegram vpn handler failed", err, "tg_id", sender.ID)
		return b.send(c, msgVPNFetchFailed)
	}
	if strings.TrimSpace(vpn.VlessLink) == "" && strings.TrimSpace(vpn.TrojanLink) == "" {
		return b.send(c, msgVPNMissing, vpnCreateMenu())
	}

	return b.send(c, msgVPNReady, vpnMenu())
}

func (b *Bot) handleVPNVLESS(c telebot.Context) error {
	return b.sendVPNLink(c, "vless", msgVLESSMissing)
}

func (b *Bot) handleVPNTrojan(c telebot.Context) error {
	return b.sendVPNLink(c, "trojan", msgTrojanMissing)
}

func (b *Bot) handleLink(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram link requested failed", nil, "reason", "sender is nil")
		return b.send(c, msgLinkFetchFailed)
	}

	args := c.Args()
	if len(args) == 0 {
		b.logger.Info("telegram link requested", "tg_id", sender.ID)
		return b.send(c, msgLinkChooseProtocol, linkMenu())
	}

	protocol := strings.ToLower(strings.TrimSpace(args[0]))
	return b.sendLink(c, protocol)
}

func (b *Bot) handleLinkVLESS(c telebot.Context) error {
	return b.sendLink(c, "vless")
}

func (b *Bot) handleLinkTrojan(c telebot.Context) error {
	return b.sendLink(c, "trojan")
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
	if err := b.respond(c); err != nil {
		b.logger.Error("telegram callback failed", err)
	}
	return b.send(c, msgStartMenu, startMenu())
}

func (b *Bot) sendVPNLink(c telebot.Context, protocol string, missingMessage string) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram vpn link handler failed", nil, "reason", "sender is nil", "protocol", protocol)
		return b.send(c, msgVPNFetchFailed)
	}

	link, err := b.deps.VPN.GetVPNLinkByProtocol(sender.ID, protocol)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return b.send(c, missingMessage)
	}
	if err != nil {
		b.logger.Error("telegram vpn link handler failed", err, "tg_id", sender.ID, "protocol", protocol)
		return b.send(c, msgVPNFetchFailed)
	}
	if strings.TrimSpace(link) == "" {
		return b.send(c, missingMessage)
	}

	return b.send(c, link)
}

func (b *Bot) sendLink(c telebot.Context, protocol string) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram link requested failed", nil, "reason", "sender is nil", "protocol", protocol)
		return b.send(c, msgLinkFetchFailed)
	}

	if protocol != "vless" && protocol != "trojan" {
		b.logger.Info("telegram link requested", "tg_id", sender.ID, "protocol", protocol)
		return b.send(c, msgLinkUnsupportedProtocol)
	}

	b.logger.Info("telegram link requested", "tg_id", sender.ID, "protocol", protocol)
	link, err := b.deps.VPN.GetVPNLinkByProtocol(sender.ID, protocol)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		b.logger.Info("telegram link not found", "tg_id", sender.ID, "protocol", protocol)
		return b.send(c, msgLinkMissing)
	}
	if errors.Is(err, service.ErrUnsupportedProtocol) {
		return b.send(c, msgLinkUnsupportedProtocol)
	}
	if err != nil {
		b.logger.Error("telegram link send failed", err, "tg_id", sender.ID, "protocol", protocol)
		return b.send(c, msgLinkFetchFailed)
	}
	if strings.TrimSpace(link) == "" {
		b.logger.Info("telegram link not found", "tg_id", sender.ID, "protocol", protocol)
		return b.send(c, msgLinkMissing)
	}

	if err := b.send(c, formatLinkMessage(protocol, link)); err != nil {
		b.logger.Error("telegram link send failed", err, "tg_id", sender.ID, "protocol", protocol)
		return err
	}
	return nil
}

func formatLinkMessage(protocol string, link string) string {
	switch protocol {
	case "vless":
		return "🔗 Ваша VLESS-ссылка:\n\n" + link
	case "trojan":
		return "🔗 Ваша Trojan-ссылка:\n\n" + link
	default:
		return link
	}
}

func (b *Bot) requestCreateVPN(c telebot.Context, protocol string) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram vpn create request failed", nil, "reason", "sender is nil", "protocol", protocol)
		return b.send(c, msgVPNCreateFailed)
	}

	b.logger.Info("telegram vpn create requested", "tg_id", sender.ID, "protocol", protocol)
	result, err := b.deps.VPN.RequestCreateVPN(service.RequestCreateVPNInput{
		TgID:     sender.ID,
		Protocol: protocol,
	})
	if errors.Is(err, service.ErrVPNAlreadyExists) {
		b.logger.Info("telegram vpn already exists", "tg_id", sender.ID, "protocol", protocol)
		return b.send(c, msgVPNAlreadyExists)
	}
	if errors.Is(err, service.ErrUnsupportedProtocol) {
		b.logger.Error("telegram vpn create request failed", err, "tg_id", sender.ID, "protocol", protocol)
		return b.send(c, msgVPNUnsupportedProtocol)
	}
	if err != nil {
		b.logger.Error("telegram vpn create request failed", err, "tg_id", sender.ID, "protocol", protocol)
		return b.send(c, msgVPNCreateFailed)
	}

	b.logger.Info("telegram vpn create request queued", "tg_id", sender.ID, "protocol", result.Protocol, "batch_id", result.BatchID, "job_id", result.JobID)
	return b.send(c, msgVPNCreateRequested)
}

func (b *Bot) respondToCallback(c telebot.Context) {
	if c.Callback() == nil {
		return
	}
	if err := b.respond(c); err != nil {
		b.logger.Error("telegram callback failed", err)
	}
}
