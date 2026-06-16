package telegrambot

import (
	"log"

	telebot "gopkg.in/telebot.v4"
)

func (b *Bot) handleMenuVPN(c telebot.Context) error {
	return respondWithStub(c, msgVPNComingSoon)
}

func (b *Bot) handleMenuInstruction(c telebot.Context) error {
	return respondWithStub(c, msgInstructionComingSoon)
}

func (b *Bot) handleMenuSupport(c telebot.Context) error {
	return respondWithStub(c, msgSupportComingSoon)
}

func respondWithStub(c telebot.Context, text string) error {
	if err := c.Respond(); err != nil {
		log.Printf("telegram callback respond failed: %v", err)
	}
	return c.Send(text)
}
