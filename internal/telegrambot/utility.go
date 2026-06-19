package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) handlePing(c telebot.Context) error {
	sender := c.Sender()
	if sender != nil {
		b.logger.Debug("telegram ping requested", "tg_id", sender.ID)
	}
	return c.Send(msgPingOK)
}
