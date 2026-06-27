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

func vpnMenu(hasVless bool, hasTrojan bool) *telebot.ReplyMarkup {
	rows := make([]telebot.Row, 0, 5)
	if hasVless {
		rows = append(rows, vpnMarkup.Row(btnVPNVLESS))
	}
	if hasTrojan {
		rows = append(rows, vpnMarkup.Row(btnVPNTrojan))
	}
	if !hasVless {
		rows = append(rows, vpnMarkup.Row(btnCreateVLESS))
	}
	if !hasTrojan {
		rows = append(rows, vpnMarkup.Row(btnCreateTrojan))
	}
	rows = append(rows, vpnMarkup.Row(btnMainMenu))
	vpnMarkup.Inline(rows...)

	return vpnMarkup
}

func vpnCreateMenu(hasVless bool, hasTrojan bool) *telebot.ReplyMarkup {
	rows := make([]telebot.Row, 0, 3)
	if !hasVless {
		rows = append(rows, vpnCreateMarkup.Row(btnCreateVLESS))
	}
	if !hasTrojan {
		rows = append(rows, vpnCreateMarkup.Row(btnCreateTrojan))
	}
	rows = append(rows, vpnCreateMarkup.Row(btnMainMenu))
	vpnCreateMarkup.Inline(rows...)

	return vpnCreateMarkup
}

func linkMenu() *telebot.ReplyMarkup {
	return linkOverviewMenu(true, true, false)
}

func linkOverviewMenu(hasVless bool, hasTrojan bool, hasPending bool) *telebot.ReplyMarkup {
	rows := make([]telebot.Row, 0, 3)
	if hasVless {
		rows = append(rows, linkMarkup.Row(btnLinkVLESS))
	}
	if hasTrojan {
		rows = append(rows, linkMarkup.Row(btnLinkTrojan))
	}
	if hasPending {
		rows = append(rows, linkMarkup.Row(btnMainMenu))
	}
	linkMarkup.Inline(rows...)
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
