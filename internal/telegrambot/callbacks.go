package telegrambot

import (
	"strings"

	telebot "gopkg.in/telebot.v4"
)

const msgUnknownCallback = "Неизвестная команда"

func (b *Bot) handleCallback(c telebot.Context) error {
	callback := c.Callback()
	if callback == nil {
		b.logger.Warn("telegram callback missing")
		return nil
	}

	senderID := int64(0)
	if sender := c.Sender(); sender != nil {
		senderID = sender.ID
	}

	key := callbackKey(callback)
	b.logger.Info("telegram callback received", "tg_id", senderID, "callback_unique", callback.Unique, "callback_data", callback.Data, "callback_key", key)

	handlerName, handler, adminOnly := b.routeCallback(key)
	if handler == nil {
		b.logger.Warn("telegram unknown callback", "tg_id", senderID, "callback_unique", callback.Unique, "callback_data", callback.Data, "callback_key", key)
		return b.respond(c, &telebot.CallbackResponse{Text: msgUnknownCallback})
	}

	if adminOnly && !b.isAdmin(senderID) {
		b.logger.Warn("telegram admin callback ignored for non-admin", "tg_id", senderID, "callback", key)
		return nil
	}

	b.logger.Info("telegram callback routed", "tg_id", senderID, "callback", key, "handler", handlerName)
	if err := b.withLogging(handlerName, handler)(c); err != nil {
		b.logger.Error("telegram callback failed", err, "tg_id", senderID, "callback", key, "handler", handlerName)
		return err
	}

	b.logger.Info("telegram callback finished", "tg_id", senderID, "callback", key, "handler", handlerName)
	return nil
}

func (b *Bot) routeCallback(key string) (string, telebot.HandlerFunc, bool) {
	switch {
	case key == callbackMenuVPN:
		return "menu_vpn", b.handleMenuVPN, false
	case key == callbackMenuInstruction:
		return "menu_instruction", b.handleMenuInstruction, false
	case key == callbackMenuSupport:
		return "support_open", b.handleSupportOpen, false
	case key == callbackVPNVLESS:
		return "vpn_vless", b.handleVPNVLESS, false
	case key == callbackVPNTrojan:
		return "vpn_trojan", b.handleVPNTrojan, false
	case key == callbackCreateVLESS:
		return "vpn_create_vless", b.handleCreateVLESS, false
	case key == callbackCreateTrojan:
		return "vpn_create_trojan", b.handleCreateTrojan, false
	case key == callbackMainMenu:
		return "main_menu", b.handleMainMenu, false
	case key == callbackLinkVLESS:
		return "link_vless", b.handleLinkVLESS, false
	case key == callbackLinkTrojan:
		return "link_trojan", b.handleLinkTrojan, false
	case key == callbackInstructionNext:
		return "instruction_next", b.handleInstructionNext, false
	case key == callbackInstructionPrev:
		return "instruction_prev", b.handleInstructionPrev, false
	case key == callbackInstructionMenu:
		return "instruction_menu", b.handleInstructionMenu, false
	case key == callbackSupportCancel:
		return "support_cancel", b.handleSupportCancel, false
	case key == callbackSupportReply || strings.HasPrefix(key, callbackSupportReply+":"):
		return "support_reply", b.handleSupportReplyStart, true
	case key == callbackBroadcastConfirm:
		return "admin_broadcast_confirm", b.handleBroadcastConfirm, true
	case key == callbackBroadcastCancel:
		return "admin_broadcast_cancel", b.handleBroadcastCancel, true
	default:
		return "", nil, false
	}
}

func callbackKey(callback *telebot.Callback) string {
	if callback == nil {
		return ""
	}
	if strings.TrimSpace(callback.Unique) != "" {
		return strings.TrimSpace(callback.Unique)
	}
	return strings.TrimSpace(callback.Data)
}
