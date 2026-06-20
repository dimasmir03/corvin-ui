package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) handleText(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	mode := b.state.GetMode(sender.ID)
	b.logger.Info("telegram text received", "tg_id", sender.ID, "mode", mode, "text_len", len(c.Text()))

	switch mode {
	case ModeSupport:
		return b.handleSupportText(c)
	case ModeSupportReply:
		return b.handleSupportReplyText(c)
	default:
		return b.handleUnknownText(c)
	}
}

func (b *Bot) handleUnknownText(c telebot.Context) error {
	return b.send(c, msgUnknownText, startMenu())
}
