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

	admin := b.bot.Group()
	admin.Use(b.adminMiddleware)
	admin.Handle("/getusers", b.withLogging("admin_getusers", b.handleGetUsers))
	admin.Handle("/senduser", b.withLogging("admin_senduser", b.handleSendUser))
	admin.Handle("/send", b.withLogging("admin_send", b.handleSendBroadcast))
}
