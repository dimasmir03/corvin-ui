package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) handleCancel(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	state := b.state.GetState(sender.ID)
	if state.Mode == ModeNone {
		return c.Send(msgNoActiveAction)
	}

	b.state.ClearMode(sender.ID)
	b.logger.Info("user mode cancelled", "tg_id", sender.ID, "mode", state.Mode)
	return c.Send(msgActionCancelled, startMenu())
}
