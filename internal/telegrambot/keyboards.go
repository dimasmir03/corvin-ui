package telegrambot

import telebot "gopkg.in/telebot.v4"

const (
	callbackMenuVPN         = "menu:vpn"
	callbackMenuInstruction = "menu:instruction"
	callbackMenuSupport     = "menu:support"

	callbackVPNVLESS     = "vpn:vless"
	callbackVPNTrojan    = "vpn:trojan"
	callbackCreateVLESS  = "vpn:create:vless"
	callbackCreateTrojan = "vpn:create:trojan"
	callbackVPNBack      = "vpn:back"
)

var (
	menuMarkup = &telebot.ReplyMarkup{}

	btnMenuVPN         = menuMarkup.Data("🔐 VPN", callbackMenuVPN)
	btnMenuInstruction = menuMarkup.Data("📖 Инструкция", callbackMenuInstruction)
	btnMenuSupport     = menuMarkup.Data("💬 Поддержка", callbackMenuSupport)

	vpnMarkup       = &telebot.ReplyMarkup{}
	vpnCreateMarkup = &telebot.ReplyMarkup{}

	btnVPNVLESS     = vpnMarkup.Data("🔗 VLESS", callbackVPNVLESS)
	btnVPNTrojan    = vpnMarkup.Data("🔗 Trojan", callbackVPNTrojan)
	btnCreateVLESS  = vpnMarkup.Data("➕ Создать VLESS", callbackCreateVLESS)
	btnCreateTrojan = vpnMarkup.Data("➕ Создать Trojan", callbackCreateTrojan)
	btnVPNBack      = vpnMarkup.Data("⬅️ Назад", callbackVPNBack)
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
		vpnMarkup.Row(btnVPNBack),
	)

	return vpnMarkup
}

func vpnCreateMenu() *telebot.ReplyMarkup {
	vpnCreateMarkup.Inline(
		vpnCreateMarkup.Row(btnCreateVLESS),
		vpnCreateMarkup.Row(btnCreateTrojan),
		vpnCreateMarkup.Row(btnVPNBack),
	)

	return vpnCreateMarkup
}
