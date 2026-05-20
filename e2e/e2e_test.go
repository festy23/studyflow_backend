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

// ---- delete & reactivate ----

func TestDeleteAndReactivate(t *testing.T) {
	// 1. Sign up
	tgID, auth, user := signUp(t, "student")
	userID := user["id"].(string)

	// 2. Delete
	resp := doRequest(t, "DELETE", "/users/users/"+userID, nil, map[string]string{
		"Authorization": auth,
	})
	assertStatus(t, resp, http.StatusOK)

	// 3. GET /users/users/me should now return 404
	resp = doRequest(t, "GET", "/users/users/me", nil, map[string]string{
		"Authorization": auth,
	})
	assertStatus(t, resp, http.StatusNotFound)

	// 4. Re-register with same telegram_id — should reactivate
	resp = doRequest(t, "POST", "/users/sign-up/telegram", map[string]any{
		"telegram_id": tgID,
		"role":        "student",
		"first_name":  "Reactivated",
	}, map[string]string{
		"Authorization": auth,
	})
	assertStatus(t, resp, http.StatusOK)
	user2 := readJSON(t, resp)
	if user2["status"] != "active" {
		t.Errorf("expected status=active, got %v", user2["status"])
	}

	// 5. GET /users/users/me should work again
	resp = doRequest(t, "GET", "/users/users/me", nil, map[string]string{
		"Authorization": auth,
	})
	assertStatus(t, resp, http.StatusOK)
	me := readJSON(t, resp)
	if me["status"] != "active" {
		t.Errorf("expected status=active, got %v", me["status"])
	}
}

func TestStudyflowStatus(t *testing.T) {
	resp := doRequest(t, "GET", "/studyflow/status", nil, nil)
	assertStatus(t, resp, http.StatusOK)
	body := readJSON(t, resp)
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", body)
	}
}

// ---- invitations ----

func TestInviteFlow(t *testing.T) {
	// 1. Tutor creates invite
	_, tutorAuth, _ := signUp(t, "tutor")
	resp := doRequest(t, "POST", "/users/users/invitations", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	invite := readJSON(t, resp)
	token := invite["token"].(string)
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// 2. Tutor lists invitations
	resp = doRequest(t, "GET", "/users/users/invitations", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)

	// 3. Student accepts
	_, studentAuth, _ := signUp(t, "student")
	resp = doRequest(t, "POST", "/users/users/invitations/"+token+"/accept", nil, map[string]string{
		"Authorization": studentAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	ts := readJSON(t, resp)
	if ts["status"] != "active" {
		t.Fatalf("expected status=active after accept, got %v", ts["status"])
	}

	// 4. Re-accept should fail (token already used)
	resp = doRequest(t, "POST", "/users/users/invitations/"+token+"/accept", nil, map[string]string{
		"Authorization": studentAuth,
	})
	assertStatus(t, resp, http.StatusNotFound)

	// 5. Tutor revokes a new invite
	resp = doRequest(t, "POST", "/users/users/invitations", nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)
	invite2 := readJSON(t, resp)
	inviteID := invite2["id"].(string)

	resp = doRequest(t, "DELETE", "/users/users/invitations/"+inviteID, nil, map[string]string{
		"Authorization": tutorAuth,
	})
	assertStatus(t, resp, http.StatusOK)

	// Accepting revoked invite should fail
	resp = doRequest(t, "POST", "/users/users/invitations/"+invite2["token"].(string)+"/accept", nil, map[string]string{
		"Authorization": studentAuth,
	})
	assertStatus(t, resp, http.StatusNotFound)
}
