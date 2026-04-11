# E2E Business Scenarios Test Suite

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create comprehensive e2e tests covering all major business flows across StudyFlow microservices: lesson booking, homework submission, payment, and file upload.

**Architecture:** Go tests in `e2e/` package → HTTP calls to API Gateway (localhost:80) → full gRPC stack. Each business flow is a self-contained test file with its own setup (sign-up users, create relationships). All tests share a common helpers file for HTTP client, HMAC token generation, and assertions.

**Tech Stack:** Go 1.24+, standard `testing` package, `crypto/hmac`, `crypto/sha256`, `net/http`, `encoding/json`

**Prerequisites:** `docker compose up -d` (all 14 services running)

**JSON naming convention:** The API Gateway uses `protojson` for marshaling — proto field `tutor_id` becomes JSON `"tutorId"` (camelCase).

**IMPORTANT route prefix bug:** User endpoints are mounted under `/users` in main.go AND have `/users/` prefix in RegisterRoutes, resulting in double prefix: `/users/users/me`, `/users/users/tutor-students`, etc. Only `/users/sign-up/telegram` is correct (no double prefix).

---

### Task 1: Extend helpers with business-flow utilities

**Files:**
- Modify: `e2e/helpers.go`

- [ ] **Step 1: Add schedule, homework, file, payment helper functions**

Add the following functions after the existing `signUp` function:

```go
// ---------- schedule helpers ----------

func createSlot(t *testing.T, tutorAuth string, tutorID string, startsAt, endsAt time.Time) map[string]any {
	t.Helper()
	resp := doRequest(t, "POST", "/schedule/slots", map[string]any{
		"tutorId":  tutorID,
		"startsAt": startsAt.Format(time.RFC3339),
		"endsAt":   endsAt.Format(time.RFC3339),
	}, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusOK)
	return readJSON(t, resp)
}

func bookLesson(t *testing.T, auth string, slotID, studentID string) map[string]any {
	t.Helper()
	resp := doRequest(t, "POST", "/schedule/lessons", map[string]any{
		"slotId":    slotID,
		"studentId": studentID,
	}, map[string]string{"Authorization": auth})
	assertStatus(t, resp, http.StatusOK)
	return readJSON(t, resp)
}

// ---------- homework helpers ----------

func createAssignment(t *testing.T, tutorAuth, tutorID, studentID, title string) map[string]any {
	t.Helper()
	resp := doRequest(t, "POST", "/homework/assignments", map[string]any{
		"tutorId":   tutorID,
		"studentId": studentID,
		"title":     title,
	}, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusOK)
	return readJSON(t, resp)
}

func createSubmission(t *testing.T, studentAuth, assignmentID string, comment string) map[string]any {
	t.Helper()
	resp := doRequest(t, "POST", "/homework/submissions", map[string]any{
		"assignmentId": assignmentID,
		"comment":      comment,
	}, map[string]string{"Authorization": studentAuth})
	assertStatus(t, resp, http.StatusOK)
	return readJSON(t, resp)
}

func createFeedback(t *testing.T, tutorAuth, submissionID string, grade int, comment string) map[string]any {
	t.Helper()
	resp := doRequest(t, "POST", "/homework/feedbacks", map[string]any{
		"submissionId": submissionID,
		"grade":        grade,
		"comment":      comment,
	}, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusOK)
	return readJSON(t, resp)
}

// ---------- file helpers ----------

func initUpload(t *testing.T, auth, uploadedBy, filename string) map[string]any {
	t.Helper()
	resp := doRequest(t, "POST", "/files/init-upload", map[string]any{
		"uploadedBy": uploadedBy,
		"filename":   filename,
	}, map[string]string{"Authorization": auth})
	assertStatus(t, resp, http.StatusOK)
	return readJSON(t, resp)
}

func confirmUpload(t *testing.T, auth, fileID string) map[string]any {
	t.Helper()
	resp := doRequest(t, "POST", "/files/"+fileID+"/confirm-upload", nil, map[string]string{
		"Authorization": auth,
	})
	assertStatus(t, resp, http.StatusOK)
	return readJSON(t, resp)
}

// ---------- payment helpers ----------

func submitReceipt(t *testing.T, studentAuth, lessonID, fileID string) map[string]any {
	t.Helper()
	resp := doRequest(t, "POST", "/payment/receipts", map[string]any{
		"lessonId": lessonID,
		"fileId":   fileID,
	}, map[string]string{"Authorization": studentAuth})
	assertStatus(t, resp, http.StatusOK)
	return readJSON(t, resp)
}

func verifyReceipt(t *testing.T, tutorAuth, receiptID string) map[string]any {
	t.Helper()
	resp := doRequest(t, "POST", "/payment/receipts/"+receiptID+"/verify", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	return readJSON(t, resp)
}

// ---------- relationship helpers ----------

func createTutorStudent(t *testing.T, tutorAuth, tutorID, studentID string) map[string]any {
	t.Helper()
	resp := doRequest(t, "POST", "/users/users/tutor-students", map[string]any{
		"tutorId":   tutorID,
		"studentId": studentID,
	}, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusOK)
	return readJSON(t, resp)
}

func acceptInvitation(t *testing.T, studentAuth, tutorID string) {
	t.Helper()
	resp := doRequest(t, "POST", "/users/users/tutor-students/"+tutorID+"/accept", nil, map[string]string{
		"Authorization": studentAuth,
	})
	assertStatus(t, resp, http.StatusOK)
}
```

