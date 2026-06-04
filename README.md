# Corvin UI

Corvin UI — веб-панель управления VPN-инфраструктурой. Проект написан на Go, использует Gin, PostgreSQL, RabbitMQ и MinIO, а интерфейс собран на базе AdminLTE/Vue.

Панель предназначена для внутреннего администрирования: учет пользователей, серверов, VPN-профилей, Telegram-интеграции и обращений пользователей.

## Возможности

- Веб-панель администратора: dashboard, пользователи, серверы, обращения.
- REST API для управления пользователями, серверами и VPN-профилями.
- Telegram API для бота: создание пользователей, выдача VPN, прием и обновление жалоб.
- Генерация VPN-ссылок VLESS и Trojan.
- Сбор статистики online-пользователей по серверам через cron job.
- Хранение файлов жалоб в MinIO.
- Отправка событий в RabbitMQ.
- Конфигурация через environment variables и systemd `EnvironmentFile`.
- Безопасный режим по умолчанию: панель слушает только `127.0.0.1:8080`.

## Быстрый старт

Установка одной командой:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/dimasmir03/corvin-ui/main/install.sh)
```

Скрипт установки:

- скачивает последний release;
- устанавливает бинарник в `/usr/local/corvin-ui`;
- создает systemd service `corvin-ui`;
- создает конфиг `/etc/corvin-ui/corvin-ui.env`;
- генерирует случайные секреты для DB, RabbitMQ, MinIO и session cookie;
- не перетирает существующий env-файл без подтверждения.

После установки панель по умолчанию доступна только локально на сервере. Для доступа с рабочей машины используйте SSH tunnel:

```bash
ssh -L 8080:127.0.0.1:8080 root@SERVER_IP
```

Затем откройте:

```text
http://127.0.0.1:8080
```

## Почему панель слушает localhost

По умолчанию используется:

```env
HTTP_ADDR=127.0.0.1:8080
AUTH_MODE=none
```

Это значит, что встроенная авторизация отключена, а доступ разрешен только через локальный bind и SSH tunnel.

В коде есть защита: если `AUTH_MODE=none`, приложение не запустится с `HTTP_ADDR=0.0.0.0:8080` или публичным IP. Это сделано специально, чтобы случайно не открыть панель в интернет без авторизации.

Если нужно слушать внешний интерфейс, включите авторизацию:

```env
AUTH_MODE=session
SESSION_SECRET=long-random-secret
HTTP_ADDR=0.0.0.0:8080
```

## Конфигурация

Основной конфиг при systemd-установке:

```text
/etc/corvin-ui/corvin-ui.env
```

Пример переменных находится в [.env.example](./.env.example).

Основные переменные:

| Переменная | Назначение | Значение по умолчанию |
| --- | --- | --- |
| `HTTP_ADDR` | Адрес веб-панели | `127.0.0.1:8080` |
| `AUTH_MODE` | Режим авторизации: `none` или session-режим | `none` |
| `SESSION_SECRET` | Секрет cookie-сессий | пусто |
| `DB_HOST` | Host PostgreSQL | `127.0.0.1` |
| `DB_PORT` | Port PostgreSQL | `5432` |
| `DB_USER` | Пользователь PostgreSQL | `` |
| `DB_PASSWORD` | Пароль PostgreSQL | пусто |
| `DB_NAME` | База PostgreSQL | `` |
| `DB_SSLMODE` | SSL mode PostgreSQL | `disable` |
| `RABBITMQ_URL` | AMQP URL для панели | пусто |
| `MINIO_ENDPOINT` | Endpoint MinIO | `127.0.0.1:9000` |
| `MINIO_ACCESS_KEY` | Access key MinIO | `` |
| `MINIO_SECRET_KEY` | Secret key MinIO | пусто |
| `MINIO_USE_SSL` | Использовать SSL для MinIO | `false` |
| `MINIO_BUCKET` | Bucket для файлов жалоб | `complaints` |

Дополнительные настройки, которые сохраняются в таблицу `settings` при первом запуске:

- `AMQP_EXCHANGE_COMPLAINTS`
- `AMQP_EXCHANGE_USERS`
- `CERT_FILE`
- `KEY_FILE`
- `CA_FILE`
- MinIO и DB-параметры для совместимости с текущими сервисами панели.

## Systemd

Service-файл проекта: [corvin-ui.service](./corvin-ui.service).

Управление сервисом:

```bash
systemctl status corvin-ui
systemctl restart corvin-ui
systemctl stop corvin-ui
```

Логи приложения пишутся в:

```text
/var/log/corvin-ui/vpnpanel.log
```

Также можно смотреть journal:

```bash
journalctl -u corvin-ui -f
```

## CLI wrapper

При установке скачивается wrapper `/usr/bin/corvin-ui`.

Основные команды:

```bash
corvin-ui start
corvin-ui stop
corvin-ui restart
corvin-ui status
corvin-ui log
corvin-ui settings show
corvin-ui settings update <field> <value>
```

Команда `settings` работает с настройками, сохраненными в базе данных.

## Docker Compose

В репозитории есть [docker-compose.yml](./docker-compose.yml) для запуска инфраструктуры и контейнерного варианта панели.

Compose использует env-file:

```yaml
env_file:
  - ${CORVIN_UI_ENV_FILE:-/etc/corvin-ui/corvin-ui.env}
