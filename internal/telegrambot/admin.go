package telegrambot

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"vpnpanel/internal/service"

	telebot "gopkg.in/telebot.v4"
)

const broadcastSendDelay = 50 * time.Millisecond

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

func (b *Bot) adminMiddleware(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		sender := c.Sender()
		logMsg := "admin command ignored for non-admin"
		if c.Callback() != nil {
			logMsg = "admin callback ignored for non-admin"
		}
		if sender == nil {
			b.logger.Debug(logMsg)
			return nil
		}
		if !b.isAdmin(sender.ID) {
			b.logger.Debug(logMsg, "tg_id", sender.ID)
			return nil
		}
		return next(c)
	}
}

func (b *Bot) handleGetUsers(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
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

func (b *Bot) handleSendUser(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	args := c.Args()
	if len(args) < 2 {
		return c.Send(msgAdminSendUserUsage)
	}

	targetTgID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
	if err != nil || targetTgID == 0 {
		return c.Send(msgAdminSendUserInvalidID)
	}

	message := strings.TrimSpace(strings.Join(args[1:], " "))
	if message == "" {
		return c.Send(msgAdminSendUserUsage)
	}

	b.logger.Info("admin senduser requested", "admin_tg_id", sender.ID, "target_tg_id", targetTgID, "message_len", len(message))
	if _, err := b.deps.Users.GetTelegramByTgID(targetTgID); err != nil {
		b.logger.Warn("admin senduser target not found", "admin_tg_id", sender.ID, "target_tg_id", targetTgID)
		return c.Send(msgAdminSendUserNotFound)
	}

	if err := b.Notifier().SendAdminDirectMessage(targetTgID, message); err != nil {
		b.logger.Error("admin senduser failed", err, "admin_tg_id", sender.ID, "target_tg_id", targetTgID, "message_len", len(message))
		return c.Send(msgAdminSendUserFailed)
	}

	b.logger.Info("admin senduser sent", "admin_tg_id", sender.ID, "target_tg_id", targetTgID, "message_len", len(message))
	return c.Send(msgAdminSendUserSent)
}

func (b *Bot) handleSendBroadcast(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	message := strings.TrimSpace(strings.Join(c.Args(), " "))
	if message == "" {
		return c.Send(msgAdminBroadcastUsage)
	}

	users, err := b.deps.Users.ListTelegramUsers()
	if err != nil {
		b.logger.Error("admin broadcast draft failed", err, "admin_tg_id", sender.ID, "message_len", len(message))
		return c.Send(msgAdminGetUsersFailed)
	}
	if len(users) == 0 {
		return c.Send(msgAdminBroadcastNoRecipients)
	}

	b.state.SetBroadcastDraft(sender.ID, BroadcastDraft{
		Text:            message,
		RecipientsCount: len(users),
		CreatedAt:       time.Now(),
	})
	b.logger.Info("admin broadcast draft created", "admin_tg_id", sender.ID, "recipients_count", len(users), "message_len", len(message))

	return c.Send(formatBroadcastPreview(message, len(users)), broadcastConfirmMenu())
}

func (b *Bot) handleBroadcastConfirm(c telebot.Context) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		return nil
	}

	draft, ok := b.state.GetBroadcastDraft(sender.ID)
	if !ok || strings.TrimSpace(draft.Text) == "" {
		return c.Send(msgAdminBroadcastNoDraft)
	}

	users, err := b.deps.Users.ListTelegramUsers()
	if err != nil {
		b.logger.Error("admin broadcast failed", err, "admin_tg_id", sender.ID, "message_len", len(draft.Text))
		return c.Send(msgAdminGetUsersFailed)
	}

	total := len(users)
	sent := 0
	failed := 0
	b.logger.Info("admin broadcast started", "admin_tg_id", sender.ID, "recipients_count", total, "message_len", len(draft.Text))
	for _, user := range users {
		if err := b.Notifier().SendAdminBroadcastMessage(user.TgID, draft.Text); err != nil {
			failed++
			b.logger.Error("admin broadcast recipient failed", err, "admin_tg_id", sender.ID, "target_tg_id", user.TgID)
		} else {
			sent++
		}
		time.Sleep(broadcastSendDelay)
	}

	b.state.ClearBroadcastDraft(sender.ID)
	b.logger.Info("admin broadcast finished", "admin_tg_id", sender.ID, "recipients_count", total, "sent", sent, "failed", failed, "message_len", len(draft.Text))
	return c.Send(formatBroadcastResult(total, sent, failed))
}

func (b *Bot) handleBroadcastCancel(c telebot.Context) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		return nil
	}
	b.state.ClearBroadcastDraft(sender.ID)
	b.logger.Info("admin broadcast cancelled", "admin_tg_id", sender.ID)
	return c.Send(msgAdminBroadcastCancelled)
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

func formatBroadcastPreview(message string, recipientsCount int) string {
	return fmt.Sprintf("📣 Рассылка\n\nПолучателей: %d\n\nТекст:\n%s\n\nОтправить?", recipientsCount, message)
}

func formatBroadcastResult(total int, sent int, failed int) string {
	return fmt.Sprintf("✅ Рассылка завершена.\n\nВсего: %d\nОтправлено: %d\nОшибок: %d", total, sent, failed)
}
