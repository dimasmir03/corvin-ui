package telegrambot

import (
	"fmt"
	"strings"
	"vpnpanel/internal/service"

	telebot "gopkg.in/telebot.v4"
)

func (b *Bot) isAdmin(tgID int64) bool {
	if b == nil || len(b.adminIDs) == 0 {
		return false
	}

	for _, adminID := range b.adminIDs {
		if adminID == tgID {
			return true
		}
	}
	return false
}

func (b *Bot) handleGetUsers(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}
	if !b.isAdmin(sender.ID) {
		b.logger.Warn("admin getusers access denied", "admin_tg_id", sender.ID)
		return c.Send(msgAccessDenied)
	}

	b.logger.Info("admin getusers requested", "admin_tg_id", sender.ID)
	users, err := b.deps.Users.ListTelegramUsers()
	if err != nil {
		b.logger.Error("admin getusers failed", err, "admin_tg_id", sender.ID)
		return c.Send(msgAdminGetUsersFailed)
	}

	b.logger.Info("admin getusers completed", "admin_tg_id", sender.ID, "users_count", len(users))
	return c.Send(formatAdminTelegramUsers(users))
}

func formatAdminTelegramUsers(users []service.AdminTelegramUserView) string {
	if len(users) == 0 {
		return msgAdminGetUsersEmpty
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("👥 Пользователи: %d", len(users)))
	builder.WriteString("\n\n")
	for i := 0; i < len(users); i++ {
		user := users[i]
		builder.WriteString(fmt.Sprintf("%d. %s — tg_id: %d — user_id: %d", i+1, adminTelegramUserDisplayName(user), user.TgID, user.UserID))
	}

	return builder.String()
}

func adminTelegramUserDisplayName(user service.AdminTelegramUserView) string {
	if username := strings.TrimSpace(user.Username); username != "" {
		return "@" + username
	}
	name := strings.TrimSpace(strings.TrimSpace(user.Firstname) + " " + strings.TrimSpace(user.Lastname))
	if name != "" {
		return name
	}
	return "без имени"
}
