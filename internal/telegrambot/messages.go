package telegrambot

const (
	msgStart = `Привет, %s!
Добро пожаловать в Corvin VPN.`

	msgRegistrationFailed    = "Не удалось зарегистрировать пользователя. Попробуйте позже."
	msgStartMenu             = "Главное меню"
	msgVPNComingSoon         = "Раздел VPN будет подключён позже."
	msgInstructionComingSoon = "Инструкция будет подключена позже."
	msgSupportComingSoon     = "Поддержка будет подключена позже."

	msgVPNReady = `🔐 Ваш VPN уже создан.

Выберите протокол:`
	msgVPNMissing = `🔐 VPN пока не создан.

Создание VPN будет подключено следующим шагом.`
	msgVPNFetchFailed = "Не удалось получить VPN. Попробуйте позже."
	msgVLESSMissing   = "VLESS-ссылка пока не создана."
	msgTrojanMissing  = "Trojan-ссылка пока не создана."

	msgVPNCreateRequested = `⏳ Запрос на создание VPN отправлен.

Я пришлю ссылку, когда сервер подготовит конфигурацию.`
	msgVPNAlreadyExists = `VPN уже создан.

Откройте раздел VPN, чтобы получить ссылку.`
	msgVPNUnsupportedProtocol = "Неподдерживаемый протокол VPN."
	msgVPNCreateFailed        = "Не удалось создать VPN. Попробуйте позже."
)
