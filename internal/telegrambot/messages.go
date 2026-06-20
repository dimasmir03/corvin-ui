package telegrambot

const (
	msgStart = `Привет, %s  
Добро пожаловать в Raven VPN! 
Мы рады здесь вас видеть!  

🦾 устойчивость к блокировкам  
😏 Швейцарский сервер
🔐 полноценная конфиденциальность 
🚀 высокая скорость 

VPN в режиме тестирования, тем самым он на данный момент бесплатный.

⬇️ ⬇️  Жмите кнопку!  ⬇️ ⬇️`

	msgRegistrationFailed    = "Не удалось зарегистрировать пользователя. Попробуйте позже."
	msgTelegramIDFailed      = "Не удалось определить Telegram ID."
	msgStartMenu             = "Меню управления:"
	msgPingOK                = "ok"
	msgVPNComingSoon         = "Раздел VPN будет подключён позже."
	msgInstructionComingSoon = "Инструкция будет подключена позже."
	msgSupportComingSoon     = "Поддержка будет подключена позже."

	msgVPNReady = `📋 Ваши VPN:
%s

Выберите протокол:`
	msgVPNMissing     = "🚀 VPN не настроен\n\nВыберите протокол для создания:"
	msgVPNFetchFailed = "❌ Ошибка проверки VPN"
	msgVLESSMissing   = "⏳ Ссылка создаётся. Попробуйте через минуту."
	msgTrojanMissing  = "⏳ Ссылка создаётся. Попробуйте через минуту."

	msgVPNCreateRequested     = "⏳ Ссылка создаётся. Попробуйте через минуту."
	msgVPNAlreadyExists       = "✅ VPN уже создан\n\n%s\n\nВыберите протокол:"
	msgVPNUnsupportedProtocol = "❌ Ошибка создания VPN"
	msgVPNCreateFailed        = "Ошибка создания VPN.\n Попробуйте позже."

	msgLinkChooseProtocol      = "  VPN не настроен\n\nНажмите /create для создания\nили выберите протокол:"
	msgLinkUnsupportedProtocol = "❌ Ошибка получения ссылки"
	msgLinkMissing             = "⏳ Ссылка создаётся. Попробуйте через минуту."
	msgLinkFetchFailed         = "что-то пошло не так! попробуйте еще раз"
)

type InstructionStep struct {
	Text      string
	ImagePath string
}

var instructionSteps = []InstructionStep{
	{
		Text: ` 
Скопируйте ссылку отправленную ботом, просто нажав на текст ссылки, чтобы она автоматически скопировалась.`,
		ImagePath: "0.jpg",
	},
	{
		Text: `
Приложения:
Android:
<a href="https://play.google.com/store/apps/details?id=dev.hexasoftware.v2box">V2Box</a>
<a href="https://play.google.com/store/apps/details?id=com.v2ray.ang&pli=1">v2rayNG</a> / <a href="https://github.com/2dust/v2rayNG">Github</a>
<a href="https://github.com/MatsuriDayo/NekoBoxForAndroid">NekoBox</a>
<a href="https://play.google.com/store/apps/details?id=com.v2raytun.android">v2RayTun</a>

iOS:
<a href="https://apps.apple.com/ca/app/v2box-v2ray-client/id6446814690">V2Box - V2ray Client</a>
<a href="https://apps.apple.com/ru/app/foxray/id6448898396">FoXray</a>
<a href="https://apps.apple.com/ru/app/foxray/id6448898396">Streisand</a>
<a href="https://apps.apple.com/ru/app/v2raytun/id6476628951">v2RayTun</a>
`,
		ImagePath: "01.jpg",
	},
	{
		Text:      `Это инструкция для приложения V2Box. Переходи в приложение V2Box` + "\n" + `1️⃣) Снизу по центру Вкладка "Config" `,
		ImagePath: "1.jpg",
	},
	{
		Text:      `2️⃣) Справа Сверху нажимаем на"+"`,
		ImagePath: "2.jpg",
	},
	{
		Text:      `3️⃣) Import v2ray uri from clipboard: твоя_ссылка_на_подписку -> OK/Add.`,
		ImagePath: "3.jpg",
	},
	{
		Text:      "4️⃣) Как только получили список протоколов после добавления ссылки внизу появятся сервера для подключения, обычно возле них есть кнопка слева от надписи (VLESS) -> Тыкаем на неё -> снизу перетаскиваем ползунок вручную (Slide to Connect) ",
		ImagePath: "4.jpg",
	},
	{
		Text:      "5️⃣) Готово! ✅",
		ImagePath: "5.jpg",
	},
}

const (
	msgInstructionFirst = "Конец"
	msgInstructionLast  = "Конец"
)

const (
	msgSupportPrompt               = "Опишите вашу проблему... (Можно приложить скриншот)"
	msgSupportSent                 = "✅ Спасибо! Ваша жалоба отправлена администратору (ID: %d)"
	msgSupportPhotoSent            = "✅ Фото и описание отправлены администратору!\n(ID жалобы: %d)"
	msgSupportPhotoPrompt          = "Опишите вашу проблему... (Можно приложить скриншот)"
	msgSupportCreateFailed         = "❌ Ошибка отправки жалобы."
	msgSupportCanceled             = "❌ Отправка жалобы отменена."
	msgSupportReplyPrompt          = "Введите ваш ответ пользователю:"
	msgSupportReplySent            = "✅ Ответ отправлен в панель!"
	msgSupportReplySavedSendFailed = "Ответ сохранён, но не удалось отправить сообщение пользователю."
	msgSupportReplyFailed          = "❌ Ошибка: неверный ID пользователя."
	msgSupportReplyCanceled        = "❌ Отправка жалобы отменена."
	msgAdminGetUsersEmpty          = "Пользователей пока нет."
	msgAdminGetUsersFailed         = "Не удалось получить список пользователей. Попробуйте позже."
	msgAdminSendUserUsage          = "Использование:\n/senduser <tg_id> <message>"
	msgAdminSendUserInvalidID      = "Некорректный Telegram ID."
	msgAdminSendUserNotFound       = "Пользователь не найден."
	msgAdminSendUserSent           = "✅ Сообщение отправлено."
	msgAdminSendUserFailed         = "Не удалось отправить сообщение пользователю."
	msgAdminBroadcastUsage         = "Использование:\n/send <message>"
	msgAdminBroadcastNoRecipients  = "Пользователей для рассылки нет."
	msgAdminBroadcastNoDraft       = "Нет активной рассылки."
	msgAdminBroadcastCancelled     = "Рассылка отменена."
	msgActionCancelled             = "Действие отменено."
	msgNoActiveAction              = "Нет активного действия."
	msgUnknownText                 = "Извините, я не понимаю эту команду."
	msgMaintenance                 = "Идут технические работы. Попробуйте позже."
)