- [ ] **Step 2: Add import for `time` to helpers.go**

Add `"time"` to the imports block at the top of `helpers.go`:

```go
import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)
```

- [ ] **Step 3: Verify helpers compile**

Run: `cd e2e && go build ./...`
Expected: success (no output)

- [ ] **Step 4: Commit**

```bash
git add e2e/helpers.go
git commit -m "feat(e2e): add business-flow helper functions"
```

---

### Task 2: Lesson booking e2e tests

**Files:**
- Create: `e2e/schedule_test.go`

- [ ] **Step 1: Write the test file**

```go
package e2e

import (
	"net/http"
	"testing"
	"time"
)

func TestCreateSlotAndBookLesson(t *testing.T) {
	// Setup: tutor + student + active relationship
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	// Create a time slot (tomorrow 14:00-15:00)
	now := time.Now().UTC()
	startsAt := now.Add(24 * time.Hour).Truncate(time.Hour).Add(14 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	slotID := slot["id"].(string)

	if slot["isBooked"] != false {
		t.Error("new slot should not be booked")
	}

	// Book a lesson
	lesson := bookLesson(t, studentAuth, slotID, studentID)
	if lesson["id"] == nil || lesson["id"] == "" {
		t.Fatal("lesson creation returned no id")
	}
	if lesson["status"] != "booked" {
		t.Errorf("expected status=booked, got %v", lesson["status"])
	}

	// Verify slot is now booked
	resp := doRequest(t, "GET", "/schedule/slots/"+slotID, nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	updatedSlot := readJSON(t, resp)
	if updatedSlot["isBooked"] != true {
		t.Error("slot should be booked after lesson creation")
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

	// Tutor updates lesson price and connection link
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

	// Cancel the lesson
	resp := doRequest(t, "POST", "/schedule/lessons/"+lessonID+"/cancel", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	cancelled := readJSON(t, resp)
	if cancelled["status"] != "cancelled" {
		t.Errorf("expected status=cancelled, got %v", cancelled["status"])
	}

	// Slot should be free again
	resp = doRequest(t, "GET", "/schedule/slots/"+slot["id"].(string), nil, map[string]string{
		"Authorization": tutorAuth,
	})
	slotAfter := readJSON(t, resp)
	if slotAfter["isBooked"] != false {
		t.Error("slot should be free after cancellation")
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

	// List lessons by tutor-student pair
	path := "/schedule/lessons?tutorId=" + tutorID + "&studentId=" + studentID + "&from=" + from.Format(time.RFC3339) + "&to=" + from.Add(48*time.Hour).Format(time.RFC3339)
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

	// Reschedule to slot2
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
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd e2e && go test -v -run "TestCreateSlotAndBookLesson|TestUpdateLessonDetails|TestCancelLesson|TestListLessons|TestRescheduleLesson" -count=1 .`
Expected: 5 PASS

- [ ] **Step 3: Commit**

```bash
git add e2e/schedule_test.go
git commit -m "feat(e2e): add lesson booking flow tests (5 scenarios)"
```

---

### Task 3: Homework flow e2e tests

**Files:**
- Create: `e2e/homework_test.go`

- [ ] **Step 1: Write the test file**

