package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) registerHandlers() {
	if b == nil || b.bot == nil {
		return
	}

	b.logger.Info("telegram registering handlers")
	b.bot.Use(telebot.MiddlewareFunc(b.maintenanceMiddleware))
	b.bot.Use(telebot.MiddlewareFunc(b.ensureTelegramUserMiddleware))

	b.bot.Handle("/ping", b.withLogging("ping", b.handlePing))
	b.bot.Handle("/start", b.withLogging("start", b.handleStart))
	b.bot.Handle("/id", b.withLogging("id", b.handleID))
	b.bot.Handle("/menu", b.withLogging("menu", b.handleMenu))
	b.bot.Handle("/vpn", b.withLogging("vpn", b.handleVPN))
	b.bot.Handle("/link", b.withLogging("link", b.handleLink))
	b.bot.Handle("/create_vpn", b.withLogging("create_vpn", b.handleCreateVPN))
	b.bot.Handle("/instruction", b.withLogging("instruction", b.handleInstruction))
	b.bot.Handle("/help", b.withLogging("help", b.handleHelp))
	b.bot.Handle("/cancel", b.withLogging("cancel", b.handleCancel))
	b.bot.Handle(telebot.OnText, b.withLogging("text", b.handleText))
	b.bot.Handle(telebot.OnPhoto, b.withLogging("photo", b.handlePhoto))
	b.bot.Handle(telebot.OnCallback, b.withLogging("callback", b.handleCallback))

	b.logger.Debug("telegram command registered", "command", "/ping", "handler", "ping")
	b.logger.Debug("telegram command registered", "command", "/start", "handler", "start")
	b.logger.Debug("telegram command registered", "command", "/id", "handler", "id")
	b.logger.Debug("telegram command registered", "command", "/menu", "handler", "menu")
	b.logger.Debug("telegram command registered", "command", "/vpn", "handler", "vpn")
	b.logger.Debug("telegram command registered", "command", "/link", "handler", "link")
	b.logger.Debug("telegram command registered", "command", "/create_vpn", "handler", "create_vpn")
	b.logger.Debug("telegram command registered", "command", "/instruction", "handler", "instruction")
	b.logger.Debug("telegram command registered", "command", "/help", "handler", "help")
	b.logger.Debug("telegram command registered", "command", "/cancel", "handler", "cancel")
	b.logger.Debug("telegram endpoint registered", "endpoint", telebot.OnText, "handler", "text")
	b.logger.Debug("telegram endpoint registered", "endpoint", telebot.OnPhoto, "handler", "photo")
	b.logger.Debug("telegram endpoint registered", "endpoint", telebot.OnCallback, "handler", "callback")

	b.logger.Debug("telegram callback route registered", "callback", callbackMenuVPN, "handler", "menu_vpn")
	b.logger.Debug("telegram callback route registered", "callback", callbackMenuInstruction, "handler", "menu_instruction")
	b.logger.Debug("telegram callback route registered", "callback", callbackMenuSupport, "handler", "support_open")
	b.logger.Debug("telegram callback route registered", "callback", callbackVPNVLESS, "handler", "vpn_vless")
	b.logger.Debug("telegram callback route registered", "callback", callbackVPNTrojan, "handler", "vpn_trojan")
	b.logger.Debug("telegram callback route registered", "callback", callbackCreateVLESS, "handler", "vpn_create_vless")
	b.logger.Debug("telegram callback route registered", "callback", callbackCreateTrojan, "handler", "vpn_create_trojan")
	b.logger.Debug("telegram callback route registered", "callback", callbackMainMenu, "handler", "main_menu")
	b.logger.Debug("telegram callback route registered", "callback", callbackLinkVLESS, "handler", "link_vless")
	b.logger.Debug("telegram callback route registered", "callback", callbackLinkTrojan, "handler", "link_trojan")
	b.logger.Debug("telegram callback route registered", "callback", callbackInstructionNext, "handler", "instruction_next")
	b.logger.Debug("telegram callback route registered", "callback", callbackInstructionPrev, "handler", "instruction_prev")
	b.logger.Debug("telegram callback route registered", "callback", callbackInstructionMenu, "handler", "instruction_menu")
	b.logger.Debug("telegram callback route registered", "callback", callbackSupportCancel, "handler", "support_cancel")

	admin := b.bot.Group()
	admin.Use(b.adminMiddleware)
	admin.Handle("/getusers", b.withLogging("admin_getusers", b.handleGetUsers))
	admin.Handle("/senduser", b.withLogging("admin_senduser", b.handleSendUser))
	admin.Handle("/send", b.withLogging("admin_send", b.handleSendBroadcast))

	b.logger.Debug("telegram admin command registered", "command", "/getusers", "handler", "admin_getusers")
	b.logger.Debug("telegram admin command registered", "command", "/senduser", "handler", "admin_senduser")
	b.logger.Debug("telegram admin command registered", "command", "/send", "handler", "admin_send")
	b.logger.Debug("telegram admin callback route registered", "callback", callbackSupportReply, "handler", "support_reply")
	b.logger.Debug("telegram admin callback route registered", "callback", callbackBroadcastConfirm, "handler", "admin_broadcast_confirm")
	b.logger.Debug("telegram admin callback route registered", "callback", callbackBroadcastCancel, "handler", "admin_broadcast_cancel")
	b.logger.Info("telegram handlers registered")
}
