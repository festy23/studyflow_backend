# StudyFlow Backend — Full Audit Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce two deliverables for the StudyFlow backend monorepo: (1) a categorized bug & issue list with file:line evidence, and (2) a complete catalog of the service's real, implemented functionality (per-microservice + cross-cutting).

**Architecture:** This is an *audit*, not an implementation. Work is parallelized across the 7 top-level modules (`api_gateway`, `user_service`, `schedule_service`, `homework_service`, `payment_service`, `file_service`, `notification_service`, `common_library`). Each module is audited by an independent `Explore`/`feature-dev:code-reviewer` subagent against a fixed checklist. A final aggregation task merges findings into two markdown reports under `docs/superpowers/audit/`.

**Tech Stack:** Go 1.24, gRPC, Chi v5, PostgreSQL, Redis, MinIO/S3, Kafka, zap, golangci-lint v2, go.uber.org/mock.

**Deliverables (DoD):**
- `docs/superpowers/audit/2026-05-11-bugs.md` — every finding with severity (Critical/High/Medium/Low), file:line, description, suggested fix.
- `docs/superpowers/audit/2026-05-11-functionality.md` — actual implemented features per service: endpoints, gRPC methods, DB tables, Kafka topics, background jobs, auth/role rules, integrations.

---

## Audit Checklist (applied per module)

Each per-module agent must produce findings against this checklist:

1. **Correctness bugs** — nil deref, off-by-one, wrong error mapping, missed `defer Close`, ignored errors (`errcheck`), goroutine leaks, context misuse.
2. **Concurrency** — data races, missing `sync` primitives, unbounded goroutines, leaking channels.
3. **gRPC / HTTP contract** — status code misuse, missing validation, unsafe `Errorf(code, err.Error())` (must be `%v`), deprecated `grpc.Dial`.
4. **Auth & authorization** — trust of `x-user-id`/`x-user-role` metadata, role checks per endpoint, IDOR (does the handler verify the resource belongs to the caller?).
5. **DB layer** — N+1, missing indexes referenced in queries, SQL injection risk (string concat), transaction boundaries, migrations sanity, `sql.ErrNoRows` handling.
6. **Resilience** — retry policy (only `codes.Unavailable`), circuit breaker presence on inter-service calls, timeouts on outbound calls.
7. **Resource hygiene** — `defer rows.Close()`, `defer resp.Body.Close()`, S3 client lifecycle, Redis pipelines closed.
8. **Logging / PII** — secrets in logs, PII (phone, email, telegram_id) in logs, request-id propagation.
9. **Kafka** — producer error handling, idempotency, consumer commit semantics (at-least-once vs at-most-once), poison-message handling.
10. **Config & secrets** — hardcoded values, missing env validation, secrets in code/compose.
11. **Tests** — coverage ≥30% per package (CI gate), interface-based mocks via `go.uber.org/mock`, table-driven tests, no real network in unit tests.
12. **Idiomatic Go** — sentinel error naming (`ErrFoo`), typed context keys, format string hygiene, modern Go (per `use-modern-go`).

---

## Task 1: Repo-wide reconnaissance

**Files:**
- Create: `docs/superpowers/audit/_recon.md` (scratch notes; will be deleted at end)

- [ ] **Step 1: Inventory the repo**

Run:
```bash
cd /Users/ivanm3/studyflow_backend
find . -maxdepth 3 -type f -name "*.go" | wc -l
find . -maxdepth 3 -type f -name "*.proto"
find . -maxdepth 4 -type d -name "migrations"
find . -maxdepth 4 -type f -name "main.go"
git log --oneline -30
```

Record: per-service Go file count, proto files, migration directories, recent commit themes.

- [ ] **Step 2: Read top-level config**

Read in full:
- `docker-compose.yml`
- `.gitlab-ci.yml`
- `.golangci.yml`
- `README.md`
- `developer_readme.md`
- `api_gateway/OpenAPI.yml`

Record: declared services, exposed ports, env vars, CI gates, lint config exclusions.

- [ ] **Step 3: Build the per-service inventory table**

Write to `docs/superpowers/audit/_recon.md`:

| Service | Lines of Go | Proto file | Migrations | Has tests? | Notes |

This table seeds the next tasks.

- [ ] **Step 4: Commit recon notes**

```bash
git add docs/superpowers/audit/_recon.md
git commit -m "audit: repo reconnaissance notes"
```

---

## Task 2: Parallel per-module audits

Dispatch **8 subagents in a single message** (one tool call block with 8 `Agent` invocations). Each uses `subagent_type: "feature-dev:code-reviewer"` and operates **read-only**.

**Module → agent assignment:**
1. `common_library/`
2. `api_gateway/`
3. `user_service/`
4. `schedule_service/`
5. `homework_service/`
6. `payment_service/`
7. `file_service/`
8. `notification_service/`

**Prompt template for each agent** (substitute `<MODULE>`):

> Audit the `<MODULE>` directory of the StudyFlow Go backend at `/Users/ivanm3/studyflow_backend/<MODULE>`. You are read-only — do NOT modify code. Apply the 12-point checklist below to every Go file in the module. For every finding, output a row:
>
> `Severity | file:line | Category | Description | Suggested fix`
>
> Severities: Critical (data loss / auth bypass / panics in prod path), High (incorrect behavior, resource leak, missing authz), Medium (poor error handling, missing tests, fragile), Low (style, naming, minor idiomatic issues).
>
> Checklist: [paste the 12-point list from the plan].
>
> Also produce a **Functionality Inventory** section listing: (a) every gRPC method or HTTP route exposed, (b) every DB table touched, (c) every Kafka topic produced/consumed, (d) every external integration (S3, Redis, Telegram, other services), (e) every background goroutine / cron, (f) the role-based access rules enforced.
>
> Report in under 1500 words. Use file:line citations for every finding — no vague claims.

