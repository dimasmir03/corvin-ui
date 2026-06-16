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

	b.bot.Handle(&btnMenuVPN, b.handleMenuVPN)
	b.bot.Handle(&btnMenuInstruction, b.handleMenuInstruction)
	b.bot.Handle(&btnMenuSupport, b.handleMenuSupport)
}
