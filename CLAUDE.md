# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

StudyFlow is a microservices-based API platform for tutors and students, with Telegram bot as the primary client. Tutors manage lessons, homework, and payments; students book lessons, submit homework, and pay.

## Architecture

- **6 microservices**: api_gateway (REST), user_service, schedule_service, homework_service, payment_service, file_service (all gRPC)
- **Tech stack**: Go 1.24, gRPC, Chi v5 (REST), PostgreSQL (separate instance per service), Redis, MinIO/S3
- **Communication**: External clients → REST API Gateway → gRPC internal services
- **Auth flow**: Telegram HMAC header → API Gateway validates → adds `x-user-id`/`x-user-role` to gRPC metadata → services trust this context

## Build & Run Commands

```bash
# Start all services locally (requires TELEGRAM_SECRET in .env)
docker-compose up

# Build individual service
docker-compose build <service-name>

# Build all services
docker-compose build --parallel
```

## Testing

Tests must achieve **≥30% coverage** per package (enforced in CI).

```bash
# Run payment_service tests (CI requirement)
cd payment_service/internal/service && go test -cover ./...

# Run schedule_service tests (CI requirement)
cd schedule_service/internal/service/service && go test -cover ./...
```

Mocking: Use `go.uber.org/mock/gomock` with interface-based mocks.

## Protocol Buffers

Each service has its own `.proto` file. Generated `.pb.go` files are committed.

```bash
# Regenerate proto files (run from service directory)
make proto
```

## Code Patterns

**Logging**: Use zap via `common_library/logging`. Each request gets UUIDv7 in API Gateway, passed via gRPC metadata.

**Error handling**: Return gRPC status codes (NOT_FOUND, INVALID_ARGUMENT, PERMISSION_DENIED, etc.).

**Resilience**: All inter-service calls must use retry with exponential backoff and circuit breaker.

**Data priority**: Lesson parameters (price, meeting link, payment info) resolve in order: lesson-specific > tutor-student pair > tutor defaults.

**File handling**: Two-step process via file_service: InitUpload returns file_id + signed URL → client uploads → use file_id in other services.

## Service Ports

- API Gateway: 8080 (HTTP), exposed via nginx on port 80
- All gRPC services: 50051
- MinIO console: 9001
- Redis: 6379

## Project Structure

```
api_gateway/         # REST entry point, routes in internal/handler/
user_service/        # Users, tutors, students, auth
schedule_service/    # Lesson slots, bookings
homework_service/    # Assignments, submissions, feedback
payment_service/     # Payment receipts, verification
file_service/        # S3 signed URLs, file metadata
common_library/      # Shared logging, gRPC utilities, interceptors
```

Each service follows: `cmd/server/main.go` entry point, `internal/` for handlers/services/repositories, `migrations/` for PostgreSQL schemas.

## Key Files

- `docker-compose.yml` - Full local environment
- `api_gateway/OpenAPI.yml` - REST API specification
- `developer_readme.md` - Development guidelines (Russian)
- `.gitlab-ci.yml` - CI pipeline (test coverage checks + builds)
