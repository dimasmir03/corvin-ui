package telegrambot

import telebot "gopkg.in/telebot.v3"

const (
	callbackMenuVPN         = "menu:vpn"
	callbackMenuInstruction = "menu:instruction"
	callbackMenuSupport     = "menu:support"
)

func startMenu() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnVPN := menu.Data("🔐 VPN", callbackMenuVPN)
	btnInstruction := menu.Data("📖 Инструкция", callbackMenuInstruction)
	btnSupport := menu.Data("💬 Поддержка", callbackMenuSupport)

	menu.Inline(
		menu.Row(btnVPN),
		menu.Row(btnInstruction),
		menu.Row(btnSupport),
	)

	return menu
}
