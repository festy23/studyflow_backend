package middleware

import (
	"common_library/logging"
	"context"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"time"
	userpb "userservice/pkg/api"
)

const authRequestTimeout = 3 * time.Second

func NewAuthMiddleware(userClient userpb.UserServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			header := r.Header.Get("Authorization")
			if header == "" {
				if logger, ok := logging.GetFromContext(ctx); ok {
					logger.Info(ctx, "no authorization header", zap.String("path", r.URL.Path))
				}
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			req := &userpb.AuthorizeByAuthHeaderRequest{AuthorizationHeader: header}
			authCtx, cancel := context.WithTimeout(ctx, authRequestTimeout)
			defer cancel()
			resp, err := userClient.AuthorizeByAuthHeader(authCtx, req)
			if err != nil {
				if status.Code(err) == codes.PermissionDenied {
					if logger, ok := logging.GetFromContext(ctx); ok {
						logger.Info(ctx, "permission denied", zap.String("path", r.URL.Path))
					}
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if logger, ok := logging.GetFromContext(ctx); ok {
					logger.Error(
						ctx, "error while sending grpc auth request",
						zap.String("path", r.URL.Path),
						zap.String("method", r.Method),
						zap.Error(err),
					)
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			r.Header.Set("X-User-Id", resp.Id)
			r.Header.Set("X-User-Role", resp.Role)
			next.ServeHTTP(w, r)
		})
	}
}
