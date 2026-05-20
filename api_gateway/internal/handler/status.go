package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"common_library/logging"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userpb "userservice/pkg/api"
)

const healthCheckTimeout = 3 * time.Second

type StatusHandler struct {
	userClient userpb.UserServiceClient
	redisConn  *redis.Client
}

func NewStatusHandler(userClient userpb.UserServiceClient, redisConn *redis.Client) *StatusHandler {
	return &StatusHandler{userClient: userClient, redisConn: redisConn}
}

func (h *StatusHandler) RegisterRoutes(r chi.Router) {
	r.Get("/status", h.Status)
}

func (h *StatusHandler) Status(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{
		"user_service": "ok",
		"redis":        "ok",
	}
	overall := "ok"

	// Check user_service gRPC connectivity
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()
	_, err := h.userClient.AuthorizeByAuthHeader(ctx, &userpb.AuthorizeByAuthHeaderRequest{
		AuthorizationHeader: "",
	})
	if err != nil {
		// Only connection-level failures indicate the service is unhealthy.
		// Application-level errors (e.g., Unauthenticated for empty header)
		// mean the service is alive and responding.
		unhealthy := true
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.Unauthenticated, codes.InvalidArgument, codes.PermissionDenied:
				// service is responding, just with an expected rejection
				unhealthy = false
			}
		}
		if unhealthy {
			// Keep internal details in logs only; expose a sanitized status.
			if logger, ok := logging.GetFromContext(r.Context()); ok {
				logger.Error(r.Context(), "health check: user_service unavailable", zap.Error(err))
			}
			checks["user_service"] = "unavailable"
			overall = "degraded"
		}
	}

	// Check Redis connectivity
	if h.redisConn == nil {
		if logger, ok := logging.GetFromContext(r.Context()); ok {
			logger.Error(r.Context(), "health check: redis client is not configured")
		}
		checks["redis"] = "unavailable"
		overall = "degraded"
	} else {
		ctx2, cancel2 := context.WithTimeout(r.Context(), healthCheckTimeout)
		defer cancel2()
		if err := h.redisConn.Ping(ctx2).Err(); err != nil {
			if logger, ok := logging.GetFromContext(r.Context()); ok {
				logger.Error(r.Context(), "health check: redis unavailable", zap.Error(err))
			}
			checks["redis"] = "unavailable"
			overall = "degraded"
		}
	}

	// If all checks failed, mark as down
	allDown := true
	for _, v := range checks {
		if v == "ok" {
			allDown = false
			break
		}
	}
	if allDown {
		overall = "down"
	}

	resp := map[string]any{
		"status": overall,
		"checks": checks,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp) //nolint:errchkjson // diagnostic endpoint
}
