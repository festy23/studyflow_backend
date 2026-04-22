package e2e

import (
	"net/http"
	"testing"
	"time"
)

func TestCreateSlotAndBookLesson(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	now := time.Now().UTC()
	startsAt := now.Add(24 * time.Hour).Truncate(time.Hour).Add(14 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	slotID := slot["id"].(string)

	lesson := bookLesson(t, studentAuth, slotID, studentID)
	if lesson["id"] == nil || lesson["id"] == "" {
		t.Fatal("lesson creation returned no id")
	}
	if lesson["status"] != "booked" {
		t.Errorf("expected status=booked, got %v", lesson["status"])
	}
}

func TestUpdateLessonDetails(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	now := time.Now().UTC()
	startsAt := now.Add(25 * time.Hour).Truncate(time.Hour).Add(10 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	lesson := bookLesson(t, tutorAuth, slot["id"].(string), studentID)
	lessonID := lesson["id"].(string)

	resp := doRequest(t, "PATCH", "/schedule/lessons/"+lessonID, map[string]any{
		"priceRub":        2000,
		"connectionLink":  "https://zoom.us/j/123456",
		"paymentInfo":     "Sberbank 1234 5678 9012",
	}, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusOK)
	updated := readJSON(t, resp)

	if updated["priceRub"] != float64(2000) {
		t.Errorf("expected priceRub=2000, got %v", updated["priceRub"])
	}
	if updated["connectionLink"] != "https://zoom.us/j/123456" {
		t.Errorf("unexpected connectionLink: %v", updated["connectionLink"])
	}
}

func TestCancelLesson(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	now := time.Now().UTC()
	startsAt := now.Add(26 * time.Hour).Truncate(time.Hour).Add(15 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	lesson := bookLesson(t, studentAuth, slot["id"].(string), studentID)
	lessonID := lesson["id"].(string)

	resp := doRequest(t, "POST", "/schedule/lessons/"+lessonID+"/cancel", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	cancelled := readJSON(t, resp)
	if cancelled["status"] != "cancelled" {
		t.Errorf("expected status=cancelled, got %v", cancelled["status"])
	}
}

func TestListLessons(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	now := time.Now().UTC()
	from := now.Add(20 * time.Hour).Truncate(time.Hour)
	startsAt := from.Add(14 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	bookLesson(t, studentAuth, slot["id"].(string), studentID)

	path := "/schedule/lessons?tutor_id=" + tutorID + "&from=" + from.Format(time.RFC3339) + "&to=" + from.Add(48*time.Hour).Format(time.RFC3339)
	resp := doRequest(t, "GET", path, nil, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusOK)
	result := readJSON(t, resp)

	lessons, ok := result["lessons"].([]any)
	if !ok || len(lessons) == 0 {
		t.Fatal("expected non-empty lessons list")
	}
}

func TestRescheduleLesson(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	now := time.Now().UTC()
	base := now.Add(30 * time.Hour).Truncate(time.Hour)

	slot1 := createSlot(t, tutorAuth, tutorID, base.Add(10*time.Hour), base.Add(11*time.Hour))
	slot2 := createSlot(t, tutorAuth, tutorID, base.Add(14*time.Hour), base.Add(15*time.Hour))

	lesson := bookLesson(t, studentAuth, slot1["id"].(string), studentID)
	lessonID := lesson["id"].(string)

	resp := doRequest(t, "POST", "/schedule/lessons/"+lessonID+"/reschedule", map[string]any{
		"newSlotId": slot2["id"].(string),
	}, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusOK)
	newLesson := readJSON(t, resp)

	if newLesson["status"] != "booked" {
		t.Errorf("expected status=booked after reschedule, got %v", newLesson["status"])
	}
	if newLesson["rescheduledFromLessonId"] != lessonID {
		t.Errorf("expected rescheduledFromLessonId=%s, got %v", lessonID, newLesson["rescheduledFromLessonId"])
	}
}
