package middleware

import (
	"common_library/logging"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// BUG-001: external clients must not be able to forge X-User-Id / X-User-Role / X-Trace-Id.
// The logging middleware must strip these headers before any handler sees them.
func TestLoggingMiddleware_StripsForgedHeaders(t *testing.T) {
	logger := logging.New(zap.NewNop())
	mw := NewLoggingMiddleware(logger)

	var seenUserID, seenUserRole, seenTraceID string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seenUserID = r.Header.Get("X-User-Id")
		seenUserRole = r.Header.Get("X-User-Role")
		seenTraceID = r.Header.Get("X-Trace-Id")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.Header.Set("X-User-Id", "attacker")
	r.Header.Set("X-User-Role", "admin")
	r.Header.Set("X-Trace-Id", "forged-trace")

	mw(next).ServeHTTP(w, r)

	assert.Empty(t, seenUserID, "X-User-Id must be stripped")
	assert.Empty(t, seenUserRole, "X-User-Role must be stripped")
	// Trace-Id is regenerated, so it must not equal the forged value.
	assert.NotEqual(t, "forged-trace", seenTraceID, "X-Trace-Id must be regenerated, not trusted from client")
	assert.NotEmpty(t, seenTraceID, "X-Trace-Id should be regenerated")
}
