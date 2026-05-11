# Authentication

StudyFlow supports two authentication formats. Both are sent via the `Authorization` HTTP header.

## 1. Telegram Mini App initData (recommended for production)

The proper way for Telegram Mini Apps. The client does **not** need to know `TELEGRAM_SECRET`.

```
Authorization: tma <initData>
```

Where `<initData>` is the value of `Telegram.WebApp.initData` from the Telegram WebApp SDK.

### How it works

1. `Telegram.WebApp.initData` contains query-string data signed by Telegram
2. Backend validates the `hash` field using HMAC-SHA256 with the bot token as secret key
3. Extracts `user.id` from the JSON `user` field
4. Looks up the user in the database

### Frontend example (Telegram Mini App)

```typescript
const tg = window.Telegram.WebApp;
const initData = tg.initData;

const response = await fetch("https://api.studyflow.dev/users/me", {
  headers: {
    Authorization: `tma ${initData}`,
  },
});
```

### TTL

initData is valid for 24 hours from `auth_date`. After that, the client must refresh by re-opening the Mini App.

## 2. Legacy HMAC (for development/testing only)

```
Authorization: telegram <telegram_id>:<utc_timestamp>:<hmac>
```

Where:
- `telegram_id` — Telegram user ID
- `utc_timestamp` — current Unix timestamp in seconds (±5 minute clock skew)
- `hmac` — hex-encoded HMAC-SHA256 of `{telegram_id}:{utc_timestamp}` using `TELEGRAM_SECRET` as key

### Generating tokens (dev)

```bash
./scripts/generate-auth-token.sh <TELEGRAM_ID> <TELEGRAM_SECRET>
# or read from .env
./scripts/generate-auth-token.sh
```

### Disabling legacy HMAC in production

Set `AUTH_DISABLE_LEGACY_HMAC=true` in the `user-service` environment. This makes `telegram ` prefix requests return 401, leaving only `tma ` (initData) as the valid auth method.

## Environment Variables

| Variable | Service | Required | Description |
|----------|---------|----------|-------------|
| `TELEGRAM_SECRET` | user-service | yes | Telegram bot token (`123456:ABC-DEF` format) |
| `AUTH_DISABLE_LEGACY_HMAC` | user-service | no | Set to `true` to disable legacy HMAC auth (default: `false`) |
