package e2e

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

const baseURL = "http://localhost:80"

func apiURL(path string) string { return baseURL + path }

func telegramSecret(t *testing.T) string {
	t.Helper()
	s := os.Getenv("TELEGRAM_SECRET")
	if s == "" {
		s = "123456:replace-with-telegram-bot-token"
	}
	return s
}

func generateHMACHeader(t *testing.T, tgID int64) string {
	t.Helper()
	now := time.Now().Unix()
	message := fmt.Sprintf("%d:%d", tgID, now)
	mac := hmac.New(sha256.New, []byte(telegramSecret(t)))
	mac.Write([]byte(message))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("telegram %s:%s", message, sig)
}

func authHeader(t *testing.T, tgID int64) string {
	t.Helper()
	return generateHMACHeader(t, tgID)
}

type httpDoer func(method, path string, body any) *http.Response

func doRequest(t *testing.T, method, path string, body any, headers map[string]string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, apiURL(path), bodyReader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Errorf("GET %s → %d (want %d), body: %s", resp.Request.URL.Path, resp.StatusCode, want, string(body))
	}
}

func readJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return m
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func newTelegramID() int64 { return time.Now().UnixNano() % 10000000000 }

func signUp(t *testing.T, role string) (tgID int64, auth string, user map[string]any) {
	t.Helper()
	tgID = newTelegramID()
	auth = authHeader(t, tgID)

	resp := doRequest(t, "POST", "/users/sign-up/telegram", map[string]any{
		"telegram_id": tgID,
		"role":        role,
		"first_name":  "Test" + role,
		"username":    "test_" + strconv.FormatInt(tgID, 10),
	}, map[string]string{
		"Authorization": auth,
	})
	assertStatus(t, resp, 200)
	user = readJSON(t, resp)
	return tgID, auth, user
}