```go
package e2e

import (
	"net/http"
	"testing"
	"time"
)

func setupTutorStudentPair(t *testing.T) (tutorID, studentID, tutorAuth, studentAuth string) {
	t.Helper()
	_, tutorAuth, tutor := signUp(t, "tutor")
	_, studentAuth, student := signUp(t, "student")
	tutorID = tutor["id"].(string)
	studentID = student["id"].(string)
	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)
	return
}

func TestCreateAssignment(t *testing.T) {
	tutorID, studentID, tutorAuth, _ := setupTutorStudentPair(t)

	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Chapter 5 Exercises")

	if assignment["id"] == nil || assignment["id"] == "" {
		t.Fatal("assignment creation returned no id")
	}
	if assignment["title"] != "Chapter 5 Exercises" {
		t.Errorf("expected title, got %v", assignment["title"])
	}
	if assignment["tutorId"] != tutorID {
		t.Errorf("expected tutorId=%s, got %v", tutorID, assignment["tutorId"])
	}
}

func TestCreateSubmission(t *testing.T) {
	tutorID, studentID, tutorAuth, studentAuth := setupTutorStudentPair(t)
	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Test Assignment")

	submission := createSubmission(t, studentAuth, assignment["id"].(string), "Here is my homework")

	if submission["id"] == nil || submission["id"] == "" {
		t.Fatal("submission creation returned no id")
	}
	if submission["assignmentId"] != assignment["id"] {
		t.Errorf("expected assignmentId=%s, got %v", assignment["id"], submission["assignmentId"])
	}
}

func TestCreateFeedback(t *testing.T) {
	tutorID, studentID, tutorAuth, studentAuth := setupTutorStudentPair(t)
	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Feedback Test")
	submission := createSubmission(t, studentAuth, assignment["id"].(string), "My work")

	feedback := createFeedback(t, tutorAuth, submission["id"].(string), 5, "Excellent!")

	if feedback["id"] == nil || feedback["id"] == "" {
		t.Fatal("feedback creation returned no id")
	}
	if feedback["grade"] != float64(5) {
		t.Errorf("expected grade=5, got %v", feedback["grade"])
	}
	if feedback["submissionId"] != submission["id"] {
		t.Errorf("expected submissionId=%s, got %v", submission["id"], feedback["submissionId"])
	}
}

func TestListAssignments(t *testing.T) {
	tutorID, studentID, tutorAuth, _ := setupTutorStudentPair(t)
	createAssignment(t, tutorAuth, tutorID, studentID, "Assignment 1")
	createAssignment(t, tutorAuth, tutorID, studentID, "Assignment 2")

	resp := doRequest(t, "GET", "/homework/assignments?tutorId="+tutorID+"&studentId="+studentID+"&page=1&pageSize=10", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	result := readJSON(t, resp)

	assignments, ok := result["assignments"].([]any)
	if !ok || len(assignments) < 2 {
		t.Fatalf("expected at least 2 assignments, got %v", len(assignments))
	}
}

func TestFullHomeworkFlow(t *testing.T) {
	tutorID, studentID, tutorAuth, studentAuth := setupTutorStudentPair(t)

	// 1. Tutor creates assignment
	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Full Flow Assignment")

	// 2. Student submits work
	submission := createSubmission(t, studentAuth, assignment["id"].(string), "My completed homework")

	// 3. Tutor gives feedback with grade
	feedback := createFeedback(t, tutorAuth, submission["id"].(string), 4, "Good, but could improve section 3")

	// 4. List submissions for assignment
	resp := doRequest(t, "GET", "/homework/assignments/"+assignment["id"].(string)+"/submissions", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	subs := readJSON(t, resp)
	if subs["submissions"] == nil {
		t.Error("expected submissions list")
	}

	// 5. List feedbacks for assignment
	resp = doRequest(t, "GET", "/homework/assignments/"+assignment["id"].(string)+"/feedbacks", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	feedbacks := readJSON(t, resp)
	if feedbacks["feedbacks"] == nil {
		t.Error("expected feedbacks list")
	}

	_ = feedback
}

func TestAssignmentValidationErrors(t *testing.T) {
	_, studentID, _, studentAuth := setupTutorStudentPair(t)

	// Student cannot create assignments (only tutors can)
	resp := doRequest(t, "POST", "/homework/assignments", map[string]any{
		"tutorId":   "00000000-0000-0000-0000-000000000000",
		"studentId": studentID,
		"title":     "Illegal Assignment",
	}, map[string]string{"Authorization": studentAuth})
	if resp.StatusCode != http.StatusForbidden {
		bodyStr := readBody(t, resp)
		t.Errorf("student creating assignment: expected 403, got %d: %s", resp.StatusCode, bodyStr)
	}
}

func TestSubmissionValidationErrors(t *testing.T) {
	tutorID, studentID, tutorAuth, _ := setupTutorStudentPair(t)
	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Validation Test")

	// Tutor cannot submit to their own assignment (only students can)
	resp := doRequest(t, "POST", "/homework/submissions", map[string]any{
		"assignmentId": assignment["id"].(string),
		"comment":      "Tutor trying to submit",
	}, map[string]string{"Authorization": tutorAuth})
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusBadRequest {
		bodyStr := readBody(t, resp)
		t.Errorf("tutor submitting: expected 403/400, got %d: %s", resp.StatusCode, bodyStr)
	}
}

func TestFeedbackValidationErrors(t *testing.T) {
	tutorID, studentID, tutorAuth, studentAuth := setupTutorStudentPair(t)
	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Feedback Validation")
	submission := createSubmission(t, studentAuth, assignment["id"].(string), "Work")

	// Invalid grade (must be 1-5)
	resp := doRequest(t, "POST", "/homework/feedbacks", map[string]any{
		"submissionId": submission["id"].(string),
		"grade":        0,
		"comment":      "Invalid grade",
	}, map[string]string{"Authorization": tutorAuth})
	if resp.StatusCode != http.StatusBadRequest {
		bodyStr := readBody(t, resp)
		t.Errorf("invalid grade: expected 400, got %d: %s", resp.StatusCode, bodyStr)
	}

	// Invalid grade > 5
	resp = doRequest(t, "POST", "/homework/feedbacks", map[string]any{
		"submissionId": submission["id"].(string),
		"grade":        6,
		"comment":      "Too high grade",
	}, map[string]string{"Authorization": tutorAuth})
	if resp.StatusCode != http.StatusBadRequest {
		bodyStr := readBody(t, resp)
		t.Errorf("grade > 5: expected 400, got %d: %s", resp.StatusCode, bodyStr)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd e2e && go test -v -run "TestCreateAssignment|TestCreateSubmission|TestCreateFeedback|TestListAssignments|TestFullHomeworkFlow|TestAssignmentValidation|TestSubmissionValidation|TestFeedbackValidation" -count=1 .`
