package telegrambot

import (
	"errors"
	"fmt"
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

	b.logger.Info("telegram vpn requested", "component", "telegrambot", "handler", "vpn", "tg_id", sender.ID)
	vpn, err := b.deps.VPN.GetVPNByTelegramID(sender.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		b.logger.Info("telegram vpn response sent", "component", "telegrambot", "handler", "vpn", "tg_id", sender.ID, "reason", "vpn_client_not_found")
		return b.send(c, msgVPNMissing, vpnCreateMenu(false, false))
	}
	if err != nil {
		b.logger.Error("telegram vpn handler failed", err, "tg_id", sender.ID)
		return b.send(c, msgVPNFetchFailed)
	}
	if strings.TrimSpace(vpn.VlessLink) == "" && strings.TrimSpace(vpn.TrojanLink) == "" {
		b.logger.Info("telegram vpn response sent", "component", "telegrambot", "handler", "vpn", "tg_id", sender.ID, "vpn_id", vpn.ID, "reason", "no_usable_profiles")
		return b.send(c, msgVPNMissing, vpnCreateMenu(hasVPNLink(vpn.VlessLink), hasVPNLink(vpn.TrojanLink)))
	}

	b.logger.Info("telegram vpn response sent", "component", "telegrambot", "handler", "vpn", "tg_id", sender.ID, "vpn_id", vpn.ID, "reason", "usable_profiles_found", "has_vless", hasVPNLink(vpn.VlessLink), "has_trojan", hasVPNLink(vpn.TrojanLink))
	return b.send(c, fmt.Sprintf(msgVPNReady, buildVPNStatus(vpn.VlessLink, vpn.TrojanLink)), vpnMenu(hasVPNLink(vpn.VlessLink), hasVPNLink(vpn.TrojanLink)))
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
	return b.requestCreateVPN(c, "all")
}

func (b *Bot) handleCreateVLESS(c telebot.Context) error {
	return b.requestCreateVPN(c, "vless")
}

func (b *Bot) handleCreateTrojan(c telebot.Context) error {
	return b.requestCreateVPN(c, "trojan")
}

func (b *Bot) handleMainMenu(c telebot.Context) error {
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

	return b.send(c, formatLinkMessage(protocol, link), telebot.ModeMarkdown)
}

func (b *Bot) sendLink(c telebot.Context, protocol string) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram link requested failed", nil, "reason", "sender is nil", "protocol", protocol)
		return b.send(c, msgLinkFetchFailed)
	}

	if protocol != "vless" && protocol != "trojan" {
		b.logger.Info("telegram link requested", "component", "telegrambot", "handler", "link", "tg_id", sender.ID, "protocol", protocol)
		b.logger.Info("telegram link response sent", "component", "telegrambot", "handler", "link", "tg_id", sender.ID, "protocol", protocol, "reason", "unsupported_protocol")
		return b.send(c, msgLinkUnsupportedProtocol)
	}

	b.logger.Info("telegram link requested", "component", "telegrambot", "handler", "link", "tg_id", sender.ID, "protocol", protocol)
	b.logger.Info("telegram link service call started", "component", "telegrambot", "handler", "link", "tg_id", sender.ID, "protocol", protocol)
	link, err := b.deps.VPN.GetVPNLinkByProtocol(sender.ID, protocol)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		b.logger.Info("telegram link response sent", "component", "telegrambot", "handler", "link", "tg_id", sender.ID, "protocol", protocol, "reason", "vpn_client_not_found")
		return b.send(c, msgLinkMissing)
	}
	if errors.Is(err, service.ErrUnsupportedProtocol) {
		b.logger.Info("telegram link response sent", "component", "telegrambot", "handler", "link", "tg_id", sender.ID, "protocol", protocol, "reason", "unsupported_protocol")
		return b.send(c, msgLinkUnsupportedProtocol)
	}
	if err != nil {
		b.logger.Error("telegram link send failed", err, "component", "telegrambot", "handler", "link", "tg_id", sender.ID, "protocol", protocol, "reason", "internal_error")
		return b.send(c, msgLinkFetchFailed)
	}
	if strings.TrimSpace(link) == "" {
		b.logger.Info("telegram link response sent", "component", "telegrambot", "handler", "link", "tg_id", sender.ID, "protocol", protocol, "reason", "no_usable_profiles")
		return b.send(c, msgLinkMissing)
	}

	if err := b.send(c, formatLinkMessage(protocol, link), telebot.ModeMarkdown); err != nil {
		b.logger.Error("telegram link send failed", err, "tg_id", sender.ID, "protocol", protocol)
		return err
	}
	b.logger.Info("telegram link response sent", "component", "telegrambot", "handler", "link", "tg_id", sender.ID, "protocol", protocol, "reason", "usable_profiles_found")
	return nil
}

func buildVPNStatus(vlessLink string, trojanLink string) string {
	status := []string{}
	if hasVPNLink(vlessLink) {
		status = append(status, "✅ Основной")
	} else {
		status = append(status, "❌ Основной")
	}
	if hasVPNLink(trojanLink) {
		status = append(status, "✅ Обход")
	} else {
		status = append(status, "❌ Обход")
	}
	return fmt.Sprintf("%s | %s", status[0], status[1])
}

func hasVPNLink(link string) bool {
	trimmed := strings.TrimSpace(link)
	return trimmed != "" && trimmed != "null"
}

func formatLinkMessage(protocol string, link string) string {
	switch protocol {
	case "vless":
		return fmt.Sprintf("🔗 **%s ссылка:**\n```\n%s\n```", "основная VLESS", link)
	case "trojan":
		return fmt.Sprintf("🔗 **%s ссылка:**\n```\n%s\n```", "обхода Trojan", link)
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
		if vpn, getErr := b.deps.VPN.GetVPNByTelegramID(sender.ID); getErr == nil {
			return b.send(c, fmt.Sprintf(msgVPNAlreadyExists, buildVPNStatus(vpn.VlessLink, vpn.TrojanLink)), vpnMenu(hasVPNLink(vpn.VlessLink), hasVPNLink(vpn.TrojanLink)))
		}
		return b.send(c, "✅ VPN уже создан", vpnMenu(false, false))
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
