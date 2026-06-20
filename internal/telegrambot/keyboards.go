package telegrambot

import telebot "gopkg.in/telebot.v4"

const (
	callbackMenuVPN         = "menu:vpn"
	callbackMenuInstruction = "menu:instruction"
	callbackMenuSupport     = "support:open"

	callbackVPNVLESS     = "vpn:vless"
	callbackVPNTrojan    = "vpn:trojan"
	callbackCreateVLESS  = "vpn:create:vless"
	callbackCreateTrojan = "vpn:create:trojan"
	callbackMainMenu     = "menu:main"

	callbackLinkVLESS  = "link:vless"
	callbackLinkTrojan = "link:trojan"

	callbackInstructionPrev = "instruction:prev"
	callbackInstructionNext = "instruction:next"
	callbackInstructionMenu = "instruction:menu"

	callbackSupportCancel = "support:cancel"
	callbackSupportReply  = "support:reply"

	callbackBroadcastConfirm = "admin:broadcast:confirm"
	callbackBroadcastCancel  = "admin:broadcast:cancel"
)

var (
	menuMarkup = &telebot.ReplyMarkup{}

	btnMenuVPN         = menuMarkup.Data("Создать VPN 🏴‍☠️", callbackMenuVPN)
	btnMenuInstruction = menuMarkup.Data("Инструкция‍  📖", callbackMenuInstruction)
	btnMenuSupport     = menuMarkup.Data("️Помощь 🆘", callbackMenuSupport)

	vpnMarkup       = &telebot.ReplyMarkup{}
	vpnCreateMarkup = &telebot.ReplyMarkup{}
	linkMarkup      = &telebot.ReplyMarkup{}

	btnVPNVLESS     = vpnMarkup.Data("🔗 Основной", callbackVPNVLESS)
	btnVPNTrojan    = vpnMarkup.Data("🛡️ Обход", callbackVPNTrojan)
	btnCreateVLESS  = vpnMarkup.Data("🔗 Основной", callbackCreateVLESS)
	btnCreateTrojan = vpnMarkup.Data("🛡️ Обход", callbackCreateTrojan)
	btnMainMenu     = vpnMarkup.Data("🏠 Главное меню", callbackMainMenu)
	btnLinkVLESS    = linkMarkup.Data("🔗 VLESS", callbackLinkVLESS)
	btnLinkTrojan   = linkMarkup.Data("🔗 Trojan", callbackLinkTrojan)

	instructionMarkup = &telebot.ReplyMarkup{}

	btnInstructionPrev = instructionMarkup.Data("⬅️ Назад", callbackInstructionPrev)
	btnInstructionNext = instructionMarkup.Data("➡️ Далее", callbackInstructionNext)
	btnInstructionMenu = instructionMarkup.Data("🏠 Главное меню", callbackInstructionMenu)

	supportMarkup   = &telebot.ReplyMarkup{}
	broadcastMarkup = &telebot.ReplyMarkup{}

	btnSupportCancel = supportMarkup.Data("❌ Отмена", callbackSupportCancel)
	btnSupportReply  = supportMarkup.Data("Ответить", callbackSupportReply)

	btnBroadcastConfirm = broadcastMarkup.Data("✅ Отправить", callbackBroadcastConfirm)
	btnBroadcastCancel  = broadcastMarkup.Data("❌ Отмена", callbackBroadcastCancel)
)

func startMenu() *telebot.ReplyMarkup {
	menuMarkup.Inline(
		menuMarkup.Row(btnMenuVPN),
		menuMarkup.Row(btnMenuInstruction),
		menuMarkup.Row(btnMenuSupport),
	)

	return menuMarkup
}

func vpnMenu() *telebot.ReplyMarkup {
	vpnMarkup.Inline(
		vpnMarkup.Row(btnVPNVLESS),
		vpnMarkup.Row(btnVPNTrojan),
		vpnMarkup.Row(btnCreateVLESS),
		vpnMarkup.Row(btnCreateTrojan),
		vpnMarkup.Row(btnMainMenu),
	)

	return vpnMarkup
}

func vpnCreateMenu() *telebot.ReplyMarkup {
	vpnCreateMarkup.Inline(
		vpnCreateMarkup.Row(btnCreateVLESS),
		vpnCreateMarkup.Row(btnCreateTrojan),
		vpnCreateMarkup.Row(btnMainMenu),
	)

	return vpnCreateMarkup
}

func linkMenu() *telebot.ReplyMarkup {
	linkMarkup.Inline(
		linkMarkup.Row(btnLinkVLESS),
		linkMarkup.Row(btnLinkTrojan),
	)

	return linkMarkup
}

func instructionMenu(step int) *telebot.ReplyMarkup {
	rows := make([]telebot.Row, 0, 2)
	var nav []telebot.Btn
	if step > 0 {
		nav = append(nav, btnInstructionPrev)
	}
	if step < len(instructionSteps)-1 {
		nav = append(nav, btnInstructionNext)
	}
	if len(nav) > 0 {
		rows = append(rows, instructionMarkup.Row(nav...))
	}
	rows = append(rows, instructionMarkup.Row(btnInstructionMenu))
	instructionMarkup.Inline(rows...)

	return instructionMarkup
}

func supportMenu() *telebot.ReplyMarkup {
	supportMarkup.Inline(
		supportMarkup.Row(btnSupportCancel),
	)

	return supportMarkup
}

func broadcastConfirmMenu() *telebot.ReplyMarkup {
	broadcastMarkup.Inline(
		broadcastMarkup.Row(btnBroadcastConfirm, btnBroadcastCancel),
	)

	return broadcastMarkup
}
