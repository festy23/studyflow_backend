# Руководство по изменению ТЗ под текущий бекенд StudyFlow

**Дата:** 2026-05-11
**Принцип:** Меняем ТЗ под бекенд, а не наоборот. Бекенд трогаем только если правка тривиальная (добавить поле, поправить валидацию).

---

## ЧАСТЬ 1. ЧТО УБРАТЬ ИЗ ТЗ (не реализовано, не планируется)

Эти пункты есть в ТЗ но отсутствуют в бекенде. Добавлять их — недели работы. Убираем из ТЗ.

| # | Пункт ТЗ | Секция ТЗ | Почему убираем |
|---|---|---|---|
| 1 | **Групповые занятия** (вместимость, бизнес-правила записи) | 4.1.10 | В коде только индивидуальные: `lessons.student_id` — одно поле, уникальный constraint на slot. Переделка всей схемы. |
| 2 | **Рекуррентные/повторяющиеся занятия** | 4.1.1 п.3 | Нет RRULE, нет recurring-логики, нет exclusion dates. Добавление требует новой схемы + сложной бизнес-логики. |
| 3 | **Календарная интеграция** (iCal/Google Calendar) | 4.1.10 | Нет iCal-экспорта, нет Google Calendar API. Неделя разработки. |
| 4 | **Роль «родитель» (parent)** | 4.1.10 | Только tutor/student. Роль parent требует новой логики видимости, связей родитель-ученик, приглашений. |
| 5 | **Onboarding / приветственные шаблоны бота** | 4.1.10 | Нет onboarding flow, нет шаблонов. Не реализовано и не нужно для MVP. |
| 6 | **Обработка входящих команд бота** (/start, /help и др.) | 4.1.7 | Бот только отправляет уведомления. Webhook/команды не реализованы. |
| 7 | **Поиск по FAQ** (full-text) | 4.1.10 | Только фильтр по категориям. full-text search — это pg_trgm или tsvector, не тривиально. |
| 8 | **Rate limiting** | Приложение 1 | Нет rate limiting ни на API Gateway, ни в сервисах. |
| 9 | **Системные отчёты** (административные, CSV/PDF) | 4.1.10 | Только `GetTutorAnalytics` для одного тьютора. Мульти-тьюторских/админских отчётов нет. |

---

## ЧАСТЬ 2. ЧТО ЗАМЕНИТЬ В ТЗ (другое название/подход в бекенде)

Эти вещи реализованы, но называются или работают иначе. Меняем формулировки в ТЗ.

| # | Было в ТЗ | Стало в ТЗ | Где в коде |
|---|---|---|---|
| 1 | **Gin** (web framework) | **Chi v5** (`go-chi/chi/v5`) | `api_gateway/go.mod` |
| 2 | **GORM** (ORM) | **Прямой SQL через pgx/v5 + scany/v2** | Все 7 сервисов, см. `user_service/internal/db/db.go` |
| 3 | **Redis + asynq** для фоновых задач | **Redis** (кэш API Gateway) + **Apache Kafka** (очереди сообщений: lesson-reminders, assignment-reminders, payment-reminders) | `docker-compose.yml`, `notification_service/cmd/server/main.go` |
| 4 | **PostgreSQL 15+** | **PostgreSQL 16** | `docker-compose.yml` (faq-db, audit-db) |
| 5 | **Go 1.21+** | **Go 1.24** | Все `go.mod` файлы |
| 6 | Статусы занятий: **planned / conducted / cancelled / rescheduled** | **booked / completed / cancelled** (+ rescheduled через `rescheduled_from_lesson_id`) | `schedule_service/migrations/000001_init_schema.up.sql:21` |
| 7 | Статусы учеников: **studying / finished / trial** | **invited / active** (+ будущие blocked / removed) | `user_service/migrations/0005`, `user_service/internal/model/models.go` |
| 8 | Пагинация: **limit / offset** | **page / page_size** | Все list-эндпоинты (schedule, homework, payment, faq) |
| 9 | Денежные суммы: **десятичный формат (2 знака)** | **Целые числа (рубли, int32)** | `payment_service/proto/payment_service.proto` — `price_rub` |
| 10 | Telegram Bot API **v5 или telebot.v3** | **Прямые HTTP-запросы к `api.telegram.org/bot<token>/sendMessage`** | `notification_service/cmd/server/telegram.go` |
| 11 | **6 микросервисов** | **8 микросервисов** (+ notification_service, + audit_service, + faq_service) | `docker-compose.yml` |

