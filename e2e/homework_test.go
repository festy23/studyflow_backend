package e2e

import (
	"net/http"
	"testing"
)

// TestCreateAssignment verifies that a tutor can create a homework assignment
// and the response contains all expected fields.
func TestCreateAssignment(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Algebra Homework")

	if assignment["id"] == nil || assignment["id"] == "" {
		t.Fatal("expected non-empty id")
	}
	if assignment["tutorId"] != tutorID {
		t.Errorf("tutorId: got %v, want %s", assignment["tutorId"], tutorID)
	}
	if assignment["studentId"] != studentID {
		t.Errorf("studentId: got %v, want %s", assignment["studentId"], studentID)
	}
	if assignment["title"] != "Algebra Homework" {
		t.Errorf("title: got %v, want 'Algebra Homework'", assignment["title"])
	}
	if assignment["createdAt"] == nil {
		t.Error("expected createdAt field")
	}
	if assignment["editedAt"] == nil {
		t.Error("expected editedAt field")
	}
}

// TestCreateSubmission verifies that a student can submit homework
// and the response contains all expected fields.
func TestCreateSubmission(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Physics Homework")
	submission := createSubmission(t, studentAuth, assignment["id"].(string), "My solution")

	if submission["id"] == nil || submission["id"] == "" {
		t.Fatal("expected non-empty id")
	}
	if submission["assignmentId"] != assignment["id"] {
		t.Errorf("assignmentId: got %v, want %s", submission["assignmentId"], assignment["id"])
	}
	if submission["comment"] != "My solution" {
		t.Errorf("comment: got %v, want 'My solution'", submission["comment"])
	}
	if submission["createdAt"] == nil {
		t.Error("expected createdAt field")
	}
	if submission["editedAt"] == nil {
		t.Error("expected editedAt field")
	}
}

// TestCreateFeedback verifies that a tutor can create feedback with a grade
// and the response contains all expected fields.
func TestCreateFeedback(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Chemistry Homework")
	submission := createSubmission(t, studentAuth, assignment["id"].(string), "Lab report")
	feedback := createFeedback(t, tutorAuth, submission["id"].(string), 4, "Good work")

	if feedback["id"] == nil || feedback["id"] == "" {
		t.Fatal("expected non-empty id")
	}
	if feedback["submissionId"] != submission["id"] {
		t.Errorf("submissionId: got %v, want %s", feedback["submissionId"], submission["id"])
	}
	if feedback["grade"] != float64(4) {
		t.Errorf("grade: got %v, want 4", feedback["grade"])
	}
	if feedback["comment"] != "Good work" {
		t.Errorf("comment: got %v, want 'Good work'", feedback["comment"])
	}
	if feedback["createdAt"] == nil {
		t.Error("expected createdAt field")
	}
	if feedback["editedAt"] == nil {
		t.Error("expected editedAt field")
	}
}

// TestListAssignments creates multiple assignments and verifies listing by tutor.
func TestListAssignments(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	createAssignment(t, tutorAuth, tutorID, studentID, "History Essay")
	createAssignment(t, tutorAuth, tutorID, studentID, "Geography Project")

	resp := doRequest(t, "GET", "/homework/assignments?tutor_id="+tutorID+"&page=1&page_size=10", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	result := readJSON(t, resp)

	assignments, ok := result["assignments"].([]any)
	if !ok {
		t.Fatal("expected assignments array in response")
	}
	if len(assignments) < 2 {
		t.Fatalf("expected at least 2 assignments, got %d", len(assignments))
	}
}

// TestFullHomeworkFlow exercises the complete homework lifecycle:
// assignment -> submission -> feedback -> list submissions -> list feedbacks.
func TestFullHomeworkFlow(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	// 1. Create assignment
	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Biology Homework")
	assignID := assignment["id"].(string)

	// 2. Create submission
	submission := createSubmission(t, studentAuth, assignID, "Biology report")
	subID := submission["id"].(string)

	// 3. Create feedback
	feedback := createFeedback(t, tutorAuth, subID, 5, "Excellent")
	_ = feedback

	// 4. List submissions for this assignment
	resp := doRequest(t, "GET", "/homework/assignments/"+assignID+"/submissions", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	subsResult := readJSON(t, resp)
	subs, ok := subsResult["submissions"].([]any)
	if !ok || len(subs) == 0 {
		t.Fatal("expected at least one submission in list")
	}

	// 5. List feedbacks for this assignment
	resp = doRequest(t, "GET", "/homework/assignments/"+assignID+"/feedbacks", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	fbResult := readJSON(t, resp)
	fbs, ok := fbResult["feedbacks"].([]any)
	if !ok || len(fbs) == 0 {
		t.Fatal("expected at least one feedback in list")
	}
}

// TestAssignmentValidationErrors verifies that only tutors can create assignments.
func TestAssignmentValidationErrors(t *testing.T) {
	_, studentAuth, student := signUp(t, "student")
	studentID := student["id"].(string)

	resp := doRequest(t, "POST", "/homework/assignments", map[string]any{
		"tutorId":   studentID,
		"studentId": studentID,
		"title":     "Should Fail",
	}, map[string]string{"Authorization": studentAuth})
	assertStatus(t, resp, http.StatusForbidden)
}

// TestSubmissionValidationErrors verifies that only the assigned student can submit.
func TestSubmissionValidationErrors(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Permission Test")

	// Tutor tries to submit to their own assignment (only the student can submit)
	resp := doRequest(t, "POST", "/homework/submissions", map[string]any{
		"assignmentId": assignment["id"],
		"comment":      "Tutor attempt",
	}, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusForbidden)
}

// TestFeedbackValidationErrors verifies that grade must be in the 1-5 range.
func TestFeedbackValidationErrors(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")
	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	createTutorStudent(t, tutorAuth, tutorID, studentID)
	acceptInvitation(t, studentAuth, tutorID)

	assignment := createAssignment(t, tutorAuth, tutorID, studentID, "Grade Validation")
	submission := createSubmission(t, studentAuth, assignment["id"].(string), "test")

	// Grade 0 is below the valid range 1-5
	resp := doRequest(t, "POST", "/homework/feedbacks", map[string]any{
		"submissionId": submission["id"],
		"grade":        0,
		"comment":      "grade too low",
	}, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusBadRequest)

	// Grade 6 is above the valid range 1-5
	resp = doRequest(t, "POST", "/homework/feedbacks", map[string]any{
		"submissionId": submission["id"],
		"grade":        6,
		"comment":      "grade too high",
	}, map[string]string{"Authorization": tutorAuth})
	assertStatus(t, resp, http.StatusBadRequest)
}
