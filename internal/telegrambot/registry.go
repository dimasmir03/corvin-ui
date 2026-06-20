package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) registerHandlers() {
	if b == nil || b.bot == nil {
		return
	}

	b.logger.Info("telegram registering handlers")
	b.registerUserHandlers()
	b.registerAdminHandlers()
	b.logger.Info("telegram handlers registered")
}

func (b *Bot) registerUserHandlers() {
	b.logger.Info("telegram registering user handlers")

	registerCommand := func(command string, name string, handler telebot.HandlerFunc) {
		b.bot.Handle(command, b.withLogging(name, handler))
		b.logger.Debug("telegram command registered", "command", command, "handler", name)
	}
	registerEndpoint := func(endpoint string, name string, handler telebot.HandlerFunc) {
		b.bot.Handle(endpoint, b.withLogging(name, handler))
		b.logger.Debug("telegram endpoint registered", "endpoint", endpoint, "handler", name)
	}

	registerCommand("/ping", "ping", b.handlePing)
	registerCommand("/start", "start", b.handleStart)
	registerCommand("/id", "id", b.handleID)
	registerCommand("/menu", "menu", b.handleMenu)
	registerCommand("/vpn", "vpn", b.handleVPN)
	registerCommand("/link", "link", b.handleLink)
	registerCommand("/create_vpn", "create_vpn", b.handleCreateVPN)
	registerCommand("/instruction", "instruction", b.handleInstruction)
	registerCommand("/help", "help", b.handleHelp)
	registerCommand("/cancel", "cancel", b.handleCancel)
	registerEndpoint(telebot.OnText, "text", b.handleText)
	registerEndpoint(telebot.OnPhoto, "photo", b.handlePhoto)
	registerEndpoint(telebot.OnCallback, "callback", b.handleCallback)

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
}

func (b *Bot) registerAdminHandlers() {
	b.logger.Info("telegram registering admin handlers")

	admin := b.bot.Group()
	admin.Use(b.adminMiddleware)

	registerCommand := func(command string, name string, handler telebot.HandlerFunc) {
		admin.Handle(command, b.withLogging(name, handler))
		b.logger.Debug("telegram admin command registered", "command", command, "handler", name)
	}

	registerCommand("/getusers", "admin_getusers", b.handleGetUsers)
	registerCommand("/senduser", "admin_senduser", b.handleSendUser)
	registerCommand("/send", "admin_send", b.handleSendBroadcast)

	b.logger.Debug("telegram admin callback route registered", "callback", callbackSupportReply, "handler", "support_reply")
	b.logger.Debug("telegram admin callback route registered", "callback", callbackBroadcastConfirm, "handler", "admin_broadcast_confirm")
	b.logger.Debug("telegram admin callback route registered", "callback", callbackBroadcastCancel, "handler", "admin_broadcast_cancel")
}
