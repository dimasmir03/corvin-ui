package telegrambot

import telebot "gopkg.in/telebot.v4"

func (b *Bot) handleCancel(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	state := b.state.GetState(sender.ID)
	hasInstruction := b.state.HasInstruction(sender.ID)
	if state.Mode == ModeNone && !hasInstruction {
		return c.Send(msgNoActiveAction)
	}

	b.state.ClearMode(sender.ID)
	b.state.ClearInstruction(sender.ID)
	b.logger.Info("user action cancelled", "tg_id", sender.ID, "mode", state.Mode)
	return c.Send(msgActionCancelled, startMenu())
}