- [ ] **Step 1: Dispatch all 8 agents in parallel**

Single message with 8 `Agent` tool calls.

- [ ] **Step 2: Save each agent's report**

Write each report to `docs/superpowers/audit/_module-<name>.md` verbatim.

- [ ] **Step 3: Commit module reports**

```bash
git add docs/superpowers/audit/_module-*.md
git commit -m "audit: per-module review reports"
```

---

## Task 3: Cross-cutting audit

Single `feature-dev:code-reviewer` agent.

**Prompt:**

> Audit cross-cutting concerns in `/Users/ivanm3/studyflow_backend` that span multiple services. Read-only. Focus areas:
>
> 1. **Auth chain integrity** — trace a request from Telegram HMAC validation in `api_gateway` → gRPC metadata → downstream services. Can a downstream service be called directly bypassing the gateway? Are `x-user-id` / `x-user-role` ever overwritable by the caller?
> 2. **Inter-service contracts** — for every gRPC client used (grep `pb.New*Client`), confirm timeout + retry + circuit breaker are wired.
> 3. **Two-step file upload flow** — trace `file_service.InitUpload` → client S3 PUT → `file_id` consumed by homework/payment. Find races, missing existence checks, orphaned uploads.
> 4. **Lesson parameter resolution** — verify the documented precedence (lesson-specific > tutor-student pair > tutor defaults) in `schedule_service`.
> 5. **Kafka topology** — list topics, producers, consumers; check for unhandled producer errors and at-least-once consumer correctness.
> 6. **docker-compose vs code** — env vars declared but unused, used but undeclared, ports/health checks missing.
> 7. **CI gates** — does the 30% coverage gate actually run for every module? Any module excluded?
>
> Output: findings table (same format as Task 2) + a "Cross-cutting functionality" section describing the end-to-end flows actually implemented.

- [ ] **Step 1: Dispatch the cross-cutting agent**
- [ ] **Step 2: Save report to `docs/superpowers/audit/_cross-cutting.md`**
- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/audit/_cross-cutting.md
git commit -m "audit: cross-cutting review report"
```

---

## Task 4: Aggregate bugs report

**Files:**
- Create: `docs/superpowers/audit/2026-05-11-bugs.md`

- [ ] **Step 1: Merge findings**

Read all `_module-*.md` and `_cross-cutting.md`. Deduplicate. Group by severity, then by module.

- [ ] **Step 2: Write the report**

Structure:

```markdown
# StudyFlow Backend — Bug & Issue Report (2026-05-11)

## Summary
- Critical: N | High: N | Medium: N | Low: N

## Critical
### [BUG-001] <title>
- **Module:** <module>
- **Location:** `path/file.go:LINE`
- **Category:** <one of 12 checklist categories>
- **Description:** <what's wrong, why it matters>
- **Suggested fix:** <concrete change>

## High
...

## Medium
...

## Low
...

## Cross-cutting Issues
...
```

Every finding must have file:line. Drop any without.

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/audit/2026-05-11-bugs.md
git commit -m "audit: aggregated bug report"
```

---

## Task 5: Aggregate functionality report

**Files:**
- Create: `docs/superpowers/audit/2026-05-11-functionality.md`

- [ ] **Step 1: Merge functionality inventories**

From each module report, extract the Functionality Inventory section.

- [ ] **Step 2: Write the report**

Structure:

```markdown
# StudyFlow Backend — Implemented Functionality (2026-05-11)

## System Overview
<one paragraph: what the system actually does today, derived from code not docs>

## API Gateway (REST)
### Endpoints
| Method | Path | Auth | Handler | Downstream gRPC | Notes |

## user_service (gRPC :50051)
### gRPC methods
| RPC | Request → Response | Roles allowed | DB tables | Notes |
### DB schema (from migrations)
### Kafka
- Produces: <topics>
- Consumes: <topics>

## schedule_service
<same shape>

## homework_service
<same shape>

## payment_service
<same shape>

## file_service
<same shape>
### S3 / MinIO flow
<two-step upload description as actually implemented>

## notification_service
<same shape>

## Cross-service flows
### Book a lesson
<step-by-step trace through code>
### Submit homework with attachments
<...>
### Pay for a lesson
<...>
### Telegram notification delivery
<...>

## Background workers / cron
<list>

## What's NOT implemented
<gaps spotted vs OpenAPI.yml or README claims>
```

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/audit/2026-05-11-functionality.md
git commit -m "audit: functionality inventory"
```

---

## Task 6: Cleanup & handoff

- [ ] **Step 1: Remove scratch files**

```bash
rm docs/superpowers/audit/_recon.md docs/superpowers/audit/_module-*.md docs/superpowers/audit/_cross-cutting.md
git add -A
git commit -m "audit: remove scratch notes"
```

- [ ] **Step 2: Print summary to user**

Report to the user: total bug counts by severity, link to both deliverable files, top 3 Critical findings inline.

---

## Self-review notes

- Spec coverage: bugs ✅ (Tasks 2–4), functionality list ✅ (Tasks 2,3,5), parallel agents ✅ (Task 2), DoD met by Tasks 4 + 5.
- No code is modified — this is an audit. No tests required.
- Every agent prompt is self-contained and gives file:line evidence requirements.
