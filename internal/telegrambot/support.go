package telegrambot

import (
	"fmt"
	"strconv"
	"strings"
	"vpnpanel/internal/service"

	telebot "gopkg.in/telebot.v4"
)

func (b *Bot) handleSupportOpen(c telebot.Context) error {
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

func (b *Bot) handleSupport(c telebot.Context) error {
	return b.handleSupportOpen(c)
}

func (b *Bot) handleHelp(c telebot.Context) error {
	sender := c.Sender()
	if sender != nil {
		b.logger.Info("support help command opened", "tg_id", sender.ID)
	}
	return b.handleSupportOpen(c)
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
	if sender == nil {
		return nil
	}

	state := b.state.GetState(sender.ID)
	b.state.ClearMode(sender.ID)
	if state.Mode == ModeSupportReply {
		b.logger.Info("support reply canceled", "admin_tg_id", sender.ID, "complaint_id", state.ComplaintID)
		return c.Send(msgSupportReplyCanceled, startMenu())
	}

	return c.Send(msgSupportCanceled, startMenu())
}

func (b *Bot) handleSupportReplyStart(c telebot.Context) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		return nil
	}
	if !b.isAdmin(sender.ID) {
		b.logger.Warn("support reply access denied", "admin_tg_id", sender.ID)
		return c.Send(msgAccessDenied)
	}

	callback := c.Callback()
	if callback == nil {
		return c.Send(msgSupportReplyFailed)
	}

	complaintID64, err := strconv.ParseUint(strings.TrimSpace(callback.Data), 10, 64)
	if err != nil || complaintID64 == 0 {
		b.logger.Error("support reply start failed", err, "admin_tg_id", sender.ID)
		return c.Send(msgSupportReplyFailed)
	}

	complaintID := uint(complaintID64)
	b.state.SetSupportReply(sender.ID, complaintID)
	b.logger.Info("support reply started", "admin_tg_id", sender.ID, "complaint_id", complaintID)
	return c.Send(fmt.Sprintf(msgSupportReplyPrompt, complaintID), supportMenu())
}

func (b *Bot) handleSupportReplyText(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}
	if !b.isAdmin(sender.ID) {
		b.logger.Warn("support reply access denied", "admin_tg_id", sender.ID)
		b.state.ClearMode(sender.ID)
		return c.Send(msgAccessDenied)
	}

	state := b.state.GetState(sender.ID)
	if state.Mode != ModeSupportReply || state.ComplaintID == 0 {
		b.state.ClearMode(sender.ID)
		return c.Send(msgSupportReplyFailed)
	}

	text := strings.TrimSpace(c.Text())
	if text == "" {
		return c.Send(fmt.Sprintf(msgSupportReplyPrompt, state.ComplaintID), supportMenu())
	}

	result, err := b.deps.Support.ReplyToComplaint(service.ReplyToComplaintInput{
		AdminTgID:   sender.ID,
		ComplaintID: state.ComplaintID,
		Text:        text,
	})
	if err != nil {
		b.logger.Error("support reply save failed", err, "admin_tg_id", sender.ID, "complaint_id", state.ComplaintID)
		return c.Send(msgSupportReplyFailed)
	}

	b.state.ClearMode(sender.ID)
	b.logger.Info("support reply saved", "admin_tg_id", sender.ID, "complaint_id", result.ComplaintID, "user_tg_id", result.UserTgID)

	if err := b.Notifier().SendSupportReply(result.UserTgID, result.Text); err != nil {
		b.logger.Error("support reply send failed", err, "admin_tg_id", sender.ID, "complaint_id", result.ComplaintID, "user_tg_id", result.UserTgID)
		return c.Send(msgSupportReplySavedSendFailed)
	}

	b.logger.Info("support reply sent", "admin_tg_id", sender.ID, "complaint_id", result.ComplaintID, "user_tg_id", result.UserTgID)
	return c.Send(msgSupportReplySent)
}
