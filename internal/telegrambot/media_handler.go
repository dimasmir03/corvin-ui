package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) handlePhoto(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	switch b.state.GetMode(sender.ID) {
	case ModeSupport:
		return b.handleSupportPhoto(c)
	default:
		return b.handleUnknownPhoto(c)
	}
}

func (b *Bot) handleUnknownPhoto(c telebot.Context) error {
	return c.Send(msgSupportPhotoPrompt, startMenu())
}
