# StudyFlow Backend — Implemented Functionality (2026-05-11)

Derived from code reading of all 6 microservices + 1 shared library at HEAD (commit 24abd45).

## System Overview

StudyFlow is a 7-process Go monorepo: 1 REST gateway + 5 gRPC business services + 1 Kafka consumer. External HTTP clients (primarily a Telegram bot) hit `api_gateway`; the gateway authenticates them via `user_service.AuthorizeByAuthHeader` and fans out to `user_service`, `schedule_service`, `homework_service`, `payment_service`, `file_service` over gRPC. PostgreSQL is one instance per service. Files travel via a two-step signed-URL flow through `file_service` to MinIO/S3. Domain events are published to Kafka topics `lesson-reminders` and `assignment-reminders`; `notification_service` is the only consumer.

**What actually works today end-to-end:** user registration via Telegram → tutor-student pairing → slot/lesson creation → assignment & submission with attachments → payment receipt submission. **What doesn't:** lesson parameter precedence (price/link/payment_info inheritance), automated lesson completion (no scheduler invokes the repo method), and Telegram notification delivery (consumer logs payloads only).

---

## API Gateway (REST, port 8080 behind nginx :80)

### Global middlewares
- Logging middleware — generates UUIDv7 trace-id, sets `X-Trace-Id`, attaches zap logger to context.
- `http.MaxBytesHandler` — 10 MB body limit.

### Routes

| Method | Path | Handler | Downstream | Auth |
|---|---|---|---|---|
| POST | `/users/sign-up/telegram` | `signup.go:26` | `UserService.RegisterViaTelegram` | None |
| GET | `/users/users/me` | `user.go:40` | `UserService.GetMe` | auth |
| GET | `/users/users/{id}` | `user.go:52` | `UserService.GetUser` | auth |
| PATCH | `/users/users/{id}` | `user.go:64` | `UserService.UpdateUser` | auth |
| GET/PATCH | `/users/tutor-profiles/{id}` | `user.go:82/94` | `GetTutorProfileByUserId` / `UpdateTutorProfile` | auth |
| GET | `/users/tutor-students/by-tutor/{id}` | `user.go:108` | `ListTutorStudents` | auth |
| GET | `/users/tutor-students/by-student/{id}` | `user.go:117` | `ListTutorsForStudent` | auth |
| GET/PATCH/DELETE | `/users/tutor-students/{tutor}/{student}` | `user.go:126/138/152` | `GetTutorStudent` / `UpdateTutorStudent` / `DeleteTutorStudent` | auth |
| POST | `/users/tutor-students` | `user.go:166` | `CreateTutorStudent` | auth |
| POST | `/users/tutor-students/{tutor}/accept` | `user.go:175` | `AcceptInvitationFromTutor` | auth |
| POST | `/files/init-upload` | `file.go:47` | `FileService.InitUpload` | auth |
| GET | `/files/{id}/meta` | `file.go:56` | `FileService.GetFileMeta` | auth |
| PUT | `/files/upload/*` | `file.go:43` | MinIO HTTP proxy | **None** |
| GET | `/files/download/*` | `file.go:44` | MinIO HTTP proxy | **None** |
| POST/GET/PATCH/DELETE | `/schedule/slots[/{id}]` | `schedule.go:160-184` | `Create/Get/Update/DeleteSlot` | auth |
| GET | `/schedule/slots/by-tutor/{tutor_id}` | `schedule.go:192` | `ListSlotsByTutor` | auth |
| GET | `/schedule/lessons` | `schedule.go:232` | `ListLessonsByTutor/Student/Pair` | auth |
| POST/GET/PATCH | `/schedule/lessons[/{id}]` | `schedule.go:200-216` | `Create/Get/UpdateLesson` | auth |
| POST | `/schedule/lessons/{id}/cancel` | `schedule.go:224` | `CancelLesson` | auth |
| GET | `/payment/info/{lesson_id}` | `payment.go:65` | `GetPaymentInfo` | auth |
| POST/GET | `/payment/receipts[/{id}]` | `payment.go:73/81` | `SubmitPaymentReceipt` / `GetReceipt` | auth |
| POST | `/payment/receipts/{id}/verify` | `payment.go:89` | `VerifyReceipt` | auth |
| GET | `/payment/receipts/{id}/file-url` | `payment.go:97` | `GetReceiptFile` | auth |
| POST/GET | `/homework/assignments` | `homework.go:107/115` | `Create/ListAssignment*` | auth |
| PATCH/DELETE | `/homework/assignments/{id}` | `homework.go:138/150` | `Update/DeleteAssignment` | auth |
| GET | `/homework/assignments/{id}/file-url` | `homework.go:162` | `GetAssignmentFile` | auth |
| GET | `/homework/assignments/{id}/submissions` | `homework.go:167` | `ListSubmissionsByAssignment` | auth |
| GET | `/homework/assignments/{id}/feedbacks` | `homework.go:206` | `ListFeedbacksByAssignment` | auth |
| POST | `/homework/submissions` | `homework.go:179` | `CreateSubmission` | auth |
| GET | `/homework/submissions/{id}/file-url` | `homework.go:183` | `GetSubmissionFile` | auth |
| POST/PATCH | `/homework/feedbacks[/{id}]` | `homework.go:189/193` | `Create/UpdateFeedback` | auth |
| GET | `/homework/feedbacks/{id}/file-url` | `homework.go:218` | `GetFeedbackFile` | auth |
| GET | `/health` | `main.go:86` | — | None |

