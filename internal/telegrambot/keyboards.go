package telegrambot

import telebot "gopkg.in/telebot.v4"

const (
	callbackMenuVPN         = "menu:vpn"
	callbackMenuInstruction = "menu:instruction"
	callbackMenuSupport     = "menu:support"
)

var (
	menuMarkup = &telebot.ReplyMarkup{}

	btnMenuVPN         = menuMarkup.Data("🔐 VPN", callbackMenuVPN)
	btnMenuInstruction = menuMarkup.Data("📖 Инструкция", callbackMenuInstruction)
	btnMenuSupport     = menuMarkup.Data("💬 Поддержка", callbackMenuSupport)
)

func startMenu() *telebot.ReplyMarkup {
	menuMarkup.Inline(
		menuMarkup.Row(btnMenuVPN),
		menuMarkup.Row(btnMenuInstruction),
		menuMarkup.Row(btnMenuSupport),
	)

	return menuMarkup
}
