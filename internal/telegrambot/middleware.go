package telegrambot

import (
	"fmt"

	telebot "gopkg.in/telebot.v4"
)

func (b *Bot) withLogging(name string, next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		senderID := int64(0)
		if sender := c.Sender(); sender != nil {
			senderID = sender.ID
		}

		args := []any{"handler", name, "tg_id", senderID}
		if callback := c.Callback(); callback != nil {
			args = append(args,
				"callback_unique", callback.Unique,
				"callback_data", callback.Data,
				"callback_id", callback.ID,
			)
			if callback.Message != nil {
				args = append(args, "message_id", callback.Message.ID)
			}
			if callback.MessageID != "" {
				args = append(args, "inline_message_id", callback.MessageID)
			}
		}
		if message := c.Message(); message != nil {
			args = append(args, "message_id", message.ID)
			if message.Text != "" {
				args = append(args, "text_len", len(message.Text))
			}
			if message.Caption != "" {
				args = append(args, "caption_len", len(message.Caption))
			}
		}

		b.logger.Info("telegram handler started", args...)
		err := next(c)
		if err != nil {
			b.logger.Error("telegram handler failed", err, args...)
			return err
		}

		b.logger.Info("telegram handler finished", "handler", name, "tg_id", senderID)
		return nil
	}
}

func (b *Bot) send(c telebot.Context, what any, opts ...any) error {
	err := c.Send(what, opts...)
	args := b.contextLogArgs(c)
	args = append(args, "content_type", fmt.Sprintf("%T", what))
	if err != nil {
		b.logger.Error("telegram send failed", err, args...)
		return err
	}
	b.logger.Debug("telegram send succeeded", args...)
	return nil
}

func (b *Bot) edit(c telebot.Context, what any, opts ...any) error {
	err := c.Edit(what, opts...)
	args := b.contextLogArgs(c)
	args = append(args, "content_type", fmt.Sprintf("%T", what))
	if err != nil {
		b.logger.Error("telegram edit failed", err, args...)
		return err
	}
	b.logger.Debug("telegram edit succeeded", args...)
	return nil
}

func (b *Bot) respond(c telebot.Context, resp ...*telebot.CallbackResponse) error {
	err := c.Respond(resp...)
	args := b.contextLogArgs(c)
	if err != nil {
		b.logger.Error("telegram callback respond failed", err, args...)
		return err
	}
	b.logger.Debug("telegram callback respond succeeded", args...)
	return nil
}

func (b *Bot) contextLogArgs(c telebot.Context) []any {
	args := make([]any, 0, 10)
	if sender := c.Sender(); sender != nil {
		args = append(args, "tg_id", sender.ID)
	}
	if callback := c.Callback(); callback != nil {
		args = append(args,
			"callback_unique", callback.Unique,
			"callback_data", callback.Data,
			"callback_id", callback.ID,
		)
		if callback.Message != nil {
			args = append(args, "message_id", callback.Message.ID)
		}
		if callback.MessageID != "" {
			args = append(args, "inline_message_id", callback.MessageID)
		}
		return args
	}
	if message := c.Message(); message != nil {
		args = append(args, "message_id", message.ID)
	}
	return args
}