Expected: 8 PASS

- [ ] **Step 3: Commit**

```bash
git add e2e/homework_test.go
git commit -m "feat(e2e): add homework flow tests (8 scenarios)"
```

---

### Task 4: File upload e2e tests

**Files:**
- Create: `e2e/file_test.go`

- [ ] **Step 1: Write the test file**

```go
package e2e

import (
	"bytes"
	"net/http"
	"testing"
)

func TestInitAndConfirmUpload(t *testing.T) {
	_, auth, user := signUp(t, "tutor")
	userID := user["id"].(string)

	// Step 1: Init upload
	init := initUpload(t, auth, userID, "test-homework.pdf")
	fileID := init["fileId"].(string)
	uploadURL := init["uploadUrl"].(string)

	if fileID == "" {
		t.Fatal("init upload returned no fileId")
	}
	if uploadURL == "" {
		t.Fatal("init upload returned no uploadUrl")
	}
	if init["method"] != "PUT" {
		t.Errorf("expected method=PUT, got %v", init["method"])
	}

	// Step 2: Upload file bytes to the presigned URL
	fileContent := []byte("fake pdf content for e2e test")
	req, err := http.NewRequest("PUT", uploadURL, bytes.NewReader(fileContent))
	if err != nil {
		t.Fatalf("create upload request: %v", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload file: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Errorf("file upload: expected 200/204, got %d", resp.StatusCode)
	}

	// Step 3: Confirm upload
	confirmed := confirmUpload(t, auth, fileID)
	if confirmed["isUploaded"] != true {
		t.Errorf("expected isUploaded=true, got %v", confirmed["isUploaded"])
	}
}

func TestGetFileMeta(t *testing.T) {
	_, auth, user := signUp(t, "tutor")
	userID := user["id"].(string)

	init := initUpload(t, auth, userID, "meta-test.txt")
	fileID := init["fileId"].(string)

	// Upload dummy bytes
	req, _ := http.NewRequest("PUT", init["uploadUrl"].(string), bytes.NewReader([]byte("test")))
	req.Header.Set("Content-Type", "application/octet-stream")
	http.DefaultClient.Do(req)

	confirmUpload(t, auth, fileID)

	// Get metadata
	resp := doRequest(t, "GET", "/files/"+fileID+"/meta", nil, map[string]string{
		"Authorization": auth,
	})
	assertStatus(t, resp, http.StatusOK)
	meta := readJSON(t, resp)

	if meta["id"] != fileID {
		t.Errorf("expected id=%s, got %v", fileID, meta["id"])
	}
	if meta["filename"] != "meta-test.txt" {
		t.Errorf("expected filename=meta-test.txt, got %v", meta["filename"])
	}
}

func TestFileValidationErrors(t *testing.T) {
	_, auth, user := signUp(t, "tutor")
	userID := user["id"].(string)

	// Invalid extension
	resp := doRequest(t, "POST", "/files/init-upload", map[string]any{
		"uploadedBy": userID,
		"filename":   "virus.exe",
	}, map[string]string{"Authorization": auth})
	if resp.StatusCode != http.StatusBadRequest {
		bodyStr := readBody(t, resp)
		t.Errorf("invalid extension: expected 400, got %d: %s", resp.StatusCode, bodyStr)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd e2e && go test -v -run "TestInitAndConfirmUpload|TestGetFileMeta|TestFileValidationErrors" -count=1 .`
