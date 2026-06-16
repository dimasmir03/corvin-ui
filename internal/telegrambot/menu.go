package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) handleMenuVPN(c telebot.Context) error {
	return b.respondWithStub(c, msgVPNComingSoon)
}

func (b *Bot) handleMenuInstruction(c telebot.Context) error {
	return b.respondWithStub(c, msgInstructionComingSoon)
}

func (b *Bot) handleMenuSupport(c telebot.Context) error {
	return b.respondWithStub(c, msgSupportComingSoon)
}

func (b *Bot) respondWithStub(c telebot.Context, text string) error {
	if err := c.Respond(); err != nil {
		b.logger.Errorf("telegram callback respond failed: %v", err)
	}
	if err := c.Send(text); err != nil {
		b.logger.Errorf("telegram send failed: %v", err)
		return err
	}
	return nil
}
