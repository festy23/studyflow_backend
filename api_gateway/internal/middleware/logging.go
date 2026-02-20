package middleware

import (
	"common_library/logging"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func NewLoggingMiddleware(logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			traceID, err := uuid.NewV7()
			if err != nil {
				traceID = uuid.New()
			}

			r.Header.Set("X-Trace-Id", traceID.String())

			ctx := logging.ContextWithLogger(r.Context(), logger)
			r = r.WithContext(ctx)

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			w.Header().Set("X-Trace-Id", traceID.String())

			next.ServeHTTP(sw, r)

			logger.Info(ctx, "request completed",
				zap.String("trace_id", traceID.String()),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", sw.status),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}