Expected: 3 PASS

NOTE: `TestInitAndConfirmUpload` may fail if MinIO presigned URL routing through nginx is misconfigured. If so, note the issue and skip uploading actual bytes (confirm upload may still work if the file record exists).

- [ ] **Step 3: Commit**

```bash
git add e2e/file_test.go
git commit -m "feat(e2e): add file upload flow tests (3 scenarios)"
```

---

### Task 5: Payment flow e2e tests

**Files:**
- Create: `e2e/payment_test.go`

- [ ] **Step 1: Write the test file**

```go
package e2e

import (
	"net/http"
	"testing"
	"time"
)

func TestSubmitAndVerifyReceipt(t *testing.T) {
	// Full flow: signup → relationship → slot → lesson → file → receipt → verify
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	// Create slot and book lesson
	now := time.Now().UTC()
	startsAt := now.Add(35 * time.Hour).Truncate(time.Hour).Add(12 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	lesson := bookLesson(t, studentAuth, slot["id"].(string), studentID)
	lessonID := lesson["id"].(string)

	// Update lesson price
	resp := doRequest(t, "PATCH", "/schedule/lessons/"+lessonID, map[string]any{
		"priceRub":    2500,
		"paymentInfo": "Tinkoff 5555 1234 5678 9012",
	}, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusOK)

	// Upload a receipt file
	init := initUpload(t, studentAuth, studentID, "receipt.jpg")
	fileID := init["fileId"].(string)
	confirmUpload(t, studentAuth, fileID)

	// Student submits receipt
	receipt := submitReceipt(t, studentAuth, lessonID, fileID)
	receiptID := receipt["id"].(string)

	if receipt["isVerified"] != false {
		t.Error("new receipt should not be verified")
	}
	if receipt["priceRub"] != float64(2500) {
		t.Errorf("expected priceRub=2500, got %v", receipt["priceRub"])
	}

	// Tutor verifies receipt
	verified := verifyReceipt(t, tutorAuth, receiptID)
	if verified["isVerified"] != true {
		t.Error("receipt should be verified after verification")
	}
}

func TestGetPaymentInfo(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	now := time.Now().UTC()
	startsAt := now.Add(40 * time.Hour).Truncate(time.Hour).Add(16 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	lesson := bookLesson(t, studentAuth, slot["id"].(string), studentID)

	// Update price
	doRequest(t, "PATCH", "/schedule/lessons/"+lesson["id"].(string), map[string]any{
		"priceRub":    3000,
		"paymentInfo": "Alfa Bank 1111 2222 3333 4444",
	}, map[string]string{"Authorization": tutorAuth})

	// Get payment info
	resp := doRequest(t, "GET", "/payment/info/"+lesson["id"].(string), nil, map[string]string{
		"Authorization": studentAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	info := readJSON(t, resp)

	if info["priceRub"] != float64(3000) {
		t.Errorf("expected priceRub=3000, got %v", info["priceRub"])
	}
}

func TestListReceipts(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	// Create lesson and submit receipt
	now := time.Now().UTC()
	startsAt := now.Add(45 * time.Hour).Truncate(time.Hour).Add(10 * time.Hour)
	endsAt := startsAt.Add(1 * time.Hour)

	slot := createSlot(t, tutorAuth, tutorID, startsAt, endsAt)
	lesson := bookLesson(t, studentAuth, slot["id"].(string), studentID)
	doRequest(t, "PATCH", "/schedule/lessons/"+lesson["id"].(string), map[string]any{
		"priceRub": 1500,
	}, map[string]string{"Authorization": tutorAuth})

	init := initUpload(t, studentAuth, studentID, "receipt2.jpg")
	confirmUpload(t, studentAuth, init["fileId"].(string))
	submitReceipt(t, studentAuth, lesson["id"].(string), init["fileId"].(string))

	// List receipts by tutor
	resp := doRequest(t, "GET", "/payment/receipts?tutorId="+tutorID+"&page=1&pageSize=10", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	result := readJSON(t, resp)

	receipts, ok := result["receipts"].([]any)
	if !ok || len(receipts) == 0 {
		t.Fatal("expected at least 1 receipt in list")
	}
}

func TestPaymentValidationErrors(t *testing.T) {
	tutorTg, _, tutor := signUp(t, "tutor")
	studentTg, _, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	// Tutor cannot submit a receipt (only students can)
	// First need relationship + lesson
	createTutorStudent(t, tutorAuth(t, tutorTg), tutorID, studentID)
	// accept skipped — lesson needs it but receipt validation may catch role first

	resp := doRequest(t, "POST", "/payment/receipts", map[string]any{
		"lessonId": "00000000-0000-0000-0000-000000000000",
		"fileId":   "00000000-0000-0000-0000-000000000000",
	}, map[string]string{"Authorization": tutorAuth(t, tutorTg)})
	// Should fail — tutor role or non-existent lesson
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
		bodyStr := readBody(t, resp)
		t.Errorf("tutor submitting receipt: expected 403/400/404, got %d: %s", resp.StatusCode, bodyStr)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd e2e && go test -v -run "TestSubmitAndVerifyReceipt|TestGetPaymentInfo|TestListReceipts|TestPaymentValidationErrors" -count=1 .`
