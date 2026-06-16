package telegrambot

import (
	"fmt"
	"strings"
	"time"
	"vpnpanel/internal/config"
	"vpnpanel/internal/service"

	telebot "gopkg.in/telebot.v3"
)

type Deps struct {
	Users *service.UsersService
}

type Bot struct {
	bot  *telebot.Bot
	deps Deps
}

func New(cfg config.TelegramConfig, deps Deps) (*Bot, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if deps.Users == nil {
		return nil, fmt.Errorf("telegrambot requires UsersService when TELEGRAM_ENABLED=true")
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

	bot := &Bot{bot: b, deps: deps}
	bot.registerHandlers()
	return bot, nil
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