---

## ЧАСТЬ 3. ЧТО ОСТАВИТЬ В ТЗ БЕЗ ИЗМЕНЕНИЙ (полностью реализовано)

| # | Пункт ТЗ | Статус |
|---|---|---|
| 1 | **REST API с JSON** | ✅ 52 эндпоинта |
| 2 | **Аутентификация через Telegram initData (HMAC-SHA256)** | ✅ `user_service/internal/authorization/telegram.go` |
| 3 | **Роли: тьютор, студент** | ✅ разграничение прав на всех операциях |
| 4 | **CRUD пользователей** | ✅ RegisterViaTelegram, GetMe, GetUser, UpdateUser |
| 5 | **CRUD учеников у тьютора** (tutor_students) | ✅ Create, Get, Update, Delete, List, Accept |
| 6 | **CRUD слотов расписания** | ✅ CreateSlot, UpdateSlot, DeleteSlot, ListSlotsByTutor |
| 7 | **CRUD занятий** | ✅ CreateLesson, UpdateLesson, GetLesson, ListLessons* |
| 8 | **Отмена и перенос занятий** | ✅ CancelLesson, RescheduleLesson с сохранением истории |
| 9 | **Авто-завершение занятий** (completed) | ✅ Background worker (5 мин интервал) |
| 10 | **CRUD домашних заданий** | ✅ assignments, submissions, feedbacks |
| 11 | **5-балльная система оценок** | ✅ `CHECK (grade BETWEEN 1 AND 5)` |
| 12 | **Статусы ДЗ** (UNSENT/UNREVIEWED/REVIEWED/OVERDUE) | ✅ CTE в SQL |
| 13 | **Файлы через S3/MinIO** (двухшаговая загрузка) | ✅ InitUpload → presigned PUT → ConfirmUpload |
| 14 | **Учёт оплат** (сумма, дата, статус is_verified) | ✅ receipts таблица |
| 15 | **Аналитика по тьютору** | ✅ GetTutorAnalytics (revenue, completed, cancelled, active_students) |
| 16 | **Напоминания об оплате** | ✅ PaymentReminderWorker → Kafka → Telegram |
| 17 | **FAQ: CRUD + категории** | ✅ 6 эндпоинтов, ListCategories |
| 18 | **FAQ: кэширование (Redis)** | ✅ API Gateway кэш (5 мин TTL) |
| 19 | **Аудит ключевых операций** | ✅ audit_service + middleware на API Gateway |
| 20 | **Docker Compose** (все сервисы контейнеризированы) | ✅ 17 сервисов |
| 21 | **gRPC межсервисное взаимодействие** | ✅ Все 8 сервисов на порту 50051 |
| 22 | **Graceful shutdown** | ✅ signal.NotifyContext + GracefulStop |
| 23 | **Retry + экспоненциальная задержка + Circuit Breaker** | ✅ `common_library/utils/retry.go` |
| 24 | **Логирование (zap, request_id UUIDv7)** | ✅ `common_library/logging/` |
| 25 | **Миграции БД** (golang-migrate) | ✅ Все сервисы |
| 26 | **OpenAPI/Swagger документация** | ✅ `api_gateway/OpenAPI.yml` (2058 строк) |
| 27 | **Фильтры по дате/статусу в расписании** | ✅ from/to + status_filter |
| 28 | **Дедупликация напоминаний** | ✅ payment_reminders table |
| 29 | **Валидация входных данных** | ✅ extension whitelist, grade range, required fields |
| 30 | **UUIDv7 для идентификаторов** | ✅ Все сервисы |

---

## ЧАСТЬ 4. ЧТО ДОБАВИТЬ В БЕКЕНД (лёгкие правки)

