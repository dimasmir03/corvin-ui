package telegrambot

import (
	"errors"
	"fmt"
	projectlogger "vpnpanel/internal/logger"

	telebot "gopkg.in/telebot.v4"
)

var ErrNotifierDisabled = errors.New("telegram notifier is disabled")

type SupportAdminNotification struct {
	ComplaintID uint
	TgID        int64
	Username    string
	Text        string
	PhotoFileID string
}

type Notifier struct {
	bot    *telebot.Bot
	logger *projectlogger.Logger
}

func NewNotifier(bot *telebot.Bot, log *projectlogger.Logger) *Notifier {
	return &Notifier{bot: bot, logger: resolveLogger(log)}
}

func (n *Notifier) SendText(tgID int64, text string) error {
	if n == nil || n.bot == nil {
		return ErrNotifierDisabled
	}

	_, err := n.bot.Send(telebot.ChatID(tgID), text)
	if err != nil {
		n.logger.Error("telegram send failed", err, "tg_id", tgID)
	}
	return err
}

func (n *Notifier) SendVPNReady(tgID int64, link string) error {
	if err := n.SendText(tgID, fmt.Sprintf("✅ VPN готов.\n\nСсылка:\n%s", link)); err != nil {
		return err
	}
	n.logger.Info("telegram vpn ready notification sent", "tg_id", tgID)
	return nil
}

func (n *Notifier) SendSupportAdminNotification(adminIDs []int64, data SupportAdminNotification) error {
	if len(adminIDs) == 0 {
		if n != nil {
			n.logger.Info("support admin notification skipped", "complaint_id", data.ComplaintID)
		}
		return nil
	}
	if n == nil || n.bot == nil {
		return ErrNotifierDisabled
	}

	markup := &telebot.ReplyMarkup{}
	btnReply := markup.Data("Ответить", callbackSupportReply, fmt.Sprint(data.ComplaintID))
	markup.Inline(markup.Row(btnReply))

	message := supportAdminNotificationMessage(data)
	var firstErr error
	for _, adminID := range adminIDs {
		if data.PhotoFileID != "" {
			photo := &telebot.Photo{
				File:    telebot.File{FileID: data.PhotoFileID},
				Caption: message,
			}
			if _, err := n.bot.Send(telebot.ChatID(adminID), photo, markup); err == nil {
				n.logger.Info("support photo admin notification sent", "complaint_id", data.ComplaintID, "admin_id", adminID)
				continue
			} else {
				n.logger.Error("support photo admin notification failed", err, "complaint_id", data.ComplaintID, "admin_id", adminID)
			}
		}

		if _, err := n.bot.Send(telebot.ChatID(adminID), message, markup); err != nil {
			n.logger.Error("support admin notification failed", err, "complaint_id", data.ComplaintID, "admin_id", adminID)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		n.logger.Info("support admin notification sent", "complaint_id", data.ComplaintID, "admin_id", adminID, "has_photo", data.PhotoFileID != "")
	}

	return firstErr
}

func supportAdminNotificationMessage(data SupportAdminNotification) string {
	message := fmt.Sprintf("💬 Новое обращение в поддержку\n\nID: %d\nПользователь: @%s / %d\n\nТекст:\n%s", data.ComplaintID, data.Username, data.TgID, data.Text)
	if data.PhotoFileID != "" {
		message += "\n\n📎 Вложение: фото"
	}
	return message
}

func (n *Notifier) SendAdminDirectMessage(tgID int64, text string) error {
	if err := n.SendText(tgID, fmt.Sprintf("Сообщение от поддержки:\n\n%s", text)); err != nil {
		return err
	}
	n.logger.Info("admin direct message sent", "tg_id", tgID)
	return nil
}

func (n *Notifier) SendSupportReply(tgID int64, text string) error {
	if err := n.SendText(tgID, fmt.Sprintf("Support reply:\n\n%s", text)); err != nil {
		return err
	}
	n.logger.Info("support reply notification sent", "tg_id", tgID)
	return nil
}
