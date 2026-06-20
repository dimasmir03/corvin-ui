package telegrambot

import (
	"fmt"
	"io"
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
		return b.send(c, msgSupportCreateFailed)
	}

	b.state.SetMode(sender.ID, ModeSupport)
	b.logger.Info("support flow opened", "tg_id", sender.ID)
	return b.send(c, msgSupportPrompt, supportMenu())
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
		return b.send(c, msgSupportCreateFailed)
	}

	text := strings.TrimSpace(c.Text())
	if text == "" {
		return b.send(c, msgSupportPrompt, supportMenu())
	}

	complaint, err := b.deps.Support.CreateComplaint(service.CreateComplaintInput{
		TgID: sender.ID,
		Text: text,
	})
	if err != nil {
		b.logger.Error("support complaint create failed", err, "tg_id", sender.ID)
		return b.send(c, msgSupportCreateFailed)
	}

	b.state.ClearMode(sender.ID)
	b.logger.Info("support complaint created", "tg_id", sender.ID, "complaint_id", complaint.ID)

	if err := b.send(c, fmt.Sprintf(msgSupportSent, complaint.ID)); err != nil {
		b.logger.Error("telegram send failed", err, "tg_id", sender.ID)
		return err
	}

	if err := b.Notifier().SendSupportAdminNotification(b.adminIDs, SupportAdminNotification{
		ComplaintID: complaint.ID,
		TgID:        complaint.TgID,
		Username:    complaint.Username,
		Text:        complaint.Text,
		PhotoFileID: complaint.PhotoFileID,
	}); err != nil {
		b.logger.Error("support admin notification failed", err, "tg_id", sender.ID, "complaint_id", complaint.ID, "admin_count", len(b.adminIDs))
	} else {
		b.logger.Info("support admin notification sent", "tg_id", sender.ID, "complaint_id", complaint.ID, "admin_count", len(b.adminIDs))
	}

	return nil
}

func (b *Bot) handleSupportPhoto(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		b.logger.Error("support photo complaint create failed", nil, "reason", "sender is nil")
		return b.send(c, msgSupportCreateFailed)
	}

	message := c.Message()
	if message == nil || message.Photo == nil || strings.TrimSpace(message.Photo.FileID) == "" {
		b.logger.Error("support photo complaint create failed", nil, "tg_id", sender.ID, "reason", "photo file_id is empty")
		return b.send(c, msgSupportCreateFailed)
	}

	caption := strings.TrimSpace(message.Caption)
	if caption == "" {
		caption = "Фото без описания"
	}

	photoFile := message.Photo.File
	reader, err := b.bot.File(&photoFile)
	if err != nil {
		b.logger.Error("support complaint photo download failed", err, "tg_id", sender.ID)
		return b.send(c, msgSupportCreateFailed)
	}
	defer reader.Close()

	photoBytes, err := io.ReadAll(reader)
	if err != nil {
		b.logger.Error("support complaint photo download failed", err, "tg_id", sender.ID)
		return b.send(c, msgSupportCreateFailed)
	}

	b.logger.Info("support complaint photo upload started", "tg_id", sender.ID, "has_photo", true)
	complaint, err := b.deps.Support.CreateComplaint(service.CreateComplaintInput{
		TgID: sender.ID,
		Text: caption,
		Photo: &service.ComplaintPhotoInput{
			FileName:       message.Photo.UniqueID + ".jpg",
			MimeType:       "image/jpeg",
			Data:           photoBytes,
			TelegramFileID: message.Photo.FileID,
			TelegramUnique: message.Photo.UniqueID,
		},
	})
	if err != nil {
		b.logger.Error("support complaint photo upload failed", err, "tg_id", sender.ID)
		return b.send(c, msgSupportCreateFailed)
	}
	b.logger.Info("support complaint photo uploaded", "tg_id", sender.ID, "complaint_id", complaint.ID)

	b.state.ClearMode(sender.ID)
	b.logger.Info("support photo complaint created", "tg_id", sender.ID, "complaint_id", complaint.ID, "has_photo", complaint.Photo)

	if err := b.send(c, fmt.Sprintf(msgSupportPhotoSent, complaint.ID)); err != nil {
		b.logger.Error("telegram send failed", err, "tg_id", sender.ID)
		return err
	}

	if err := b.Notifier().SendSupportAdminNotification(b.adminIDs, SupportAdminNotification{
		ComplaintID: complaint.ID,
		TgID:        complaint.TgID,
		Username:    complaint.Username,
		Text:        complaint.Text,
		PhotoFileID: complaint.PhotoFileID,
	}); err != nil {
		b.logger.Error("support photo admin notification failed", err, "tg_id", sender.ID, "complaint_id", complaint.ID, "admin_count", len(b.adminIDs))
	} else {
		b.logger.Info("support photo admin notification sent", "tg_id", sender.ID, "complaint_id", complaint.ID, "admin_count", len(b.adminIDs))
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
		return b.send(c, msgSupportReplyCanceled, startMenu())
	}

	return b.send(c, msgSupportCanceled, startMenu())
}

