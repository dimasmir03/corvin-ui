package telegrambot

import (
	"fmt"
	"log"
	"strings"
	"vpnpanel/internal/service"

	telebot "gopkg.in/telebot.v3"
)

func (b *Bot) registerHandlers() {
	if b == nil || b.bot == nil {
		return
	}

	b.bot.Handle("/ping", func(c telebot.Context) error {
		return c.Send("ok")
	})
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle(&telebot.Btn{Unique: callbackMenuVPN}, func(c telebot.Context) error {
		return respondWithStub(c, vpnComingSoonMessage)
	})
	b.bot.Handle(&telebot.Btn{Unique: callbackMenuInstruction}, func(c telebot.Context) error {
		return respondWithStub(c, instructionSoonMessage)
	})
	b.bot.Handle(&telebot.Btn{Unique: callbackMenuSupport}, func(c telebot.Context) error {
		return respondWithStub(c, supportSoonMessage)
	})
}

func (b *Bot) handleStart(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return c.Send(registrationFailedMessage)
	}

	_, err := b.deps.Users.EnsureTelegramUser(service.TelegramUserInput{
		TgID:      sender.ID,
		Username:  sender.Username,
		Firstname: sender.FirstName,
		Lastname:  sender.LastName,
	})
	if err != nil {
		log.Printf("telegram /start ensure user failed tg_id=%d: %v", sender.ID, err)
		return c.Send(registrationFailedMessage)
	}

	return c.Send(fmt.Sprintf(welcomeMessage, displayName(sender)), startMenu())
}

func respondWithStub(c telebot.Context, text string) error {
	if err := c.Respond(); err != nil {
		log.Printf("telegram callback respond failed: %v", err)
	}
	return c.Send(text)
}

func displayName(user *telebot.User) string {
	name := strings.TrimSpace(strings.TrimSpace(user.FirstName + " " + user.LastName))
	if name != "" {
		return name
	}
	if user.Username != "" {
		return user.Username
	}
	return "друг"
}