### OpenAPI ↔ code delta
All OpenAPI-declared routes exist. Two undocumented routes: `PUT/GET /files/upload/*` and `/files/download/*`.

### Background goroutines
- 1× `srv.ListenAndServe()` goroutine. Graceful shutdown via `http.Server.Shutdown` (no gRPC server here).

---

## user_service (gRPC :50051)

### RPCs

| RPC | Roles | DB tables |
|---|---|---|
| `RegisterViaTelegram` | unauthenticated | `users`, `telegram_accounts`, `tutor_profiles` (tx) |
| `AuthorizeByAuthHeader` | unauthenticated | `telegram_accounts`, `users` |
| `GetMe` | any auth | `users` |
| `GetUser` | any auth | `users` → `UserPublic` |
| `UpdateUser` | owner | `users` |
| `GetTutorProfileByUserId` | owner | `tutor_profiles` |
| `UpdateTutorProfile` | owner | `tutor_profiles` |
| `CreateTutorStudent` | tutor (owner) | `tutor_students` (status=invited) |
| `GetTutorStudent` | pair members | `tutor_students` |
| `UpdateTutorStudent` | tutor (owner) | `tutor_students` |
| `DeleteTutorStudent` | tutor (owner) | `tutor_students` |
| `ListTutorStudents` | tutor (owner) | `tutor_students` |
| `ListTutorsForStudent` | student (owner) | `tutor_students` |
| `ResolveTutorStudentContext` | pair members | `tutor_profiles`, `tutor_students` |
| `AcceptInvitationFromTutor` | student | `tutor_students` (invited → active) |

### DB schema
- `users` (id, role, auth_provider, status, first_name, last_name, timezone)
- `telegram_accounts` (id, user_id FK, telegram_id BIGINT UNIQUE, username)
- `tutor_profiles` (id, user_id FK UNIQUE, payment_info, lesson_price_rub, lesson_connection_link)
- `tutor_students` (id, tutor_id, student_id, lesson_price_rub, lesson_connection_link, status; UNIQUE pair)

### Kafka / Redis / workers
None. No Kafka, no Redis, no background goroutines.

### Auth model
- Telegram HMAC validated in `internal/authorization/telegram.go` with a 5-minute timestamp window.
- All authenticated RPCs read identity from gRPC metadata via `ctxdata.GetUserID/GetUserRole`.

---

## schedule_service (gRPC :50051)

### RPCs

| RPC | Roles | DB tables |
|---|---|---|
| `GetSlot` | tutor (own) or paired student | `slots` |
| `CreateSlot` | tutor only | `slots` |
| `UpdateSlot` | tutor (owner) — blocked if booked | `slots` |
| `DeleteSlot` | tutor (owner) — blocked if booked | `slots` |
| `ListSlotsByTutor` | tutor (own) or paired student | `slots` |
| `GetLesson` | tutor or student of lesson | `lessons`, `slots` |
| `CreateLesson` | tutor or student of pair | `lessons`, `slots` (tx, emits Kafka) |
| `UpdateLesson` | tutor (owner) | `lessons`, `slots` |
| `CancelLesson` | tutor or student | `lessons`, `slots` (tx, no Kafka) |
| `ListLessonsByTutor` | tutor (own) | `lessons` |
| `ListLessonsByStudent` | student (own) | `lessons` |
| `ListLessonsByPair` | tutor or student of pair | `lessons` |
| `ListCompletedUnpaidLessons` | tutor | `lessons` **(returns all tutors — bug)** |
| `MarkAsPaid` | (no check — bug) | `lessons` |

### DB schema
- `slots` (id, tutor_id, starts_at, ends_at, is_booked, created_at, edited_at)
- `lessons` (id, slot_id FK, student_id, status, is_paid, connection_link, price_rub, payment_info, created_at, edited_at)

