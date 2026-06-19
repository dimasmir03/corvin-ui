package telegrambot

const (
	msgStart = `Привет, %s!
Добро пожаловать в Corvin VPN.`

	msgRegistrationFailed    = "Не удалось зарегистрировать пользователя. Попробуйте позже."
	msgTelegramIDFailed      = "Не удалось определить Telegram ID."
	msgStartMenu             = "Главное меню"
	msgPingOK                = "ok"
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

	msgLinkChooseProtocol      = "Выберите протокол:"
	msgLinkUnsupportedProtocol = "Неподдерживаемый протокол VPN.\nДоступные варианты: vless, trojan."
	msgLinkMissing             = "Ссылка пока не создана.\n\nОткройте /vpn и создайте VPN."
	msgLinkFetchFailed         = "Не удалось получить ссылку. Попробуйте позже."
)

type InstructionStep struct {
	Text      string
	ImagePath string
}

var instructionSteps = []InstructionStep{
	{
		Text:      `Скопируйте ссылку, отправленную ботом: просто нажмите на текст ссылки, чтобы она автоматически скопировалась.`,
		ImagePath: "0.jpg",
	},
	{
		Text: `Приложения:

Android:
<a href="https://play.google.com/store/apps/details?id=dev.hexasoftware.v2box">V2Box</a>
<a href="https://play.google.com/store/apps/details?id=com.v2ray.ang&pli=1">v2rayNG</a> / <a href="https://github.com/2dust/v2rayNG">Github</a>
<a href="https://github.com/MatsuriDayo/NekoBoxForAndroid">NekoBox</a>
<a href="https://play.google.com/store/apps/details?id=com.v2raytun.android">v2RayTun</a>

iOS:
<a href="https://apps.apple.com/ca/app/v2box-v2ray-client/id6446814690">V2Box - V2ray Client</a>
<a href="https://apps.apple.com/ru/app/foxray/id6448898396">FoXray</a>
<a href="https://apps.apple.com/ru/app/foxray/id6448898396">Streisand</a>
<a href="https://apps.apple.com/ru/app/v2raytun/id6476628951">v2RayTun</a>`,
		ImagePath: "01.jpg",
	},
	{
		Text: `Это инструкция для приложения V2Box. Перейдите в приложение V2Box.

1️⃣ Снизу по центру откройте вкладку "Config".`,
		ImagePath: "1.jpg",
	},
	{
		Text:      `2️⃣ Справа сверху нажмите "+".`,
		ImagePath: "2.jpg",
	},
	{
		Text:      `3️⃣ Выберите "Import v2ray uri from clipboard": ваша ссылка на подписку -> OK/Add.`,
		ImagePath: "3.jpg",
	},
	{
		Text:      `4️⃣ После добавления ссылки внизу появятся серверы для подключения. Нажмите кнопку рядом с протоколом VLESS, затем вручную перетащите ползунок "Slide to Connect".`,
		ImagePath: "4.jpg",
	},
	{
		Text:      `5️⃣ Готово! ✅`,
		ImagePath: "5.jpg",
	},
}

const (
	msgInstructionFirst = "Это первый шаг."
	msgInstructionLast  = "Это последний шаг."
)

const (
	msgSupportPrompt               = "Напишите ваше обращение одним сообщением.\nМожно описать проблему текстом."
	msgSupportSent                 = "✅ Обращение отправлено в поддержку."
	msgSupportPhotoSent            = "✅ Обращение с фото отправлено в поддержку."
	msgSupportPhotoPrompt          = "Чтобы отправить фото в поддержку, сначала откройте /help."
	msgSupportCreateFailed         = "Не удалось отправить обращение. Попробуйте позже."
	msgSupportCanceled             = "Обращение отменено."
	msgSupportReplyPrompt          = "Напишите ответ на обращение #%d."
	msgSupportReplySent            = "✅ Ответ отправлен пользователю."
	msgSupportReplySavedSendFailed = "Ответ сохранён, но не удалось отправить сообщение пользователю."
	msgSupportReplyFailed          = "Не удалось сохранить ответ. Попробуйте позже."
	msgSupportReplyCanceled        = "Ответ отменён."
	msgAccessDenied                = "Нет доступа."
	msgActionCancelled             = "Действие отменено."
	msgNoActiveAction              = "Нет активного действия."
	msgUnknownText                 = "Выберите действие в меню."
)
