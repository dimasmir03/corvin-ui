package telegrambot

import (
	"fmt"
	"strings"
	"time"
	"vpnpanel/internal/config"
	projectlogger "vpnpanel/internal/logger"
	"vpnpanel/internal/service"

	telebot "gopkg.in/telebot.v4"
)

type Deps struct {
	Users   *service.UsersService
	VPN     *service.VPNService
	Support *service.SupportService
	Logger  *projectlogger.Logger
}

type Bot struct {
	bot      *telebot.Bot
	deps     Deps
	logger   *projectlogger.Logger
	adminIDs []int64
	notifier *Notifier
	state    *StateStore
}

func New(cfg config.TelegramConfig, deps Deps) (*Bot, error) {
	technicalLog := resolveLogger(deps.Logger).With("component", "telegrambot")
	if !cfg.Enabled {
		technicalLog.Warn("telegram bot disabled")
		return nil, nil
	}
	if deps.Users == nil {
		return nil, fmt.Errorf("telegrambot requires UsersService when TELEGRAM_ENABLED=true")
	}
	if deps.VPN == nil {
		return nil, fmt.Errorf("telegrambot requires VPNService when TELEGRAM_ENABLED=true")
	}
	if deps.Support == nil {
		return nil, fmt.Errorf("telegrambot requires SupportService when TELEGRAM_ENABLED=true")
	}

	technicalLog.Info("telegram bot enabled")

	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required when TELEGRAM_ENABLED=true")
	}

	tb, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		technicalLog.Error("telegram bot init failed", err)
		return nil, err
	}

	bot := &Bot{
		bot:      tb,
		deps:     deps,
		logger:   technicalLog,
		adminIDs: cfg.AdminIDs,
		notifier: NewNotifier(tb, technicalLog),
		state:    NewStateStore(),
	}
	bot.registerHandlers()
	return bot, nil
}

func (b *Bot) Start() {
	if b == nil || b.bot == nil {
		return
	}
	b.logger.Info("telegram bot started")
	go b.bot.Start()
}

func (b *Bot) Stop() {
	if b == nil || b.bot == nil {
		return
	}
	b.bot.Stop()
	b.logger.Info("telegram bot stopped")
}

func (b *Bot) Notifier() *Notifier {
	if b == nil || b.notifier == nil {
		return NewNotifier(nil, resolveLogger(nil))
	}
	return b.notifier
}

func resolveLogger(log *projectlogger.Logger) *projectlogger.Logger {
	if log != nil {
		return log
	}
	return projectlogger.Default()
}
