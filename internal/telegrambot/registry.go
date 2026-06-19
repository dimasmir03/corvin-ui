package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) registerHandlers() {
	if b == nil || b.bot == nil {
		return
	}

	b.bot.Handle("/ping", func(c telebot.Context) error {
		return c.Send("ok")
	})
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/id", b.handleID)
	b.bot.Handle("/vpn", b.handleVPN)
	b.bot.Handle("/create_vpn", b.handleCreateVPN)
	b.bot.Handle("/instruction", b.handleInstruction)
	b.bot.Handle("/help", b.handleSupport)
	b.bot.Handle(telebot.OnText, b.handleText)

	b.bot.Handle(&btnMenuVPN, b.handleMenuVPN)
	b.bot.Handle(&btnMenuInstruction, b.handleMenuInstruction)
	b.bot.Handle(&btnMenuSupport, b.handleSupport)
	b.bot.Handle(&btnVPNVLESS, b.handleVPNVLESS)
	b.bot.Handle(&btnVPNTrojan, b.handleVPNTrojan)
	b.bot.Handle(&btnCreateVLESS, b.handleCreateVLESS)
	b.bot.Handle(&btnCreateTrojan, b.handleCreateTrojan)
	b.bot.Handle(&btnVPNBack, b.handleVPNBack)
	b.bot.Handle(&btnInstructionNext, b.handleInstructionNext)
	b.bot.Handle(&btnInstructionPrev, b.handleInstructionPrev)
	b.bot.Handle(&btnInstructionMenu, b.handleInstructionMenu)
	b.bot.Handle(&btnSupportCancel, b.handleSupportCancel)
	b.bot.Handle(&btnSupportReply, b.handleSupportReplyStart)
}