### Lesson-parameter precedence
**Not implemented.** `CreateLesson` stores nil; `convertrepoLessonToProto` returns whatever the DB holds. No call to user_service to merge tutor-student pair / tutor defaults at any layer.

### Kafka
- Produces: `lesson-reminders` (event type "booked") on `CreateLesson` only. Does not emit on `CancelLesson`.
- Consumes: none.

### Background goroutines / cron
**None.** `repo.UpdateCompletedLessons` exists on the interface but has no caller. Lessons never transition to `completed` status in the running system.

---

## homework_service (gRPC :50051; entry at `cmd/service/`)

### RPCs

| RPC | Roles | DB tables |
|---|---|---|
| `CreateAssignment` | tutor | `assignments` |
| `GetAssignment` | tutor or student | `assignments` |
| `UpdateAssignment` | tutor (owner — broken check) | `assignments` |
| `DeleteAssignment` | tutor (owner) | `assignments` |
| `ListAssignmentsByTutor/Student/Pair` | corresponding role | `assignments`, `submissions`, `feedbacks` (CTE) |
| `CreateSubmission` | student | `submissions` |
| `ListSubmissionsByAssignment` | tutor or student | `submissions` |
| `GetSubmissionFile` | tutor or student | `submissions`, `assignments` |
| `CreateFeedback` | tutor | `feedbacks` |
| `UpdateFeedback` | tutor | `feedbacks` |
| `ListFeedbacksByAssignment` | tutor or student | `feedbacks`, `submissions`, `assignments` |
| `GetAssignmentFile` | any authenticated (weak check) | `assignments` |
| `GetFeedbackFile` | tutor or student | three-table chain |

### DB schema
- `assignments` (id, tutor_id, student_id, title, description, file_id?, due_date?, created_at, edited_at)
- `submissions` (id, assignment_id FK CASCADE, file_id?, comment, created_at, edited_at)
- `feedbacks` (id, submission_id FK CASCADE, file_id?, comment, created_at, edited_at)

### Submission state machine
Not stored — purely computed via SQL CTE: `UNSENT` / `OVERDUE` / `UNREVIEWED` / `REVIEWED`. **No write-time enforcement** — students can submit even after `REVIEWED`.

### File flow
`file_id` stored as opaque UUID; **no existence check at write**. Only validated lazily when `Get*File` calls `file_service.GenerateDownloadURL` (wrapped in 3-attempt retry).

### Kafka
- Produces: `assignment-reminders` (`assignment_id`, `student_id`, `tutor_id`, `due_date`, `title`).
- Consumes: none.

### Background goroutines
- `ReminderWorker` ticker every 1 min querying `FindAssignmentsDueSoon(24h)` and producing to `assignment-reminders`. **Duplicates each assignment ~1,440 times** (no dedup).

---

## payment_service (gRPC :50051)

### RPCs

| RPC | Roles | DB tables |
|---|---|---|
| `GetPaymentInfo` | any auth | none (delegates to schedule_service) |
| `SubmitPaymentReceipt` | student | `receipts` (INSERT) + schedule.GetLesson + schedule.MarkAsPaid |
| `GetReceipt` | **none (bug)** | `receipts` |
| `VerifyReceipt` | any tutor (no ownership — bug) | `receipts` (UPDATE) |
| `GetReceiptFile` | **none (bug)** | `receipts` + file_service.GenerateDownloadURL |

### DB schema
- `receipts` (id UUID PK, lesson_id UUID NOT NULL, file_id UUID NOT NULL, is_verified BOOL DEFAULT false, created_at, edited_at)
- Indexes: `idx_receipts_lesson_id`, partial on `is_verified=false`.
- **No UNIQUE on `lesson_id`** → duplicate-receipt race.

### State machine
Boolean `is_verified` only. States: pending → verified. No rejected, no reason, no un-verify.

### Kafka / workers
None. Comment in code references planned notification event; not wired.

---

## file_service (gRPC :50051)

### RPCs

| RPC | Roles | DB tables |
|---|---|---|
| `InitUpload` | any auth | `files` INSERT, presigned PUT URL |
| `GenerateDownloadURL` | **none (bug)** | `files` SELECT, presigned GET URL |
| `GetFileMeta` | **none (bug)** | `files` SELECT |

### DB schema
- `files` (id UUID PK, extension VARCHAR(32), uploaded_by UUID, filename TEXT NULL, created_at). No `is_uploaded` flag, no soft-delete, no expiry.

### S3 layout
- Bucket from `S3_BUCKET_NAME`, created at startup.
- Key: `<uuid><ext>` (flat namespace).
- Presigned PUT/GET TTL = **5 minutes** each.
- No `ContentType` constraint on PUT; no size cap.

