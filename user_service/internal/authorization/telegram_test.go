package authorization

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"
	"userservice/internal/errdefs"
)

func sign(t *testing.T, secret, message string) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func makeHeader(t *testing.T, secret string, tgId int64, ts int64) string {
	t.Helper()
	message := fmt.Sprintf("%d:%d", tgId, ts)
	return fmt.Sprintf("%s:%s", message, sign(t, secret, message))
}

func TestGetTelegramId_TimestampBoundary(t *testing.T) {
	const secret = "shh"
	const tgId int64 = 42
	now := time.Now().Unix()

	t.Run("exactly at upper boundary now+5min is accepted", func(t *testing.T) {
		ts := now + 5*60
		header := makeHeader(t, secret, tgId, ts)
		got, err := GetTelegramId(secret, header)
		if err != nil {
			t.Fatalf("expected ok, got err: %v", err)
		}
		if got != tgId {
			t.Fatalf("got %d, want %d", got, tgId)
		}
	})

	t.Run("just past upper boundary now+5min+1 is rejected", func(t *testing.T) {
		ts := now + 5*60 + 1
		header := makeHeader(t, secret, tgId, ts)
		_, err := GetTelegramId(secret, header)
		if err == nil || !errors.Is(err, errdefs.ErrAuthentication) {
			t.Fatalf("expected ErrAuthentication, got %v", err)
		}
	})

	t.Run("too old (now-5min) is rejected", func(t *testing.T) {
		ts := now - 5*60
		header := makeHeader(t, secret, tgId, ts)
		_, err := GetTelegramId(secret, header)
		if err == nil || !errors.Is(err, errdefs.ErrAuthentication) {
			t.Fatalf("expected ErrAuthentication, got %v", err)
		}
	})
}
