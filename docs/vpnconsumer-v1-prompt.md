# Задача для чата проекта vpnconsumer (совместимость с vpnpanel v1)

Нужно обновить `vpnconsumer` под новый command flow `vpnpanel`. Не вводи собственные `jobs` или batch-состояния: панель использует `vpn_profile_nodes` как доменное состояние и журнал команды, а consumer остаётся идемпотентным исполнителем команд для одного физического сервера.

## Команды

Consumer уже обрабатывает `create_client`. Добавь полноценную поддержку:

- `enable_client` — включить существующего клиента в нужном inbound;
- `disable_client` — временно запретить подключение, сохранив credentials;
- `delete_client` — удалить клиента из inbound;
- повтор любой команды с теми же `job_id`, `profile_id`, `server_id` и `client_code` должен быть безопасным и возвращать успешное фактическое состояние;
- команда адресована через routing key `create.server.<server_id>` для обратной совместимости с существующей привязкой очереди. Тип операции определяется полем `action`/`command_type`, а не routing key.

Не добавляй обработку цепочек/hops. Один `server_id` — одна возможная точка подключения, один agent и управляемый inbound.

## Проверка адресата

- выполнять команду только если `target_server_id` (или совместимый `server_id`) равен локальному `SERVER_ID`;
- несовпадение возвращать как terminal error без изменения 3x-ui;
- `node_id` не вводить и не использовать как новую сущность;
- протокол команды обязан совпадать с протоколом управляемого inbound.

## Поиск клиента

Основной стабильный ключ — `client_code`/email из команды. Для VLESS также сверять UUID, для Trojan — password, но не писать эти значения в логи. Если remote ID уже известен, его можно использовать как дополнительную оптимизацию, но consumer не должен зависеть только от него.

## Результат

Публиковать результат в существующий result exchange/queue со следующими полями:

```json
{
  "event_type": "job_result",
  "job_id": 123,
  "profile_id": 45,
  "server_id": "server-nl-1",
  "command_type": "disable_client",
  "protocol": "vless",
  "client_code": "cvn_xxxxxxxx",
  "status": "success",
  "remote_client_id": "optional",
  "config_link": null,
  "error": null
}
```

Для `create_client` можно вернуть `config_link`, но не логировать его. Для control-команд `config_link` не нужен. При ошибке вернуть `status=failed` и безопасное описание без UUID/password/config URL.

## Семантика операций

### enable_client

- если клиент существует и уже включён — success;
- если существует и выключен — включить и проверить сохранённое состояние;
- если клиент отсутствует — failed с понятным кодом/сообщением, не создавать его молча.

### disable_client

- если уже выключен — success;
- отключить так, чтобы текущий config перестал авторизовываться, но повторный `enable_client` восстановил доступ с теми же credentials;
- если API 3x-ui не имеет отдельного enabled-флага для клиента, выбери обратимую реализацию и явно задокументируй её.

### delete_client

- если клиент уже отсутствует — success;
- удалить только точного клиента из целевого inbound;
- не удалять inbound и не затрагивать других клиентов.

## Надёжность

- ack RabbitMQ только после подтверждённого изменения 3x-ui и успешной публикации результата;
- transient ошибки 3x-ui/RabbitMQ должны приводить к retry/Nack согласно текущей политике;
- terminal validation errors не должны бесконечно переотправляться;
- дедупликация должна переживать рестарт процесса (если локального durable store сейчас нет — предложи минимальное решение или докажи идемпотентность через чтение фактического состояния 3x-ui);
- запрети логирование credentials, Authorization, config links и полного payload команды.

## Обязательные тесты

1. create повторяется без дубля клиента;
2. enable: enabled → success и disabled → enabled;
3. disable: enabled → disabled и повтор → success;
4. delete: existing → removed и missing → success;
5. чужой `server_id` отклоняется без вызова 3x-ui;
6. неверный protocol/inbound отклоняется;
7. result содержит тот же `job_id/profile_id/server_id/command_type`;
8. логи и ошибки не содержат UUID, password или config URL;
9. transient ошибка публикуется/повторяется согласно текущей RabbitMQ policy.

В конце перечисли изменённые файлы, формат поддержанных команд, стратегию idempotency и всё, что требуется настроить в 3x-ui.
