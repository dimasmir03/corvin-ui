package telegrambot

import (
	"errors"
	"fmt"
	projectlogger "vpnpanel/internal/logger"

	telebot "gopkg.in/telebot.v4"
)

var ErrNotifierDisabled = errors.New("telegram notifier is disabled")

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

func (n *Notifier) SendSupportReply(tgID int64, text string) error {
	if err := n.SendText(tgID, fmt.Sprintf("Support reply:\n\n%s", text)); err != nil {
		return err
	}
	n.logger.Info("support reply notification sent", "tg_id", tgID)
	return nil
}
