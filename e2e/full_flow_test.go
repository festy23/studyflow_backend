package e2e

import (
	"net/http"
	"testing"
	"time"
)

// TestFullBusinessScenario: Signup → Relationship → Slot → Lesson → Assignment → Submission → Feedback → Receipt → Verify
func TestFullBusinessScenario(t *testing.T) {
	_, tutorAuth, tutor := signUp(t, "tutor")
	_, studentAuth, student := signUp(t, "student")

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	t.Logf("tutor=%s student=%s", tutorID, studentID)

	// Phase 1: Relationship
	ts := createTutorStudent(t, tutorAuth, tutorID, studentID)
	t.Logf("tutor-student created: %v", ts["id"])
	acceptInvitation(t, studentAuth, tutorID)

	// Phase 2: Schedule lesson
	now := time.Now().UTC()
	base := now.Add(50 * time.Hour).Truncate(time.Hour)
	slot := createSlot(t, tutorAuth, tutorID, base.Add(10*time.Hour), base.Add(11*time.Hour))
	lesson := bookLesson(t, studentAuth, slot["id"].(string), studentID)
	lessonID := lesson["id"].(string)
	t.Logf("lesson booked: %s", lessonID)

	doRequest(t, "PATCH", "/schedule/lessons/"+lessonID, map[string]any{
		"priceRub":    2000,
		"paymentInfo": "Sberbank 1234",
	}, map[string]string{"Authorization": tutorAuth})

	// Phase 3: Assignment → Submission → Feedback
	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Full Flow Test Assignment")
	assignmentID := assignment["id"].(string)
	t.Logf("assignment: %s", assignmentID)

	submission := createSubmission(t, studentAuth, assignmentID, "My homework submission")
	submissionID := submission["id"].(string)
	t.Logf("submission: %s", submissionID)

	feedback := createFeedback(t, tutorAuth, submissionID, 5, "Perfect work!")
	t.Logf("feedback: %v grade=%v", feedback["id"], feedback["grade"])

	// Phase 4: Payment
	receiptInit := initUpload(t, studentAuth, studentID, "payment-receipt.jpg")
	receiptFileID := receiptInit["fileId"].(string)
	confirmUpload(t, studentAuth, receiptFileID)

	receipt := submitReceipt(t, studentAuth, lessonID, receiptFileID)
	receiptID := receipt["id"].(string)
	t.Logf("receipt: %s", receiptID)

	verified := verifyReceipt(t, tutorAuth, receiptID)
	if verified["isVerified"] != true {
		t.Error("receipt should be verified")
	}
	t.Log("full business flow complete")
}

func TestMultipleStudentsPerTutor(t *testing.T) {
	_, tutorAuth, tutor := signUp(t, "tutor")
	tutorID := tutor["id"].(string)

	for i := 0; i < 3; i++ {
		_, studentAuth, student := signUp(t, "student")
		studentID := student["id"].(string)
		createTutorStudent(t, tutorAuth, tutorID, studentID)
		acceptInvitation(t, studentAuth, tutorID)
	}

	resp := doRequest(t, "GET", "/users/tutor-students/by-tutor/"+tutorID, nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	result := readJSON(t, resp)

	studentsList, ok := result["students"].([]any)
	if !ok || len(studentsList) < 3 {
		t.Fatalf("expected at least 3 students, got %v", len(studentsList))
	}
}