Эти вещи можно добавить в бекенд за несколько часов. Проще поправить код, чем менять ТЗ.

| # | Что сделать | Сложность | Файлы |
|---|---|---|---|
| 1 | **Зафиксировать PostgreSQL на `postgres:16-alpine`** в docker-compose.yml | 5 мин | `docker-compose.yml` — заменить 5 строк `postgres:latest` |
| 2 | **Добавить `user_id` в поля лога** | 10 мин | `common_library/logging/logger.go` — `fieldsWithTraceID()` добавить `ctxdata.GetUserID(ctx)` |
| 3 | **Добавить auth-middleware на FAQ write-эндпоинты** | 15 мин | `api_gateway/internal/handler/faq.go` — `r.Group()` с auth |
| 4 | **Отправлять Kafka-событие при подтверждении оплаты** (уведомление тьютора) | 30 мин | `payment_service/internal/service/service.go:156` — добавить вызов `eventSender.Send()` |
| 5 | **Исправить proto/Go расхождение: статусы `blocked`/`removed` для TutorStudent** | 30 мин | `user_service/internal/model/models.go` — добавить `blocked`, `removed` в `IsValid()` |
| 6 | **Retry с backoff для Telegram API** | 1 час | `notification_service/cmd/server/telegram.go` — обернуть `SendMessage` в `RetryWithBackoff` |
| 7 | **Добавить `DeleteUser` RPC** (деактивация — установка status='deleted') | 1 час | `user_service/proto`, service, handler |
| 8 | **Добавить `DeleteFile` RPC** в file_service | 1 час | `file_service/proto`, service — удаление из S3 + БД |

**Итого:** ~4-5 часов лёгких правок закрывают почти все пробелы.

---

## ЧАСТЬ 5. ЧТО ДОБАВИТЬ В ТЗ (новые разделы)

Эти вещи есть в бекенде, но отсутствуют в ТЗ. Добавить как новые пункты.

| # | Что добавить в ТЗ | Где в бекенде |
|---|---|---|
| 1 | **Apache Kafka** как брокер сообщений для асинхронных уведомлений | `docker-compose.yml` (сервис `kafka`), все продюсеры, `notification_service` consumer |
| 2 | **Notification Service** — микросервис отправки уведомлений через Telegram Bot API | `notification_service/` |
| 3 | **FAQ Service** — микросервис хранения и выдачи часто задаваемых вопросов | `faq_service/` |
| 4 | **Audit Service** — микросервис журналирования ключевых операций | `audit_service/` |
| 5 | **Двухшаговая загрузка файлов** (InitUpload → presigned URL → ConfirmUpload) | `file_service/internal/service/service.go` |
| 6 | **Автоматическое завершение уроков** (background worker) | `schedule_service/internal/worker/completion.go` |
| 7 | **Динамические статусы домашних заданий** (CTE, не хранятся в БД) | `homework_service/internal/repository/assignment.go` |
| 8 | **Приоритет параметров урока** (lesson > pair > tutor defaults) | `schedule_service/internal/service/service/utils.go` |
| 9 | **Circuit Breaker** для межсервисных вызовов | `common_library/utils/retry.go` |
| 10 | **UUIDv7** для первичных ключей | Все сервисы |
| 11 | **nginx** как reverse proxy на порту 80 | `nginx/default.conf` |

---

## ЧАСТЬ 6. ИТОГОВЫЙ ЧЕК-ЛИСТ ДЛЯ ТЗ

### 6.1. Секция «Программные средства» (4.4.2) — новая редакция:

```
- Система контейнеризации: Docker + Docker Compose
- Язык программирования: Go 1.24
- Web-фреймворк: Chi v5 (github.com/go-chi/chi/v5)
- СУБД: PostgreSQL 16
- Доступ к БД: pgx/v5 + прямой SQL
- Хранилище файлов: MinIO (S3-совместимое)
- Брокер сообщений: Apache Kafka (segmentio/kafka-go)
- Кэширование: Redis (go-redis/v9)
- Межсервисное взаимодействие: gRPC (google.golang.org/grpc)
- Миграции БД: golang-migrate/migrate/v4
- Логирование: zap (go.uber.org/zap)
- Телеграм-уведомления: прямые HTTP-запросы к Telegram Bot API
```

