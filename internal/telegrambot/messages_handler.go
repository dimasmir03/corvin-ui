package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) handleText(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	switch b.state.GetMode(sender.ID) {
	case ModeSupport:
		return b.handleSupportText(c)
	case ModeSupportReply:
		return b.handleSupportReplyText(c)
	default:
		return b.handleUnknownText(c)
	}
}

func (b *Bot) handleUnknownText(c telebot.Context) error {
	return c.Send(msgUnknownText, startMenu())
}
