package e2e

import (
	"net/http"
	"testing"
)

// ---- health & public (no auth) ----

func TestHealth(t *testing.T) {
	resp := doRequest(t, "GET", "/health", nil, nil)
	assertStatus(t, resp, http.StatusOK)
	body := readJSON(t, resp)
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", body)
	}
}

func TestCORSPreflight(t *testing.T) {
	resp := doRequest(t, "OPTIONS", "/health", nil, map[string]string{
		"Origin":                         "https://web.telegram.org",
		"Access-Control-Request-Method":  "GET",
		"Access-Control-Request-Headers": "Authorization",
	})
	assertStatus(t, resp, http.StatusNoContent)
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing Access-Control-Allow-Origin")
	}
	if resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Error("missing Access-Control-Allow-Methods")
	}
}

func TestListFAQs(t *testing.T) {
	resp := doRequest(t, "GET", "/faqs", nil, nil)
	assertStatus(t, resp, http.StatusOK)
	// Verify it's valid JSON array or object
	body := readBody(t, resp)
	if len(body) == 0 {
		t.Error("empty FAQ response")
	}
}

// ---- auth ----

func TestNoAuthHeader(t *testing.T) {
	resp := doRequest(t, "GET", "/users/users/me", nil, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestInvalidToken(t *testing.T) {
	resp := doRequest(t, "GET", "/users/users/me", nil, map[string]string{
		"Authorization": "telegram 1:1:badhmac",
	})
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestInvalidInitData(t *testing.T) {
	resp := doRequest(t, "GET", "/users/users/me", nil, map[string]string{
		"Authorization": "tma auth_date=1&hash=bad&user=%7B%22id%22%3A1%7D",
	})
	assertStatus(t, resp, http.StatusUnauthorized)
}

// ---- sign-up + get-me ----

func TestSignUpAndGetMe(t *testing.T) {
	_, auth, user := signUp(t, "student")

	if user["id"] == nil || user["id"] == "" {
		t.Fatal("signup returned no user id")
	}
	id := user["id"].(string)

	// Get own profile using the same auth token
	resp := doRequest(t, "GET", "/users/users/me", nil, map[string]string{
		"Authorization": auth,
	})
	assertStatus(t, resp, http.StatusOK)
	me := readJSON(t, resp)
	if me["id"] != id {
		t.Errorf("GetMe returned id=%v, want %s", me["id"], id)
	}
	if me["role"] != "student" {
		t.Errorf("GetMe returned role=%v, want student", me["role"])
	}
}

// ---- error cases ----

func TestNotFound(t *testing.T) {
	_, auth, _ := signUp(t, "tutor")
	resp := doRequest(t, "GET", "/users/users/00000000-0000-0000-0000-000000000000", nil, map[string]string{
		"Authorization": auth,
	})
	// Should be 404 — user not found
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
		body := readBody(t, resp)
		t.Errorf("expected 404 or 403, got %d: %s", resp.StatusCode, body)
	}
}

// ---- tutor-student flow ----

func TestTutorStudentFlow(t *testing.T) {
	tutorTg, tutorAuth, tutor := signUp(t, "tutor")
	studentTg, studentAuth, student := signUp(t, "student")

	_ = tutorTg
	_ = studentTg

	tutorID := tutor["id"].(string)
	studentID := student["id"].(string)

	// Tutor creates relationship with student
	resp := doRequest(t, "POST", "/users/tutor-students", map[string]any{
		"tutor_id":   tutorID,
		"student_id": studentID,
	}, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	ts := readJSON(t, resp)
	if ts["id"] == nil {
		t.Fatal("tutor-student creation returned no id")
	}
	tsID := ts["id"].(string)

	// Tutor fetches their profile
	resp = doRequest(t, "GET", "/users/tutor-profiles/"+tutorID, nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)

	// Student fetches tutor profile (should also work)
	resp = doRequest(t, "GET", "/users/tutor-profiles/"+tutorID, nil, map[string]string{
		"Authorization": studentAuth,
	})
	assertStatus(t, resp, http.StatusOK)

	_ = tsID
}

// ---- dual sign-up (tutor then student) ----

func TestSameUserBothRoles(t *testing.T) {
	// A user signs up as tutor, then signs up again as student
	// The second sign-up should return already exists or the same user
	tgID := newTelegramID()
	auth := authHeader(t, tgID)

	resp := doRequest(t, "POST", "/users/sign-up/telegram", map[string]any{
		"telegram_id": tgID,
		"role":        "tutor",
		"first_name":  "DualRole",
	}, map[string]string{
		"Authorization": auth,
	})
	assertStatus(t, resp, http.StatusOK)

	resp2 := doRequest(t, "POST", "/users/sign-up/telegram", map[string]any{
		"telegram_id": tgID,
		"role":        "student",
		"first_name":  "DualRole",
	}, map[string]string{
		"Authorization": auth,
	})
	// Should return AlreadyExists or OK
	code := resp2.StatusCode
	if code != http.StatusOK && code != http.StatusConflict {
		body := readBody(t, resp)
		t.Errorf("second signup: expected 200 or 409, got %d: %s", code, body)
	}
}