### 6.2. Секция «Функциональные модули» (4.1.10) — новая редакция:

Оставить:
- Расписание: создание/редактирование/удаление занятий и слотов, статусы, напоминания, фильтры
- Напоминания (занятия, оплата, ДЗ) через Kafka + Telegram
- Отмена и перенос занятий
- Домашние задания: хранение, файлы (S3/MinIO), статусы, оценки (1-5), комментарии
- Оплаты: учёт, напоминания, базовая статистика (на одного тьютора)
- FAQ: CRUD, категории, кэширование (Redis)
- Аналитика: агрегаты по продажам, клиентам, активности (один тьютор)
- Статусы учеников: invited / active
- Категории пользователей: tutor / student, ограничения видимости
- Аудит ключевых операций (middleware + audit_service)

Убрать:
- ~~Инструкция по запуску бота / onboarding~~
- ~~Групповые занятия~~
- ~~Рекуррентные занятия~~
- ~~Календарная интеграция~~
- ~~Роль «родитель»~~
- ~~Поиск по FAQ~~
- ~~Сбор фидбека (пунктуальность, качество ДЗ, активность, поведение)~~

Заменить:
- «planned/conducted/cancelled/rescheduled» → «booked/completed/cancelled (+ rescheduled)»
- «studying/finished/trial» → «invited/active»

### 6.3. Секция «API сервер» (4.1.1) — добавить подпункты:

Новые функции, которые есть в коде но не в ТЗ:
- 7. Аудит действий: журналирование всех мутаций (создание/изменение/удаление) с привязкой к пользователю
- 8. FAQ: CRUD часто задаваемых вопросов с категоризацией и кэшированием
- 9. Уведомления: отправка через Telegram Bot API по событиям Kafka

### 6.4. Секция «Безопасность» (4.1.9) — уточнить:

- Вместо «JWT или аналогичный механизм» указать: «Валидация Telegram initData при каждом запросе через HMAC-SHA256 с проверкой подписи. Идентификатор и роль пользователя передаются между сервисами через gRPC metadata (x-user-id, x-user-role).»

### 6.5. Секция «Специальные требования» (4.8) — добавить:

- Автоматическое завершение проведённых занятий (background worker, каждые 5 минут)
- Очистка неподтверждённых загрузок файлов (orphan cleanup, раз в час)
- Дедупликация платёжных напоминаний через БД
- UUIDv7 для всех первичных ключей

---

## ЧАСТЬ 7. ЧТО НЕ ТРОГАТЬ В ТЗ (остаётся как есть)

- Разделы 1-3 (Введение, Основания, Назначение)
- Раздел 4.1.2 (Входные данные API) — JSON + multipart/form-data
- Раздел 4.1.3 (Выходные данные API) — JSON + HTTP-коды + Telegram сообщения
- Раздел 4.1.4 (Временные характеристики) — 500ms / 3-5s
- Раздел 4.1.5 (Интерфейс API) — REST, HTTPS, OpenAPI
- Раздел 4.1.6 (База данных) — реляционная, ссылочная целостность, индексы
- Раздел 4.1.7 (Интеграция с Telegram) — валидация initData, Telegram ID, Bot API для отправки
- Раздел 4.1.11 (Обработка ошибок и логирование)
- Раздел 4.1.12 (Организация данных) — JSON, пагинация (page/page_size уточнить), ISO 8601
- Раздел 4.1.13 (Временные характеристики)
- Раздел 4.1.14 (Интерфейс взаимодействия)
- Раздел 4.2 (Надёжность) — добавить упоминание Kafka как гарантию доставки уведомлений
- Разделы 4.3-4.7 (Эксплуатация, тех. средства, совместимость, маркировка, транспортировка)
- Раздел 5 (Программная документация)
- Раздел 6 (Технико-экономические показатели)
- Раздел 7 (Стадии и этапы)
- Раздел 8 (Контроль и приёмка)
