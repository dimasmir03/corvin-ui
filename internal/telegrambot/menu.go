package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) handleMenuVPN(c telebot.Context) error {
	return b.handleVPN(c)
}

func (b *Bot) handleMenuInstruction(c telebot.Context) error {
	return b.handleInstruction(c)
}

func (b *Bot) handleMenuSupport(c telebot.Context) error {
	return b.handleSupport(c)
}

func (b *Bot) respondWithStub(c telebot.Context, text string) error {
	if err := c.Respond(); err != nil {
		b.logger.Error("telegram callback failed", err)
	}
	if err := c.Send(text); err != nil {
		b.logger.Error("telegram send failed", err)
		return err
	}
	return nil
}
