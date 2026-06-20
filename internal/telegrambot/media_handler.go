package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) handlePhoto(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	mode := b.state.GetMode(sender.ID)
	hasCaption := false
	if message := c.Message(); message != nil {
		hasCaption = message.Caption != ""
	}
	b.logger.Info("telegram photo received", "tg_id", sender.ID, "mode", mode, "has_caption", hasCaption)

	switch mode {
	case ModeSupport:
		return b.handleSupportPhoto(c)
	default:
		return b.handleUnknownPhoto(c)
	}
}

func (b *Bot) handleUnknownPhoto(c telebot.Context) error {
	return b.send(c, msgSupportPhotoPrompt, startMenu())
}
