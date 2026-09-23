# Corvin Mobile API

Namespace: `/api/mobile/v1`. Mobile routes do not use the admin session cookie.

| Method | Path | Authentication | Purpose |
|---|---|---|---|
| POST | `/auth/telegram/start` | public | Create PKCE Telegram login |
| GET | `/auth/telegram/status` | public | Poll `pending/approved/denied/expired` only |
| POST | `/auth/telegram/exchange` | public | One-time exchange for access/refresh tokens |
| POST | `/auth/refresh` | public | Rotate opaque refresh token |
| POST | `/auth/logout` | bearer | Revoke current session |
| GET | `/bootstrap` | bearer | User, subscription, device, VPN summary and capabilities |
| GET | `/servers` | bearer | Managed catalog without addresses or credentials |
| GET | `/subscription` | bearer | Current subscription and device usage |
| GET | `/devices` | bearer | User devices |
| DELETE | `/devices/{device_id}` | bearer | Revoke device, sessions and queue credential deletion |
| POST | `/vpn/connect` | bearer + `Idempotency-Key` | Return config or provisioning operation |
| GET | `/operations/{operation_id}` | bearer | Poll provisioning |
| GET | `/vpn/diagnostics` | bearer | Observed public IP; always `no-store` |
| GET | `/connectivity-check` | public | Stable `204` endpoint for Xray delay checks |

The `auto` entry in `/servers` is virtual. In global `pinned` mode it resolves to the selected main server; in `automatic` mode the backend chooses the healthiest server allowed for the user. A concrete `server_id` selected by the app is never overridden by the global Auto setting.

## Required configuration

```dotenv
MOBILE_API_ENABLED=true
MOBILE_JWT_SECRET=<at-least-32-random-characters>
MOBILE_TELEGRAM_BOT_USERNAME=<bot username without @>
MOBILE_PUBLIC_BASE_URL=https://panel.example.com
MOBILE_ACCESS_TTL_SECONDS=900
MOBILE_REFRESH_TTL_HOURS=720
MOBILE_LOGIN_TTL_MINUTES=10
MOBILE_MIN_APP_VERSION=1.0.0
MOBILE_TRUSTED_PROXIES=127.0.0.1/32,100.64.0.0/10
```

`MOBILE_CONNECTIVITY_CHECK_URL` can override the URL derived from `MOBILE_PUBLIC_BASE_URL`. Only put a reverse proxy CIDR in `MOBILE_TRUSTED_PROXIES`; forwarded IP headers from all other clients are ignored.

## Telegram approval

The Telegram bot recognizes `/start login_<one-time-token>`, ensures the Telegram user exists and approves the login. The approval token, refresh tokens and VPN credentials are stored only as hashes or hidden model fields where applicable and are never included in application logs.

## Contract notes

The implementation follows `vpnmobile/docs/mobile-api.openapi.yaml`. GeoIP enrichment is optional in that contract and currently returns `null` country/city values; `observed_ip` is always returned. Remote credential deletion requires `vpnconsumer` support for `delete_client`; until that is deployed, device/session revocation is immediate in the API but an already installed Xray credential can remain valid on the remote inbound.
