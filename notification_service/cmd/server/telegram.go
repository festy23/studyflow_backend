package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// errRetriableSend means the caller should NOT commit the Kafka offset.
// Examples: HTTP 429 from Telegram, network errors. The same message will be
// re-fetched on the next consumer iteration.
var errRetriableSend = errors.New("retriable telegram send failure")

// telegramSender abstracts the Bot API so the consumer loop is testable.
type telegramSender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// telegramClient calls https://api.telegram.org/bot<token>/sendMessage.
// Minimal implementation — only the fields we need.
type telegramClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func newTelegramClient(token string) *telegramClient {
	return &telegramClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    "https://api.telegram.org",
		token:      token,
	}
}

// SendMessage delivers a plain-text Telegram message.
// Returns errRetriableSend for 429 / 5xx / network errors so the caller can
// avoid committing the Kafka offset; other errors (e.g. 400 chat_not_found)
// are returned as nil from the *delivery* perspective — they are terminal
// for this message and we don't want to block the partition.
func (c *telegramClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	body, err := json.Marshal(map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	endpoint := c.baseURL + "/bot" + url.PathEscape(c.token) + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", errRetriableSend, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	// 429 and 5xx are transient.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return fmt.Errorf("%w: telegram status %d", errRetriableSend, resp.StatusCode)
	}

	// 4xx other than 429 — terminal (bad chat_id, bot blocked, etc.).
	// Read a bit of body for the log line but treat as non-retriable.
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("telegram terminal status %d: %s", resp.StatusCode, string(respBody))
}
