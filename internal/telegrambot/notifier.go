package telegrambot

import (
	"errors"
	"fmt"

	telebot "gopkg.in/telebot.v4"
)

var ErrNotifierDisabled = errors.New("telegram notifier is disabled")

type Notifier struct {
	bot *telebot.Bot
}

func NewNotifier(bot *telebot.Bot) *Notifier {
	return &Notifier{bot: bot}
}

func (n *Notifier) SendText(tgID int64, text string) error {
	if n == nil || n.bot == nil {
		return ErrNotifierDisabled
	}

	_, err := n.bot.Send(telebot.ChatID(tgID), text)
	return err
}

func (n *Notifier) SendVPNReady(tgID int64, link string) error {
	return n.SendText(tgID, fmt.Sprintf("VPN ready.\n\nLink:\n%s", link))
}

func (n *Notifier) SendSupportReply(tgID int64, text string) error {
	return n.SendText(tgID, fmt.Sprintf("Support reply:\n\n%s", text))
}
