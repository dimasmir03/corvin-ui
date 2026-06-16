package telegrambot

import (
	"fmt"
	"strings"
	"vpnpanel/internal/service"

	telebot "gopkg.in/telebot.v4"
)

func (b *Bot) handleStart(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		b.logger.Error("telegram start handler failed", nil, "reason", "sender is nil")
		return c.Send(msgRegistrationFailed)
	}

	_, err := b.deps.Users.EnsureTelegramUser(service.TelegramUserInput{
		TgID:      sender.ID,
		Username:  sender.Username,
		Firstname: sender.FirstName,
		Lastname:  sender.LastName,
	})
	if err != nil {
		b.logger.Error("telegram start handler failed", err, "tg_id", sender.ID)
		return c.Send(msgRegistrationFailed)
	}

	if err := c.Send(fmt.Sprintf(msgStart, displayName(sender)), startMenu()); err != nil {
		b.logger.Error("telegram send failed", err, "tg_id", sender.ID)
		return err
	}
	return nil
}

func (b *Bot) handleID(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return c.Send("Не удалось определить Telegram ID.")
	}
	return c.Send(fmt.Sprintf("Ваш Telegram ID: %d", sender.ID))
}

func displayName(user *telebot.User) string {
	name := strings.TrimSpace(strings.TrimSpace(user.FirstName + " " + user.LastName))
	if name != "" {
		return name
	}
	if user.Username != "" {
		return user.Username
	}
	return "друг"
}
