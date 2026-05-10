package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type sensitiveReq struct {
	UserPhone string
	AuthToken string
}

func newTestLogger() (*Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(&buf), zapcore.DebugLevel)
	return New(zap.New(core)), &buf
}

func TestUnaryLoggingInterceptor_DoesNotLogRequestBody(t *testing.T) {
	logger, buf := newTestLogger()
	interceptor := NewUnaryLoggingInterceptor(logger)

	md := metadata.New(map[string]string{
		"x-user-id":    "user-secret-123",
		"x-user-role":  "tutor",
		"authorization": "Bearer super-secret-token",
		"x-trace-id":   "trace-abc",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	req := &sensitiveReq{UserPhone: "+15551234567", AuthToken: "tok_sensitive_xyz"}
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}

	_, err := interceptor(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()

	forbidden := []string{
		"+15551234567",
		"tok_sensitive_xyz",
		"user-secret-123",
		"super-secret-token",
		"x-user-id",
		"x-user-role",
		"authorization",
		"\"request\"",
		"\"metadata\"",
	}
	for _, s := range forbidden {
		if strings.Contains(out, s) {
			t.Errorf("log output must not contain %q; got: %s", s, out)
		}
	}

	// Trace id should be present.
	if !strings.Contains(out, "trace-abc") {
		t.Errorf("expected trace id in log output, got: %s", out)
	}
	if !strings.Contains(out, "/svc/Method") {
		t.Errorf("expected method in log output, got: %s", out)
	}

	// Each line must be valid JSON (sanity).
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line not valid JSON: %s err=%v", line, err)
		}
	}
}
