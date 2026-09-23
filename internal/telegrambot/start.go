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
		return b.send(c, msgRegistrationFailed)
	}

	_, err := b.deps.Users.EnsureTelegramUser(service.TelegramUserInput{
		TgID:      sender.ID,
		Username:  sender.Username,
		Firstname: sender.FirstName,
		Lastname:  sender.LastName,
	})
	if err != nil {
		b.logger.Error("telegram start handler failed", err, "tg_id", sender.ID)
		return b.send(c, msgRegistrationFailed)
	}
	if message := c.Message(); message != nil && strings.HasPrefix(message.Payload, "login_") && b.deps.Mobile != nil {
		if err := b.deps.Mobile.ApproveTelegramLogin(strings.TrimPrefix(message.Payload, "login_"), sender.ID); err != nil {
			b.logger.Error("mobile login approval failed", err, "tg_id", sender.ID)
			return b.send(c, "Не удалось подтвердить вход. Запрос мог истечь — начните вход в приложении заново.")
		}
		return b.send(c, "Вход в Corvin Mobile подтверждён. Вернитесь в приложение.")
	}

	if err := b.send(c, fmt.Sprintf(msgStart, displayName(sender)), startMenu()); err != nil {
		b.logger.Error("telegram send failed", err, "tg_id", sender.ID)
		return err
	}
	return nil
}

func (b *Bot) handleID(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return b.send(c, msgTelegramIDFailed)
	}

	message := "Username @" + sender.Username + "\n"
	message += fmt.Sprintf("ID %d\n", sender.ID)
	message += "FirstName " + sender.FirstName + "\n"
	message += "LastName " + sender.LastName + "\n"
	message += "Lang " + sender.LanguageCode + "\n"
	b.logger.Debug("telegram id requested", "tg_id", sender.ID)
	return b.send(c, message)
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
