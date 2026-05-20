package main

import (
	"fmt"
	"time"
)

// dateLayoutRU is the human-readable date/time layout used in all Telegram
// notifications: e.g. "12.05.2026 10:00 UTC". Times are formatted in UTC.
const dateLayoutRU = "02.01.2006 15:04 UTC"

// Lesson reminder templates. The single %s is an optional date clause
// (" на 12.05.2026 10:00 UTC" or "") produced by lessonWhenClause.
const (
	msgLessonBooked             = "Записан новый урок%s."
	msgLessonCancelledStudent   = "Ваш урок%s отменён."
	msgLessonCancelledTutor     = "Урок%s отменён учеником."
	msgLessonRescheduledStudent = "Ваш урок перенесён%s."
	msgLessonRescheduledTutor   = "Урок перенесён%s."
	msgLessonReminder           = "Напоминание об уроке%s."
)

// Assignment reminder building blocks.
const (
	msgAssignmentStudent = "Напоминание о задании"
	msgAssignmentTutor   = "Задание не сдано"
	assignmentTitleSep   = ": "
	assignmentDuePrefix  = " — срок до "
)

// Payment reminder templates. The single %s is an optional price clause
// (" на 500 ₽" or "") produced by paymentPriceClause.
const (
	msgPaymentStudent = "Напоминание об оплате%s."
	msgPaymentTutor   = "Оплата всё ещё не поступила%s."
	paymentPriceFmt   = " на %.0f ₽"
)

// lessonWhenClause returns " на <date>" for a non-zero time, or "" otherwise.
// The leading space and preposition let it slot into the lesson templates.
func lessonWhenClause(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return " на " + t.UTC().Format(dateLayoutRU)
}

// paymentPriceClause returns " на <price> ₽" for a positive price, or "".
func paymentPriceClause(priceRub float64) string {
	if priceRub <= 0 {
		return ""
	}
	return fmt.Sprintf(paymentPriceFmt, priceRub)
}
