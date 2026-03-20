package authorization

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
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

func makeInitData(t *testing.T, botToken string, fields map[string]string) string {
	t.Helper()
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	checkParts := make([]string, 0, len(keys))
	values := url.Values{}
	for _, key := range keys {
		checkParts = append(checkParts, key+"="+fields[key])
		values.Set(key, fields[key])
	}

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretMAC.Write([]byte(botToken))
	secret := secretMAC.Sum(nil)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(strings.Join(checkParts, "\n")))
	values.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return values.Encode()
}

func TestGetTelegramIDFromInitData(t *testing.T) {
	const botToken = "123456:ABC-DEF"
	const tgID int64 = 987654321

	initData := makeInitData(t, botToken, map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"query_id":  "AAHdF6IQAAAAAN0XohDhrOrc",
		"user":      `{"id":987654321,"first_name":"Ivan","username":"ivan"}`,
	})

	got, err := GetTelegramIDFromInitData(botToken, initData, initDataTTL)
	if err != nil {
		t.Fatalf("expected ok, got err: %v", err)
	}
	if got != tgID {
		t.Fatalf("got %d, want %d", got, tgID)
	}
}

func TestGetTelegramIDFromInitData_RejectsExpired(t *testing.T) {
	const botToken = "123456:ABC-DEF"
	initData := makeInitData(t, botToken, map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Add(-25*time.Hour).Unix(), 10),
		"user":      `{"id":987654321}`,
	})

	_, err := GetTelegramIDFromInitData(botToken, initData, initDataTTL)
	if err == nil || !errors.Is(err, errdefs.ErrAuthentication) {
		t.Fatalf("expected ErrAuthentication, got %v", err)
	}
}

func TestGetTelegramIDFromInitData_RejectsInvalidHash(t *testing.T) {
	const botToken = "123456:ABC-DEF"
	initData := makeInitData(t, botToken, map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"user":      `{"id":987654321}`,
	})
	initData = strings.Replace(initData, "987654321", "987654322", 1)

	_, err := GetTelegramIDFromInitData(botToken, initData, initDataTTL)
	if err == nil || !errors.Is(err, errdefs.ErrAuthentication) {
		t.Fatalf("expected ErrAuthentication, got %v", err)
	}
}
