package authorization

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"userservice/internal/errdefs"
)

const initDataTTL = 24 * time.Hour

func GetTelegramId(secret string, header string) (int64, error) {
	header = strings.TrimSpace(header)
	if strings.Contains(header, "auth_date=") && strings.Contains(header, "hash=") {
		return GetTelegramIDFromInitData(secret, header, initDataTTL)
	}

	payload := strings.Split(header, ":")
	if len(payload) != 3 {
		return 0, fmt.Errorf(
			"authorization: header payload len mismatch got %d: %w",
			len(payload), errdefs.ErrAuthentication,
		)
	}

	tgId, err := strconv.ParseInt(payload[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf(
			"authorization: cannot parse tgId %s: %w",
			payload[0], errdefs.ErrAuthentication,
		)
	}

	timestamp, err := strconv.ParseInt(payload[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf(
			"authorization: cannot parse timestamp %s: %w",
			payload[1], errdefs.ErrAuthentication,
		)
	}
	now := time.Now().Unix()
	var diffSeconds int64 = 5 * 60
	if now-diffSeconds >= timestamp || timestamp > now+diffSeconds {
		return 0, fmt.Errorf(
			"authorization: timestamp expired %s: %w",
			payload[1], errdefs.ErrAuthentication,
		)
	}

	message := fmt.Sprintf("%s:%s", payload[0], payload[1])
	if !ValidMAC(message, secret, payload[2]) {
		return 0, fmt.Errorf(
			"authorization: invalid hmac: %w",
			errdefs.ErrAuthentication,
		)
	}

	return tgId, nil
}

func GetTelegramIDFromInitData(botToken string, initData string, ttl time.Duration) (int64, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return 0, fmt.Errorf("authorization: invalid initData: %w", errdefs.ErrAuthentication)
	}

	actualHash := values.Get("hash")
	if actualHash == "" {
		return 0, fmt.Errorf("authorization: initData hash is empty: %w", errdefs.ErrAuthentication)
	}
	values.Del("hash")

	authDateRaw := values.Get("auth_date")
	authDate, err := strconv.ParseInt(authDateRaw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("authorization: invalid auth_date %s: %w", authDateRaw, errdefs.ErrAuthentication)
	}
	if ttl > 0 {
		now := time.Now()
		authTime := time.Unix(authDate, 0)
		if authTime.After(now.Add(5*time.Minute)) || now.Sub(authTime) > ttl {
			return 0, fmt.Errorf("authorization: initData expired: %w", errdefs.ErrAuthentication)
		}
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	checkParts := make([]string, 0, len(keys))
	for _, key := range keys {
		checkParts = append(checkParts, key+"="+values.Get(key))
	}
	checkString := strings.Join(checkParts, "\n")

	secretKeyMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretKeyMAC.Write([]byte(botToken))
	secretKey := secretKeyMAC.Sum(nil)

	if !validMACBytes(checkString, secretKey, actualHash) {
		return 0, fmt.Errorf("authorization: invalid initData hmac: %w", errdefs.ErrAuthentication)
	}

	var tgUser struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(values.Get("user")), &tgUser); err != nil || tgUser.ID == 0 {
		return 0, fmt.Errorf("authorization: invalid initData user: %w", errdefs.ErrAuthentication)
	}

	return tgUser.ID, nil
}

func ValidMAC(message, key, messageMAC string) bool {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(message))
	expectedMAC := mac.Sum(nil)
	expectedHex := hex.EncodeToString(expectedMAC)
	return hmac.Equal([]byte(messageMAC), []byte(expectedHex))
}

func validMACBytes(message string, key []byte, messageMAC string) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	expectedMAC := mac.Sum(nil)
	expectedHex := hex.EncodeToString(expectedMAC)
	return hmac.Equal([]byte(messageMAC), []byte(expectedHex))
}
