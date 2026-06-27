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
	return b.sendLink(c, "vless")
}

func (b *Bot) handleVPNTrojan(c telebot.Context) error {
	return b.sendLink(c, "trojan")
}

func (b *Bot) handleLink(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram link requested failed", nil, "reason", "sender is nil")
		return b.send(c, msgLinkFetchFailed)
	}

	args := c.Args()
	if len(args) == 0 {
		return b.sendLinkOverview(c)
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

func (b *Bot) sendLinkOverview(c telebot.Context) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram link overview failed", nil, "component", "telegrambot", "handler", "link_overview", "reason", "sender is nil")
		return b.send(c, msgLinkFetchFailed)
	}

	b.logger.Info("telegram link overview requested", "component", "telegrambot", "handler", "link_overview", "tg_id", sender.ID)
	overview, err := b.deps.VPN.GetLinkOverview(sender.ID)
	if err != nil {
		b.logger.Error("telegram link overview failed", err, "component", "telegrambot", "handler", "link_overview", "tg_id", sender.ID, "reason", "internal_error")
		return b.send(c, msgLinkFetchFailed)
	}

	vless := overview.Profiles["vless"]
	trojan := overview.Profiles["trojan"]
	hasVless := linkProfileAvailable(vless)
	hasTrojan := linkProfileAvailable(trojan)
	message, markup := b.linkOverviewResponse(overview, hasVless, hasTrojan)
	b.logger.Info("telegram link overview sent", "component", "telegrambot", "handler", "link_overview", "tg_id", sender.ID, "reason", overview.Reason, "has_vless", hasVless, "has_trojan", hasTrojan, "vless_status", vless.Status, "trojan_status", trojan.Status)
	if markup != nil {
		return b.send(c, message, markup)
	}
	return b.send(c, message)
}

func (b *Bot) sendLink(c telebot.Context, protocol string) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram protocol link failed", nil, "component", "telegrambot", "handler", "protocol_link", "reason", "sender is nil", "protocol", protocol)
		return b.send(c, msgLinkFetchFailed)
	}

	protocol = strings.ToLower(strings.TrimSpace(protocol))
	b.logger.Info("telegram protocol link requested", "component", "telegrambot", "handler", "protocol_link", "tg_id", sender.ID, "protocol", protocol)
	if protocol != "vless" && protocol != "trojan" {
		b.logger.Info("telegram protocol link sent", "component", "telegrambot", "handler", "protocol_link", "tg_id", sender.ID, "protocol", protocol, "reason", "unsupported_protocol")
		return b.send(c, msgLinkUnsupportedProtocol)
	}

	b.logger.Info("telegram protocol link service call started", "component", "telegrambot", "handler", "protocol_link", "tg_id", sender.ID, "protocol", protocol)
	result, err := b.deps.VPN.GetProtocolLink(sender.ID, protocol)
	if errors.Is(err, service.ErrUnsupportedProtocol) {
		b.logger.Info("telegram protocol link sent", "component", "telegrambot", "handler", "protocol_link", "tg_id", sender.ID, "protocol", protocol, "reason", "unsupported_protocol")
		return b.send(c, msgLinkUnsupportedProtocol)
	}
	if err != nil {
		b.logger.Error("telegram protocol link failed", err, "component", "telegrambot", "handler", "protocol_link", "tg_id", sender.ID, "protocol", protocol, "reason", "internal_error")
		return b.send(c, msgLinkFetchFailed)
	}
	if result.Reason != "protocol_link_found" || strings.TrimSpace(result.Link) == "" {
		message := protocolMissingMessage(protocol, result.Reason)
		b.logger.Info("telegram protocol link sent", "component", "telegrambot", "handler", "protocol_link", "tg_id", sender.ID, "protocol", protocol, "status", result.Status, "reason", result.Reason)
		return b.send(c, message)
	}

	if err := b.send(c, formatLinkMessage(protocol, result.Link), telebot.ModeMarkdown); err != nil {
		b.logger.Error("telegram protocol link send failed", err, "component", "telegrambot", "handler", "protocol_link", "tg_id", sender.ID, "protocol", protocol)
		return err
	}
	b.logger.Info("telegram protocol link sent", "component", "telegrambot", "handler", "protocol_link", "tg_id", sender.ID, "protocol", protocol, "status", result.Status, "reason", result.Reason)
	return nil
}

func (b *Bot) linkOverviewResponse(overview service.LinkOverviewResult, hasVless bool, hasTrojan bool) (string, *telebot.ReplyMarkup) {
	switch overview.Reason {
	case "both_links_available":
		return msgLinkChooseProtocol, linkOverviewMenu(true, true, false)
	case "single_link_available":
		if hasVless {
			return fmt.Sprintf(msgLinkSingleAvailable, "VLESS", "Trojan"), linkOverviewMenu(true, false, false)
		}
		return fmt.Sprintf(msgLinkSingleAvailable, "Trojan", "VLESS"), linkOverviewMenu(false, true, false)
	case "vpn_not_configured", "telegram_user_not_found", "vpn_profiles_not_found":
		return msgLinkVPNNotConfigured, linkOverviewMenu(false, false, true)
	case "profiles_pending":
		return msgLinkProfilesPending, nil
	case "profiles_failed":
		return msgLinkProfilesFailed, nil
	default:
		return linkOverviewStatusMessage(overview), nil
	}
}

func linkProfileAvailable(profile service.LinkProfileView) bool {
	return profile.Usable && strings.TrimSpace(profile.FinalLink) != ""
}

func linkOverviewStatusMessage(overview service.LinkOverviewResult) string {
	vless := overview.Profiles["vless"]
	trojan := overview.Profiles["trojan"]
	return fmt.Sprintf("VPN пока не готов.\n\nVLESS: %s\nTrojan: %s", linkStatusLabel(vless), linkStatusLabel(trojan))
}

func linkStatusLabel(profile service.LinkProfileView) string {
	if !profile.Exists {
		return "не создан"
	}
	switch profile.Status {
	case "active", "partial":
		if strings.TrimSpace(profile.FinalLink) != "" {
			return "готов"
		}
		return "ссылка не готова"
	case "pending":
		return "создаётся"
	case "failed":
		return "ошибка"
	default:
		return "не готов"
	}
}

func protocolMissingMessage(protocol string, reason string) string {
	switch protocol {
	case "vless":
		switch reason {
		case "protocol_pending":
			return msgVLESSPending
		case "protocol_failed":
			return msgVLESSFailed
		default:
			return msgVLESSNotCreated
		}
	case "trojan":
		switch reason {
		case "protocol_pending":
			return msgTrojanPending
		case "protocol_failed":
			return msgTrojanFailed
		default:
			return msgTrojanNotCreated
		}
	default:
		return msgLinkUnsupportedProtocol
	}
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