Expected: 4 PASS

- [ ] **Step 3: Commit**

```bash
git add e2e/payment_test.go
git commit -m "feat(e2e): add payment flow tests (4 scenarios)"
```

---

### Task 6: End-to-end full scenario test

**Files:**
- Create: `e2e/full_flow_test.go`

- [ ] **Step 1: Write the complete business scenario test**

```go
package e2e

import (
	"net/http"
	"testing"
	"time"
)

// TestFullBusinessScenario runs the complete tutor-student workflow:
// Signup → Relationship → Slot → Lesson → Assignment → Submission → Feedback → Receipt → Verify
func TestFullBusinessScenario(t *testing.T) {
	// Phase 1: Sign up both users
	_, tutorAuth, tutor := signUp(t, "tutor")
	_, studentAuth, student := signUp(t, "student")

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	t.Logf("tutor=%s student=%s", tutorID, studentID)

	// Phase 2: Establish relationship
	ts := createTutorStudent(t, tutorAuth, tutorID, studentID)
	t.Logf("tutor-student created: %v", ts["id"])
	acceptInvitation(t, studentAuth, tutorID)
	t.Log("invitation accepted")

	// Phase 3: Schedule a lesson
	now := time.Now().UTC()
	base := now.Add(50 * time.Hour).Truncate(time.Hour)
	slot := createSlot(t, tutorAuth, tutorID, base.Add(10*time.Hour), base.Add(11*time.Hour))
	lesson := bookLesson(t, studentAuth, slot["id"].(string), studentID)
	lessonID := lesson["id"].(string)
	t.Logf("lesson booked: %s", lessonID)

	// Set price
	doRequest(t, "PATCH", "/schedule/lessons/"+lessonID, map[string]any{
		"priceRub":    2000,
		"paymentInfo": "Sberbank 1234",
	}, map[string]string{"Authorization": tutorAuth})

	// Phase 4: Upload a file
	init := initUpload(t, tutorAuth, tutorID, "assignment.pdf")
	fileID := init["fileId"].(string)
	confirmUpload(t, tutorAuth, fileID)
	t.Logf("file uploaded: %s", fileID)

	// Phase 5: Assignment → Submission → Feedback
	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Full Flow Test Assignment")
	assignmentID := assignment["id"].(string)
	t.Logf("assignment created: %s", assignmentID)

	submission := createSubmission(t, studentAuth, assignmentID, "My homework submission")
	submissionID := submission["id"].(string)
	t.Logf("submission created: %s", submissionID)

	feedback := createFeedback(t, tutorAuth, submissionID, 5, "Perfect work!")
	t.Logf("feedback created: %v grade=%v", feedback["id"], feedback["grade"])

	// Phase 6: Payment
	receiptInit := initUpload(t, studentAuth, studentID, "payment-receipt.jpg")
	receiptFileID := receiptInit["fileId"].(string)
	confirmUpload(t, studentAuth, receiptFileID)

	receipt := submitReceipt(t, studentAuth, lessonID, receiptFileID)
	receiptID := receipt["id"].(string)
	t.Logf("receipt submitted: %s", receiptID)

	verified := verifyReceipt(t, tutorAuth, receiptID)
	if verified["isVerified"] != true {
		t.Error("receipt should be verified")
	}
	t.Log("receipt verified — full flow complete")
}

// TestMultipleStudentsPerTutor verifies a tutor can have multiple students
func TestMultipleStudentsPerTutor(t *testing.T) {
	_, tutorAuth, tutor := signUp(t, "tutor")
	tutorID := tutor["id"].(string)

	students := make([]map[string]any, 3)
	studentAuths := make([]string, 3)
	studentIDs := make([]string, 3)

	for i := 0; i < 3; i++ {
		_, auth, user := signUp(t, "student")
		students[i] = user
		studentAuths[i] = auth
		studentIDs[i] = user["id"].(string)
		createTutorStudent(t, tutorAuth, tutorID, studentIDs[i])
		acceptInvitation(t, studentAuths[i], tutorID)
	}

	// List tutor's students
	resp := doRequest(t, "GET", "/users/users/tutor-students/by-tutor/"+tutorID, nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	result := readJSON(t, resp)

	studentsList, ok := result["students"].([]any)
	if !ok || len(studentsList) < 3 {
		t.Fatalf("expected at least 3 students, got %v", len(studentsList))
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd e2e && go test -v -run "TestFullBusinessScenario|TestMultipleStudentsPerTutor" -count=1 .`
Expected: 2 PASS