```

Для локальной проверки можно использовать `.env.example`:

```bash
docker compose --env-file .env.example config
docker compose --env-file .env.example up -d
```

Локальные порты в compose привязаны к `127.0.0.1`, где это важно:

- панель: `127.0.0.1:8080:8080`
- PostgreSQL: `127.0.0.1:5432:5432`
- RabbitMQ management: `127.0.0.1:15672:15672`
- MinIO console: `127.0.0.1:9001:9001`

AMQP over TLS сейчас опубликован как `1765:5671`, потому что панель и бот используют RabbitMQ URL.

## HTTP routes

Публичные routes:

- `GET /login`
- `POST /login`

Панель:

- `GET /panel/`
- `GET /panel/servers`
- `GET /panel/servers/new`
- `GET /panel/servers/edit/:id`
- `GET /panel/users`
- `GET /panel/users/new`
- `GET /panel/users/edit/:id`
- `GET /panel/complaints`

API:

- `/api/servers/*` — серверы и статистика online.
- `/api/users/*` — пользователи.
- `/api/vpn/*` — VPN-профили и регенерация ссылок.
- `/api/telegram/*` — интеграция с Telegram-ботом.
- `/api/complaints/*` — обращения пользователей.
- `/api/media/*` — файлы из storage.

## Разработка

Требования:

- Go `1.25.3` или совместимая версия.
- PostgreSQL.
- RabbitMQ.
- MinIO.

Проверки:

```bash
gofmt -w ./cmd ./internal
go test ./...
go build -o /tmp/vpnpanel-check ./cmd/vpnpanel
```

Запуск локально зависит от доступности PostgreSQL, RabbitMQ и MinIO. Переменные окружения можно взять из `.env.example` и адаптировать под локальную инфраструктуру.

## Структура проекта

```text
cmd/vpnpanel/          entrypoint приложения
internal/app/          Gin server, routes, cron startup
internal/config/       конфигурация из env и validation
internal/db/           подключение PostgreSQL и миграции GORM
internal/handlers/     HTTP handlers и API controllers
internal/repository/   работа с базой данных
internal/storage/      MinIO client
internal/broker/       RabbitMQ producer
internal/templates/    HTML templates панели
internal/static/       AdminLTE, Vue, CSS, JS, assets
internal/utils/        генерация VPN-ссылок и helpers
```

## Безопасность

- Не публикуйте `HTTP_ADDR=0.0.0.0:8080` вместе с `AUTH_MODE=none`.
- Не коммитьте реальные env-файлы и секреты.
- После установки храните `/etc/corvin-ui/corvin-ui.env` с правами `600`.
- Для стандартного режима используйте SSH tunnel вместо открытия панели наружу.

## Лицензия

Проект распространяется по лицензии GPL-3.0. См. [LICENSE](./LICENSE).
