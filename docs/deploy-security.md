# Deploy And Security Notes

## Environment

Use `.env.example` as the baseline for local and staging configuration. `TELEGRAM_SECRET` is currently used by `user_service` for both legacy HMAC auth and Telegram WebApp `initData` validation, so set it to the Telegram bot token before enabling WebApp login.

Do not commit production secrets. Override hardcoded local defaults from `docker-compose.yml` in production through an environment-specific compose file, orchestrator secrets, or CI/CD secret storage.

## TLS

The bundled nginx config is local HTTP only. Production traffic must terminate TLS at an external reverse proxy or load balancer, then forward to `api-gateway:8080` inside the private network.

Minimum proxy requirements:

- Redirect HTTP to HTTPS.
- Set `X-Forwarded-Proto`, `X-Forwarded-For`, and `Host`.
- Do not expose gRPC service ports or PostgreSQL/Redis/MinIO ports publicly.
- Limit request body size according to expected file upload size.

## Backups And Restore

Back up every PostgreSQL volume independently because each service owns its own database. Back up MinIO object data together with `file_service` metadata so signed file references remain restorable.

Suggested restore drill:

1. Stop application services that write data.
2. Restore PostgreSQL dumps for `user-db`, `schedule-db`, `homework-db`, `payment-db`, and `file-db`.
3. Restore MinIO bucket data.
4. Run migrations with `POSTGRES_AUTO_MIGRATE=true`.
5. Start services and verify sign-in, lesson listing, homework file download, and receipt listing.

Run a restore check before each release that changes migrations or file lifecycle logic.
