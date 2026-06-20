package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) handleMenu(c telebot.Context) error {
	sender := c.Sender()
	if sender != nil {
		b.logger.Info("menu command opened", "tg_id", sender.ID)
	}
	return b.send(c, msgStartMenu, startMenu())
}

func (b *Bot) handleMenuVPN(c telebot.Context) error {
	return b.handleVPN(c)
}

func (b *Bot) handleMenuInstruction(c telebot.Context) error {
	return b.handleInstruction(c)
}

func (b *Bot) handleMenuSupport(c telebot.Context) error {
	return b.handleSupportOpen(c)
}

func (b *Bot) respondWithStub(c telebot.Context, text string) error {
	if err := b.respond(c); err != nil {
		b.logger.Error("telegram callback failed", err)
	}
	if err := b.send(c, text); err != nil {
		b.logger.Error("telegram send failed", err)
		return err
	}
	return nil
}
