package telegrambot

import (
	"strings"
	"vpnpanel/internal/service"

	telebot "gopkg.in/telebot.v4"
)

func (b *Bot) handleSupport(c telebot.Context) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("support flow opened failed", nil, "reason", "sender is nil")
		return c.Send(msgSupportCreateFailed)
	}

	b.state.SetMode(sender.ID, ModeSupport)
	b.logger.Info("support flow opened", "tg_id", sender.ID)
	return c.Send(msgSupportPrompt, supportMenu())
}

func (b *Bot) handleSupportText(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		b.logger.Error("support complaint create failed", nil, "reason", "sender is nil")
		return c.Send(msgSupportCreateFailed)
	}

	text := strings.TrimSpace(c.Text())
	if text == "" {
		return c.Send(msgSupportPrompt, supportMenu())
	}

	complaint, err := b.deps.Support.CreateComplaint(service.CreateComplaintInput{
		TgID: sender.ID,
		Text: text,
	})
	if err != nil {
		b.logger.Error("support complaint create failed", err, "tg_id", sender.ID)
		return c.Send(msgSupportCreateFailed)
	}

	b.state.ClearMode(sender.ID)
	b.logger.Info("support complaint created", "tg_id", sender.ID, "complaint_id", complaint.ID)

	if err := c.Send(msgSupportSent); err != nil {
		b.logger.Error("telegram send failed", err, "tg_id", sender.ID)
		return err
	}

	if err := b.Notifier().SendSupportAdminNotification(b.adminIDs, SupportAdminNotification{
		ComplaintID: complaint.ID,
		TgID:        complaint.TgID,
		Username:    complaint.Username,
		Text:        complaint.Text,
	}); err != nil {
		b.logger.Error("support admin notification failed", err, "tg_id", sender.ID, "complaint_id", complaint.ID, "admin_count", len(b.adminIDs))
	} else {
		b.logger.Info("support admin notification sent", "tg_id", sender.ID, "complaint_id", complaint.ID, "admin_count", len(b.adminIDs))
	}

	return nil
}

func (b *Bot) handleSupportCancel(c telebot.Context) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender != nil {
		b.state.ClearMode(sender.ID)
	}
	return c.Send(msgSupportCanceled, startMenu())
}

func (b *Bot) handleSupportReplyPlaceholder(c telebot.Context) error {
	b.respondToCallback(c)
	return c.Send(msgSupportReplyPlaceholder)
}