func (b *Bot) handleSupportReplyStart(c telebot.Context) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		return nil
	}
	callback := c.Callback()
	if callback == nil {
		return b.send(c, msgSupportReplyFailed)
	}

	complaintIDText := strings.TrimSpace(callback.Data)
	if complaintIDText == "" {
		complaintIDText = strings.TrimSpace(callback.Unique)
	}
	if strings.HasPrefix(complaintIDText, callbackSupportReply+":") {
		complaintIDText = strings.TrimPrefix(complaintIDText, callbackSupportReply+":")
	}
	complaintID64, err := strconv.ParseUint(complaintIDText, 10, 64)
	if err != nil || complaintID64 == 0 {
		b.logger.Error("support reply start failed", err, "admin_tg_id", sender.ID)
		return b.send(c, msgSupportReplyFailed)
	}

	complaintID := uint(complaintID64)
	b.state.SetSupportReply(sender.ID, complaintID)
	b.logger.Info("support reply started", "admin_tg_id", sender.ID, "complaint_id", complaintID)
	return b.send(c, msgSupportReplyPrompt, supportMenu())
}

func (b *Bot) handleSupportReplyText(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}
	if !b.isAdmin(sender.ID) {
		b.logger.Warn("admin reply ignored for non-admin", "tg_id", sender.ID)
		b.state.ClearMode(sender.ID)
		return nil
	}

	state := b.state.GetState(sender.ID)
	if state.Mode != ModeSupportReply || state.ComplaintID == 0 {
		b.state.ClearMode(sender.ID)
		return b.send(c, msgSupportReplyFailed)
	}

	text := strings.TrimSpace(c.Text())
	if text == "" {
		return b.send(c, msgSupportReplyPrompt, supportMenu())
	}

	result, err := b.deps.Support.ReplyToComplaint(service.ReplyToComplaintInput{
		AdminTgID:   sender.ID,
		ComplaintID: state.ComplaintID,
		Text:        text,
	})
	if err != nil {
		b.logger.Error("support reply save failed", err, "admin_tg_id", sender.ID, "complaint_id", state.ComplaintID)
		return b.send(c, msgSupportReplyFailed)
	}

	b.state.ClearMode(sender.ID)
	b.logger.Info("support reply saved", "admin_tg_id", sender.ID, "complaint_id", result.ComplaintID, "user_tg_id", result.UserTgID)

	if err := b.Notifier().SendSupportReply(result.UserTgID, result.Text); err != nil {
		b.logger.Error("support reply send failed", err, "admin_tg_id", sender.ID, "complaint_id", result.ComplaintID, "user_tg_id", result.UserTgID)
		return b.send(c, msgSupportReplySavedSendFailed)
	}

	b.logger.Info("support reply sent", "admin_tg_id", sender.ID, "complaint_id", result.ComplaintID, "user_tg_id", result.UserTgID)
	return b.send(c, msgSupportReplySent)
}
