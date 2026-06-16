package telegrambot

import (
	"fmt"
	"strings"
	"time"
	"vpnpanel/internal/config"

	telebot "gopkg.in/telebot.v4"
)

type Bot struct {
	bot *telebot.Bot
}

func New(cfg config.TelegramConfig) (*Bot, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required when TELEGRAM_ENABLED=true")
	}

	b, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, err
	}

	b.Handle("/ping", func(c telebot.Context) error {
		return c.Send("ok")
	})

	return &Bot{bot: b}, nil
}

func (b *Bot) Start() {
	if b == nil || b.bot == nil {
		return
	}
	go b.bot.Start()
}

func (b *Bot) Stop() {
	if b == nil || b.bot == nil {
		return
	}
	b.bot.Stop()
}

func (b *Bot) Notifier() *Notifier {
	if b == nil {
		return NewNotifier(nil)
	}
	return NewNotifier(b.bot)
}