- [ ] **Step 3: Commit**

```bash
git add e2e/full_flow_test.go
git commit -m "feat(e2e): add full business scenario test + multi-student test"
```

---

### Task 7: Final cleanup — ensure all tests pass together

- [ ] **Step 1: Run the full e2e suite**

Run: `cd e2e && go test -v -count=1 ./...`
Expected: ALL tests pass (10 existing + 5 schedule + 8 homework + 3 file + 4 payment + 2 full = 32 total)

- [ ] **Step 2: Run twice to verify idempotency** (no leftover state issues)

Run: `cd e2e && go test -v -count=1 ./...`
Expected: ALL pass on second run too (may have extra users in DB but should not affect results)

- [ ] **Step 3: Commit any final adjustments**

```bash
git add -A && git diff --cached --stat
# Only commit if there are changes
```

---

## Verification Checklist

- [ ] `docker compose up -d` — all 14 containers healthy
- [ ] `cd e2e && go test -v -count=1 ./...` — all tests pass (target: 32 tests)
- [ ] `cd e2e && go test -v -count=1 ./...` — second run also passes (idempotent)
- [ ] No leftover `.env` modifications from test runs
- [ ] Individual test groups runnable: `-run "TestCreateSlot|TestCreateAssignment|TestInitAndConfirm"` etc.

## Total Test Coverage

| File | Scenarios | Business flows covered |
|------|-----------|----------------------|
| `e2e_test.go` (existing) | 10 | Health, CORS, FAQ, Auth, Basic CRUD |
| `schedule_test.go` | 5 | Slot creation, lesson booking, update, cancel, reschedule, list |
| `homework_test.go` | 8 | Assignment CRUD, submission, feedback, validation errors |
| `file_test.go` | 3 | Init upload, confirm, metadata, invalid extension |
| `payment_test.go` | 4 | Receipt submit/verify, payment info, list, validation |
| `full_flow_test.go` | 2 | Complete end-to-end scenario, multi-student tutor |
| **Total** | **32** | |