# Frontend API Guide

**Base URL:** `https://unfixable-apron-pointy.ngrok-free.dev`

## Headers

Every request must include:

```typescript
const HEADERS = {
  "Content-Type": "application/json",
  "ngrok-skip-browser-warning": "true",   // bypass ngrok interstitial
};
```

## Auth

### Mini App (correct way — no secret needed)

```typescript
const initData = window.Telegram.WebApp.initData;

fetch(`${API}/users/users/me`, {
  headers: {
    ...HEADERS,
    Authorization: `tma ${initData}`,
  },
});
```

### Dev only (HMAC — requires secret)

```bash
./scripts/generate-auth-token.sh <tg_id> <bot_token>
```

## JSON Convention

- **Request body**: `snake_case` — `telegram_id`, `tutor_id`, `starts_at`
- **Response body**: `camelCase` — `telegramId`, `tutorId`, `startsAt`

## API Reference

Prefixes from `main.go` route mounting:
- `/users` — auth, users, tutor-profiles, tutor-students
- `/files` — file upload/download
- `/schedule` — slots, lessons
- `/homework` — assignments, submissions, feedbacks
- `/payment` — receipts, analytics
- `/faqs` — public FAQ

### Auth & Users

| Method | Path | Auth | Body |
|--------|------|------|------|
| `POST` | `/users/sign-up/telegram` | No | `{telegram_id, role, first_name?, last_name?, username?}` |
| `GET` | `/users/users/me` | Yes | — |
| `GET` | `/users/users/{id}` | Yes | — |
| `PATCH` | `/users/users/{id}` | Yes | `{first_name?, last_name?, timezone?}` |

### Tutor Profiles

| Method | Path | Auth |
|--------|------|------|
| `GET` | `/users/tutor-profiles/{id}` | Yes |
| `PATCH` | `/users/tutor-profiles/{id}` | Yes |

### Tutor-Student Relationships

| Method | Path | Auth | Body |
|--------|------|------|------|
| `POST` | `/users/tutor-students` | Yes | `{tutor_id, student_id, lesson_price_rub?, lesson_connection_link?}` |
| `POST` | `/users/tutor-students/{tutor_id}/accept` | Yes (student) | — |
| `GET` | `/users/tutor-students/by-tutor/{id}` | Yes | — |
| `GET` | `/users/tutor-students/by-student/{id}` | Yes | — |
| `PATCH` | `/users/tutor-students/{tutor_id}/{student_id}` | Yes | `{status?, lesson_price_rub?, lesson_connection_link?}` |
| `DELETE` | `/users/tutor-students/{tutor_id}/{student_id}` | Yes | — |

### Schedule — Slots

| Method | Path | Auth | Body |
|--------|------|------|------|
| `POST` | `/schedule/slots` | Yes (tutor) | `{tutor_id, starts_at, ends_at}` |
| `GET` | `/schedule/slots/{id}` | Yes | — |
| `GET` | `/schedule/slots/by-tutor/{tutor_id}?only_available=true` | Yes | — |
| `PATCH` | `/schedule/slots/{id}` | Yes (tutor) | `{starts_at?, ends_at?}` |
| `DELETE` | `/schedule/slots/{id}` | Yes (tutor) | — |

### Schedule — Lessons

| Method | Path | Auth | Body |
|--------|------|------|------|
| `POST` | `/schedule/lessons` | Yes | `{slot_id, student_id}` |
| `GET` | `/schedule/lessons/{id}` | Yes | — |
| `GET` | `/schedule/lessons?tutor_id=X&from=ISO&to=ISO` | Yes | — |
| `PATCH` | `/schedule/lessons/{id}` | Yes (tutor) | `{price_rub?, connection_link?, payment_info?}` |
| `POST` | `/schedule/lessons/{id}/cancel` | Yes | — |
| `POST` | `/schedule/lessons/{id}/reschedule` | Yes | `{new_slot_id}` |

### Homework — Assignments

