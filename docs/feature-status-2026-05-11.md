# Feature Status 2026-05-11

| Area | Feature | Status | Notes |
| --- | --- | --- | --- |
| Notifications | Tutor receives lesson/homework notifications | Done | `notification_service` builds messages for both `student_id` and `tutor_id`. |
| Payments | List receipts by tutor/student | Done | gRPC `ListReceipts`, repository queries, and HTTP `GET /payment/receipts` exist. |
| Schedule | Date filters in `ListLessons` | Done | `from`/`to` are exposed in proto, gateway query parsing, service validation, and SQL. |
| Payments | Store receipt amount | Done | `receipts.price_rub` exists and is copied from the lesson on receipt submit. |
| Payments | Payment reminders | Done | `payment_service` worker sends `payment-reminders` for completed unpaid lessons and stores sent markers. |
| Files | Upload lifecycle | Done | `is_uploaded`, `ConfirmUpload`, HTTP confirm endpoint, and orphan cleanup worker are implemented. |
| Schedule | Reschedule lesson | Done | Dedicated `RescheduleLesson` flow cancels the old lesson, books a new slot, and stores the original lesson relation. |
| Product | Groups, recurring lessons, analytics, FAQ, grades, JWT, initData | Out of scope | These are larger domain changes and are not implemented in this pass. |
