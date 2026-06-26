package telegrambot

import (
	"fmt"
	"strings"
	"vpnpanel/internal/service"

	telebot "gopkg.in/telebot.v4"
)

func (b *Bot) maintenanceMiddleware(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		if !b.maintenanceEnabled {
			return next(c)
		}

		sender := c.Sender()
		if sender != nil && b.isAdmin(sender.ID) {
			b.logger.Debug("telegram maintenance bypassed for admin", "tg_id", sender.ID)
			return next(c)
		}

		if sender != nil {
			b.logger.Info("telegram maintenance response sent", "tg_id", sender.ID)
		} else {
			b.logger.Info("telegram maintenance response sent", "reason", "sender is nil")
		}

		if c.Callback() != nil {
			if err := b.respond(c); err != nil {
				return err
			}
		}
		return b.send(c, msgMaintenance)
	}
}

func (b *Bot) ensureTelegramUserMiddleware(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		sender := c.Sender()
		if sender == nil {
			b.logger.Warn("telegram user check skipped", "reason", "sender is nil")
			return next(c)
		}

		b.logger.Info("telegram user lookup started", "component", "telegrambot", "operation", "ensure_user", "tg_id", sender.ID)
		telegramUser, err := b.deps.Users.EnsureTelegramUser(service.TelegramUserInput{
			TgID:      sender.ID,
			Username:  sender.Username,
			Firstname: sender.FirstName,
			Lastname:  sender.LastName,
		})
		if err != nil {
			b.logger.Error("telegram user check failed", err, "component", "telegrambot", "operation", "ensure_user", "tg_id", sender.ID, "reason", "ensure_user_failed")
			if c.Callback() != nil {
				if respondErr := b.respond(c); respondErr != nil {
					return respondErr
				}
			}
			return b.send(c, msgRegistrationFailed)
		}

		b.logger.Info("telegram user found", "component", "telegrambot", "operation", "ensure_user", "tg_id", sender.ID, "user_id", telegramUser.UserID, "reason", "user_ensured")
		return next(c)
	}
}

func (b *Bot) withLogging(name string, next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		senderID := int64(0)
		if sender := c.Sender(); sender != nil {
			senderID = sender.ID
		}

		args := []any{"component", "telegrambot", "handler", name, "tg_id", senderID}
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

		b.logger.Info("telegram command received", append(args, "command", telegramCommandName(c, name))...)
		b.logger.Info("telegram handler started", args...)
		err := next(c)
		if err != nil {
			b.logger.Error("telegram handler failed", err, args...)
			return err
		}

		b.logger.Info("telegram handler finished", append(args, "reason", "success")...)
		return nil
	}
}

func (b *Bot) send(c telebot.Context, what any, opts ...any) error {
	args := b.contextLogArgs(c)
	args = append(args, "external_system", "telegram_api", "content_type", fmt.Sprintf("%T", what))
	b.logger.Info("telegram send started", args...)
	err := c.Send(what, opts...)
	if err != nil {
		b.logger.Error("telegram send failed", err, args...)
		return err
	}
	b.logger.Info("telegram send succeeded", args...)
	return nil
}

func (b *Bot) edit(c telebot.Context, what any, opts ...any) error {
	args := b.contextLogArgs(c)
	args = append(args, "external_system", "telegram_api", "content_type", fmt.Sprintf("%T", what))
	b.logger.Info("telegram edit started", args...)
	err := c.Edit(what, opts...)
	if err != nil {
		b.logger.Error("telegram edit failed", err, args...)
		return err
	}
	b.logger.Info("telegram edit succeeded", args...)
	return nil
}

func (b *Bot) respond(c telebot.Context, resp ...*telebot.CallbackResponse) error {
	args := b.contextLogArgs(c)
	args = append(args, "external_system", "telegram_api")
	b.logger.Info("telegram callback respond started", args...)
	err := c.Respond(resp...)
	if err != nil {
		b.logger.Error("telegram callback respond failed", err, args...)
		return err
	}
	b.logger.Info("telegram callback respond succeeded", args...)
	return nil
}

func telegramCommandName(c telebot.Context, fallback string) string {
	if message := c.Message(); message != nil {
		text := strings.TrimSpace(message.Text)
		if strings.HasPrefix(text, "/") {
			return strings.Fields(text)[0]
		}
	}
	if c.Callback() != nil {
		return "callback"
	}
	return fallback
}

func (b *Bot) contextLogArgs(c telebot.Context) []any {
	args := make([]any, 0, 12)
	args = append(args, "component", "telegrambot")
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