| Method | Path | Auth | Body |
|--------|------|------|------|
| `POST` | `/homework/assignments` | Yes (tutor) | `{tutor_id, student_id, title?, description?, file_id?, due_date?}` |
| `GET` | `/homework/assignments?tutor_id=X&page=1&page_size=20` | Yes | — |
| `PATCH` | `/homework/assignments/{id}` | Yes (tutor) | `{title?, description?, due_date?}` |
| `DELETE` | `/homework/assignments/{id}` | Yes (tutor) | — |
| `GET` | `/homework/assignments/{id}/file-url` | Yes | — |
| `GET` | `/homework/assignments/{id}/submissions` | Yes | — |
| `GET` | `/homework/assignments/{id}/feedbacks` | Yes | — |

### Homework — Submissions & Feedbacks

| Method | Path | Auth | Body |
|--------|------|------|------|
| `POST` | `/homework/submissions` | Yes (student) | `{assignment_id, file_id?, comment?}` |
| `GET` | `/homework/submissions/{id}/file-url` | Yes | — |
| `POST` | `/homework/feedbacks` | Yes (tutor) | `{submission_id, grade?, comment?, file_id?}` |
| `PATCH` | `/homework/feedbacks/{id}` | Yes (tutor) | `{grade?, comment?}` |
| `GET` | `/homework/feedbacks/{id}/file-url` | Yes | — |

### Files

| Method | Path | Auth | Body |
|--------|------|------|------|
| `POST` | `/files/init-upload` | Yes | `{uploaded_by, filename}` |
| `POST` | `/files/{id}/confirm-upload` | Yes | — |
| `GET` | `/files/{id}/meta` | Yes | — |

Returns `{fileId, uploadUrl, method: "PUT"}`. Upload bytes to `uploadUrl` with `PUT`, then call `confirm-upload`.

### Payments

| Method | Path | Auth | Body |
|--------|------|------|------|
| `POST` | `/payment/receipts` | Yes (student) | `{lesson_id, file_id}` |
| `GET` | `/payment/receipts?tutor_id=X` or `?student_id=X` | Yes | — |
| `GET` | `/payment/receipts/{id}` | Yes | — |
| `POST` | `/payment/receipts/{id}/verify` | Yes (tutor) | — |
| `GET` | `/payment/receipts/{id}/file-url` | Yes | — |
| `GET` | `/payment/info/{lesson_id}` | Yes | — |

### FAQ (public, no auth)

| Method | Path |
|--------|------|
| `GET` | `/faqs` |
| `GET` | `/faqs?category=general` |
| `GET` | `/faqs/categories` |
| `GET` | `/faqs/{id}` |

## TypeScript Quick Start

```typescript
const API = "https://unfixable-apron-pointy.ngrok-free.dev";

const api = {
  headers: () => ({
    "Content-Type": "application/json",
    "ngrok-skip-browser-warning": "true",
    Authorization: `tma ${window.Telegram.WebApp.initData}`,
  }),

  async me() {
    const r = await fetch(`${API}/users/users/me`, { headers: this.headers() });
    return r.json();
  },

  async signUp(telegramId: number, role: "tutor" | "student", firstName?: string) {
    const r = await fetch(`${API}/users/sign-up/telegram`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "ngrok-skip-browser-warning": "true" },
      body: JSON.stringify({ telegram_id: telegramId, role, first_name: firstName }),
    });
    return r.json();
  },

  async listSlots(tutorId: string, onlyAvailable = true) {
    const r = await fetch(
      `${API}/schedule/slots/by-tutor/${tutorId}?only_available=${onlyAvailable}`,
      { headers: this.headers() }
    );
    return r.json();
  },

  async createLesson(slotId: string, studentId: string) {
    const r = await fetch(`${API}/schedule/lessons`, {
      method: "POST",
      headers: this.headers(),
      body: JSON.stringify({ slot_id: slotId, student_id: studentId }),
    });
    return r.json();
  },
};

// Usage:
const user = await api.me();
console.log(user.id, user.role);
```

## Error Responses

| HTTP | Meaning |
|------|---------|
| 200 | OK |
| 204 | No Content (OPTIONS preflight) |
| 400 | Bad Request (invalid params) |
| 401 | Unauthorized (missing/invalid auth) |
| 403 | Forbidden (wrong role) |
| 404 | Not Found |
| 409 | Conflict (already exists) |
| 500 | Internal Server Error |
