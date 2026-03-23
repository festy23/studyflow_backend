package middleware

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	auditpb "audit_service/pkg/api"
	"go.uber.org/zap"
)

type auditResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// NewAuditMiddleware returns a chi-compatible middleware that records mutation
// requests (POST, PATCH, PUT, DELETE) to the audit service in a fire-and-forget
// fashion when the response status is 2xx.
func NewAuditMiddleware(client auditpb.AuditServiceClient, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only record mutation requests.
			if r.Method != http.MethodPost &&
				r.Method != http.MethodPatch &&
				r.Method != http.MethodPut &&
				r.Method != http.MethodDelete {
				next.ServeHTTP(w, r)
				return
			}

			// Must have a user identity.
			userID := r.Header.Get("X-User-Id")
			if userID == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Read request body (restore for downstream handlers).
			var bodyBytes []byte
			if r.Body != nil {
				var err error
				bodyBytes, err = io.ReadAll(r.Body)
				if err != nil {
					bodyBytes = nil
				}
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			arw := &auditResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(arw, r)

			// Only record successful mutations.
			if arw.status < 200 || arw.status >= 300 {
				return
			}

			fireAndForget(client, logger, userID, r, bodyBytes)
		})
	}
}

func fireAndForget(client auditpb.AuditServiceClient, logger *zap.Logger, userID string, r *http.Request, body []byte) {
	resourceType, resourceID := parseResourceFromPath(r.URL.Path)
	action := buildActionName(r.Method, resourceType)

	ip := extractIP(r.RemoteAddr)
	req := &auditpb.RecordEventRequest{
		UserId:       userID,
		Action:       action,
		ResourceType: resourceType,
		IpAddress:    &ip,
	}
	if resourceID != "" {
		req.ResourceId = &resourceID
	}
	if len(body) > 0 {
		details := string(body)
		// Cap body size to 4 KiB to avoid storing oversized payloads.
		if len(details) > 4096 {
			details = details[:4096]
		}
		req.Details = &details
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := client.RecordEvent(ctx, req); err != nil {
			logger.Warn("audit: failed to record event",
				zap.String("user_id", userID),
				zap.String("action", action),
				zap.Error(err),
			)
		}
	}()
}

// parseResourceFromPath extracts a resource type and optional resource ID from
// a URL path.  It looks for the first UUID-like segment and treats the
// preceding segment as the resource type.
func parseResourceFromPath(path string) (resourceType, resourceID string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if looksLikeUUID(part) && i > 0 {
			return singularize(parts[i-1]), part
		}
	}
	// No ID found — use the last meaningful segment as resource type.
	for i := len(parts) - 1; i >= 0; i-- {
		if p := parts[i]; p != "" && !strings.HasPrefix(p, "{") {
			return singularize(p), ""
		}
	}
	return "", ""
}

// buildActionName returns a dotted action string such as "lesson.created" or
// "receipt.verified".
func buildActionName(method, resourceType string) string {
	verb := map[string]string{
		http.MethodPost:   "created",
		http.MethodPut:    "updated",
		http.MethodPatch:  "updated",
		http.MethodDelete: "deleted",
	}[method]
	if verb == "" {
		verb = "modified"
	}
	return resourceType + "." + verb
}

// looksLikeUUID returns true when s looks like a standard UUID hex string.
func looksLikeUUID(s string) bool {
	return len(s) == 36 && strings.Count(s, "-") == 4
}

// singularize strips a trailing "s" from a lowercase word as a best-effort
// heuristic.
func singularize(s string) string {
	// -ies => -y (e.g. categories => category)
	if strings.HasSuffix(s, "ies") && len(s) > 3 {
		return strings.TrimSuffix(s, "ies") + "y"
	}
	// -ses => -s (e.g. statuses => status)
	if strings.HasSuffix(s, "ses") && len(s) > 3 {
		return strings.TrimSuffix(s, "es")
	}
	// -es => (e.g. boxes => box)
	if strings.HasSuffix(s, "es") && len(s) > 3 && s[len(s)-3] != 's' {
		return strings.TrimSuffix(s, "es")
	}
	// -s => (e.g. faqs => faq, receipts => receipt)
	if strings.HasSuffix(s, "s") && len(s) > 1 && !strings.HasSuffix(s, "ss") {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// extractIP strips the port from a RemoteAddr string.
func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
