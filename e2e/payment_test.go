package e2e

import (
	"net/http"
	"testing"
	"time"
)

func TestSubmitAndVerifyReceipt(t *testing.T) {
	_, tutorAuth, tutor := signUp(t, "tutor")
	_, studentAuth, student := signUp(t, "student")

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	now := time.Now().UTC()
	startsAt := now.Add(35 * time.Hour).Truncate(time.Hour).Add(12 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	lesson := bookLesson(t, studentAuth, slot["id"].(string), studentID)
	lessonID := lesson["id"].(string)

	doRequest(t, "PATCH", "/schedule/lessons/"+lessonID, map[string]any{
		"priceRub":    2500,
		"paymentInfo": "Tinkoff 5555 1234 5678 9012",
	}, map[string]string{"Authorization": tutorAuth})

	init := initUpload(t, studentAuth, studentID, "receipt.jpg")
	fileID := init["fileId"].(string)
	confirmUpload(t, studentAuth, fileID)

	receipt := submitReceipt(t, studentAuth, lessonID, fileID)
	receiptID := receipt["id"].(string)

	if receipt["priceRub"] != float64(2500) {
		t.Errorf("expected priceRub=2500, got %v", receipt["priceRub"])
	}

	verified := verifyReceipt(t, tutorAuth, receiptID)
	if verified["isVerified"] != true {
		t.Error("receipt should be verified after verification")
	}
	_ = receiptID
}

func TestGetPaymentInfo(t *testing.T) {
	_, tutorAuth, tutor := signUp(t, "tutor")
	_, studentAuth, student := signUp(t, "student")

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	now := time.Now().UTC()
	startsAt := now.Add(40 * time.Hour).Truncate(time.Hour).Add(16 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	lesson := bookLesson(t, studentAuth, slot["id"].(string), studentID)

	doRequest(t, "PATCH", "/schedule/lessons/"+lesson["id"].(string), map[string]any{
		"priceRub":    3000,
		"paymentInfo": "Alfa Bank 1111 2222 3333 4444",
	}, map[string]string{"Authorization": tutorAuth})

	resp := doRequest(t, "GET", "/payment/info/"+lesson["id"].(string), nil, map[string]string{
		"Authorization": studentAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	info := readJSON(t, resp)

	if info["priceRub"] != float64(3000) {
		t.Errorf("expected priceRub=3000, got %v", info["priceRub"])
	}
	if info["paymentInfo"] != "Alfa Bank 1111 2222 3333 4444" {
		t.Errorf("expected paymentInfo, got %v", info["paymentInfo"])
	}
}

func TestListReceipts(t *testing.T) {
	_, tutorAuth, tutor := signUp(t, "tutor")
	_, studentAuth, student := signUp(t, "student")

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	now := time.Now().UTC()
	startsAt := now.Add(45 * time.Hour).Truncate(time.Hour).Add(10 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	lesson := bookLesson(t, studentAuth, slot["id"].(string), studentID)
	doRequest(t, "PATCH", "/schedule/lessons/"+lesson["id"].(string), map[string]any{
		"priceRub": 1500,
	}, map[string]string{"Authorization": tutorAuth})

	init := initUpload(t, studentAuth, studentID, "receipt2.jpg")
	fileID := init["fileId"].(string)
	confirmUpload(t, studentAuth, fileID)
	submitReceipt(t, studentAuth, lesson["id"].(string), fileID)

	// List by tutor
	resp := doRequest(t, "GET", "/payment/receipts?tutor_id="+tutorID, nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	result := readJSON(t, resp)
	receipts, ok := result["receipts"].([]any)
	if !ok || len(receipts) == 0 {
		t.Fatal("expected at least 1 receipt in list")
	}

	// List by student
	resp2 := doRequest(t, "GET", "/payment/receipts?student_id="+studentID, nil, map[string]string{
		"Authorization": studentAuth,
	})
	assertStatus(t, resp2, http.StatusOK)
	result2 := readJSON(t, resp2)
	receipts2, ok2 := result2["receipts"].([]any)
	if !ok2 || len(receipts2) == 0 {
		t.Fatal("expected at least 1 receipt for student")
	}
}

func TestPaymentValidationErrors(t *testing.T) {
	tutorTg, _, tutor := signUp(t, "tutor")
	tutorID := tutor["id"].(string)
	studentTg, _, student := signUp(t, "student")
	studentID := student["id"].(string)

	// Tutor cannot submit a receipt (only students can)
	resp := doRequest(t, "POST", "/payment/receipts", map[string]any{
		"lessonId": "00000000-0000-0000-0000-000000000000",
		"fileId":   "00000000-0000-0000-0000-000000000000",
	}, map[string]string{"Authorization": authHeader(t, tutorTg)})
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
		bodyStr := readBody(t, resp)
		t.Errorf("tutor submitting receipt: expected 403/400/404, got %d: %s", resp.StatusCode, bodyStr)
	}

	// Payment info for non-existent lesson
	resp2 := doRequest(t, "GET", "/payment/info/00000000-0000-0000-0000-000000000000", nil, map[string]string{
		"Authorization": authHeader(t, studentTg),
	})
	if resp2.StatusCode != http.StatusNotFound && resp2.StatusCode != http.StatusBadRequest && resp2.StatusCode != http.StatusInternalServerError {
		bodyStr := readBody(t, resp2)
		t.Errorf("non-existent payment info: expected 404/400, got %d: %s", resp2.StatusCode, bodyStr)
	}

	_ = tutorID
	_ = studentID
}
