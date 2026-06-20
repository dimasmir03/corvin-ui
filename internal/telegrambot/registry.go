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
	registerCallback := func(button *telebot.Btn, callback string, name string, handler telebot.HandlerFunc) {
		b.bot.Handle(button, b.withLogging(name, handler))
		b.logger.Debug("telegram callback registered", "callback", callback, "handler", name)
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

	registerCallback(&btnMenuVPN, callbackMenuVPN, "menu_vpn", b.handleMenuVPN)
	registerCallback(&btnMenuInstruction, callbackMenuInstruction, "menu_instruction", b.handleMenuInstruction)
	registerCallback(&btnMenuSupport, callbackMenuSupport, "support_open", b.handleSupportOpen)
	registerCallback(&btnVPNVLESS, callbackVPNVLESS, "vpn_vless", b.handleVPNVLESS)
	registerCallback(&btnVPNTrojan, callbackVPNTrojan, "vpn_trojan", b.handleVPNTrojan)
	registerCallback(&btnCreateVLESS, callbackCreateVLESS, "vpn_create_vless", b.handleCreateVLESS)
	registerCallback(&btnCreateTrojan, callbackCreateTrojan, "vpn_create_trojan", b.handleCreateTrojan)
	registerCallback(&btnVPNBack, callbackVPNBack, "vpn_back", b.handleVPNBack)
	registerCallback(&btnLinkVLESS, callbackLinkVLESS, "link_vless", b.handleLinkVLESS)
	registerCallback(&btnLinkTrojan, callbackLinkTrojan, "link_trojan", b.handleLinkTrojan)
	registerCallback(&btnInstructionNext, callbackInstructionNext, "instruction_next", b.handleInstructionNext)
	registerCallback(&btnInstructionPrev, callbackInstructionPrev, "instruction_prev", b.handleInstructionPrev)
	registerCallback(&btnInstructionMenu, callbackInstructionMenu, "instruction_menu", b.handleInstructionMenu)
	registerCallback(&btnSupportCancel, callbackSupportCancel, "support_cancel", b.handleSupportCancel)
}

func (b *Bot) registerAdminHandlers() {
	b.logger.Info("telegram registering admin handlers")

	admin := b.bot.Group()
	admin.Use(b.adminMiddleware)

	registerCommand := func(command string, name string, handler telebot.HandlerFunc) {
		admin.Handle(command, b.withLogging(name, handler))
		b.logger.Debug("telegram admin command registered", "command", command, "handler", name)
	}
	registerCallback := func(button *telebot.Btn, callback string, name string, handler telebot.HandlerFunc) {
		admin.Handle(button, b.withLogging(name, handler))
		b.logger.Debug("telegram admin callback registered", "callback", callback, "handler", name)
	}

	registerCommand("/getusers", "admin_getusers", b.handleGetUsers)
	registerCommand("/senduser", "admin_senduser", b.handleSendUser)
	registerCommand("/send", "admin_send", b.handleSendBroadcast)
	registerCallback(&btnSupportReply, callbackSupportReply, "support_reply", b.handleSupportReplyStart)
	registerCallback(&btnBroadcastConfirm, callbackBroadcastConfirm, "admin_broadcast_confirm", b.handleBroadcastConfirm)
	registerCallback(&btnBroadcastCancel, callbackBroadcastCancel, "admin_broadcast_cancel", b.handleBroadcastCancel)
}