### Background goroutines / cleanup / Kafka
None. No orphan cleanup; no Kafka.

---

## notification_service (Kafka consumer only)

### Status
**Stub.** Connects to Kafka, consumes events from configured topics, JSON-unmarshals, logs the payload, commits offset, exits cleanly on SIGTERM. **No Telegram Bot API integration exists** — no token reader, no HTTP client, no formatting, no send call.

### Kafka topics consumed
- `lesson-reminders` (default)
- `assignment-reminders` (default)
- Any topics passed via `KAFKA_TOPICS` env (CSV)

### Per-topic dispatch
None — all topics handled identically (log).

### Workers / DB
1 main loop, single-threaded fetch→process→commit. No DB.

---

## common_library

### Packages
- `ctxdata` — typed-key getters/setters for `traceID`, `userID`, `userRole`.
- `logging` — zap wrapper with context-aware methods + `NewUnaryLoggingInterceptor`.
- `metadata` — `NewMetadataUnaryInterceptor` extracts metadata into context.
- `utils` — generic `RetryWithBackoff[T]`, `CircuitBreaker`, `RetryWithCircuitBreaker[T]`, `IsRetriable`.

### Exposed across services
Interceptors used by every gRPC server. Retry/CB used by `payment_service`, `schedule_service`, `homework_service` for upstream calls. **Not used by `api_gateway` for any of its outbound calls.**

### Workers
None (request-scoped only).

---

## Cross-Service Flows (Actual)

### Book a lesson
1. Client `POST /schedule/lessons` → `api_gateway/.../schedule.go` → raw gRPC `schedule_service.CreateLesson`.
2. `CreateLesson` validates slot ownership, calls `user_service.ResolveTutorStudentContext` via retry+CB.
3. Single tx: insert `lessons` row + `slots.is_booked=true`.
4. Emits `lesson-reminders` event (best-effort; no `RequireOne` on payment side, but schedule uses `RequireOne`).
5. `notification_service` reads, logs, commits. **No delivery.**

### Submit homework with attachment
1. `POST /files/init-upload` → `file_service.InitUpload` → `{file_id, signed PUT URL}`.
2. Client `PUT` directly to MinIO via the unauthenticated gateway proxy `/files/upload/*`.
3. Client `POST /homework/submissions {assignment_id, file_id, comment}` → `homework_service.CreateSubmission` → row stored without validating file_id.
4. Tutor later calls `GetSubmissionFile` → homework_service calls `file_service.GenerateDownloadURL` (no ACL check) → returns signed GET URL.

### Pay for a lesson
1. `POST /payment/receipts {lesson_id, file_id}` → `payment_service.SubmitPaymentReceipt`.
2. Calls `schedule.GetLesson` (via retry+CB) → inserts receipt → calls `schedule.MarkAsPaid` (no authz on schedule side — anyone can call this).
3. Tutor `POST /payment/receipts/{id}/verify` → flips `is_verified`. **No ownership check on the tutor side.**
4. Either party `GET /payment/receipts/{id}/file-url` → unauthorized read works.

### Telegram notification delivery
**Not implemented.** Events produced; consumer logs only.

---

## Background workers / cron summary

| Service | Worker | Status |
|---|---|---|
| api_gateway | HTTP serve loop | working |
| user_service | gRPC serve | working; `Stop()` not `GracefulStop()` |
| schedule_service | gRPC serve | working; **`UpdateCompletedLessons` never invoked** |
| homework_service | gRPC serve + `ReminderWorker` 1-min ticker | running but **duplicates events** |
| payment_service | gRPC serve | working |
| file_service | gRPC serve | working; no orphan cleanup |
| notification_service | single-threaded Kafka loop | stub; no delivery |

---

## What's NOT implemented (vs README / CLAUDE.md / OpenAPI)

1. **Lesson parameter precedence** (lesson > pair > tutor defaults) — entirely missing from `schedule_service`.
2. **Lesson auto-completion** — repo method exists, no caller.
3. **Telegram delivery in `notification_service`** — consumer is a stub.
4. **File upload confirmation / orphan cleanup** in `file_service`.
5. **File ACL** in `file_service` — any caller with a UUID downloads.
6. **Retry/CB at the API gateway** — every fan-out call is raw.
7. **Lesson cancellation event** to Kafka — no producer call in `CancelLesson`.
8. **Receipt rejection state** in `payment_service` — schema has bool only.
9. **CI coverage of `notification_service`** — no lint/test/build jobs.
10. **End-to-end auth chain hardening** — `x-user-id` is injectable on unauthenticated gateway routes.
