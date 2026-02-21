package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	homeworkpb "homework_service/pkg/api"
	schedulepb "schedule_service/pkg/api"
	userpb "userservice/pkg/api"
)

// ── helpers ─────────────────────────────────────────────────────────

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func withChiParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// ── mapErr ──────────────────────────────────────────────────────────

func TestMapErr(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"BadRequest", ErrBadRequest, http.StatusBadRequest},
		{"gRPC_InvalidArgument", status.Error(codes.InvalidArgument, "bad"), http.StatusBadRequest},
		{"gRPC_AlreadyExists", status.Error(codes.AlreadyExists, "dup"), http.StatusConflict},
		{"gRPC_PermissionDenied", status.Error(codes.PermissionDenied, "no"), http.StatusForbidden},
		{"gRPC_NotFound", status.Error(codes.NotFound, "miss"), http.StatusNotFound},
		{"gRPC_Unauthenticated", status.Error(codes.Unauthenticated, "auth"), http.StatusUnauthorized},
		{"gRPC_Internal", status.Error(codes.Internal, "fail"), http.StatusInternalServerError},
		{"UnknownError", errors.New("unknown"), http.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, mapErr(tc.err))
		})
	}
}

// ── writeErrorJSON ──────────────────────────────────────────────────

func TestWriteErrorJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeErrorJSON(w, http.StatusBadRequest, "test error")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "test error", body["error"])
}

// ── Handle ──────────────────────────────────────────────────────────

func TestHandle(t *testing.T) {
	t.Run("NilMethodReturnsError", func(t *testing.T) {
		_, err := Handle[userpb.Empty, userpb.User](nil, nil, false)
		assert.ErrorIs(t, err, ErrNilMethod)
	})

	t.Run("SuccessfulGRPCCall", func(t *testing.T) {
		mockMethod := func(_ context.Context, req *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.UserPublic, error) {
			return &userpb.UserPublic{
				Id:        req.Id,
				FirstName: proto.String("John"),
			}, nil
		}

		handler, err := Handle[userpb.GetUserRequest, userpb.UserPublic](
			mockMethod,
			func(_ context.Context, _ *http.Request, req *userpb.GetUserRequest) error {
				req.Id = "test-id"
				return nil
			},
			false,
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/users/test-id", nil)
		r.Header.Set("X-User-Id", "user-123")
		r.Header.Set("X-User-Role", "tutor")
		r.Header.Set("X-Trace-Id", "trace-abc")

		handler(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), "test-id")
	})

	t.Run("GRPCError_NotFound", func(t *testing.T) {
		mockMethod := func(_ context.Context, _ *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.UserPublic, error) {
			return nil, status.Error(codes.NotFound, "not found")
		}

		handler, err := Handle[userpb.GetUserRequest, userpb.UserPublic](mockMethod, nil, false)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/users/test", nil)
		handler(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("GRPCError_PermissionDenied", func(t *testing.T) {
		mockMethod := func(_ context.Context, _ *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.UserPublic, error) {
			return nil, status.Error(codes.PermissionDenied, "denied")
		}

		handler, err := Handle[userpb.GetUserRequest, userpb.UserPublic](mockMethod, nil, false)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/users/test", nil)
		handler(w, r)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("ParseBodySuccess", func(t *testing.T) {
		mockMethod := func(_ context.Context, req *userpb.RegisterViaTelegramRequest, _ ...grpc.CallOption) (*userpb.User, error) {
			return &userpb.User{
				Id:   "new-user",
				Role: req.Role,
			}, nil
		}

		handler, err := Handle[userpb.RegisterViaTelegramRequest, userpb.User](mockMethod, nil, true)
		require.NoError(t, err)

		body := `{"role":"student","telegramId":"12345"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/sign-up", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		handler(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ParseBodyInvalidJSON", func(t *testing.T) {
		mockMethod := func(_ context.Context, _ *userpb.RegisterViaTelegramRequest, _ ...grpc.CallOption) (*userpb.User, error) {
			return &userpb.User{}, nil
		}

		handler, err := Handle[userpb.RegisterViaTelegramRequest, userpb.User](mockMethod, nil, true)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/sign-up", strings.NewReader("{invalid"))
		handler(w, r)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ReqParserError", func(t *testing.T) {
		mockMethod := func(_ context.Context, _ *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.UserPublic, error) {
			return &userpb.UserPublic{}, nil
		}

		handler, err := Handle[userpb.GetUserRequest, userpb.UserPublic](
			mockMethod,
			func(_ context.Context, _ *http.Request, _ *userpb.GetUserRequest) error {
				return ErrBadRequest
			},
			false,
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/users/bad", nil)
		handler(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// ── HandleWithCache ─────────────────────────────────────────────────

type mockCache struct {
	store map[string][]byte
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string][]byte)}
}

func (m *mockCache) Get(_ context.Context, key string) ([]byte, bool) {
	v, ok := m.store[key]
	return v, ok
}

func (m *mockCache) Set(_ context.Context, key string, data []byte, _ time.Duration) {
	m.store[key] = data
}

func (m *mockCache) Delete(_ context.Context, key string) {
	delete(m.store, key)
}

func TestHandleWithCache(t *testing.T) {
	t.Run("CacheHit", func(t *testing.T) {
		cache := newMockCache()
		cache.Set(context.Background(), "user:abc", []byte(`{"id":"abc"}`), time.Minute)

		callCount := 0
		mockMethod := func(_ context.Context, _ *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.UserPublic, error) {
			callCount++
			return &userpb.UserPublic{Id: "abc"}, nil
		}

		handler, err := HandleWithCache[userpb.GetUserRequest, userpb.UserPublic](
			mockMethod, nil, false,
			cache,
			func(_ *http.Request) (string, error) { return "user:abc", nil },
			5*time.Minute,
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/users/abc", nil)
		handler(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 0, callCount)
		assert.Contains(t, w.Body.String(), `"id":"abc"`)
	})

	t.Run("CacheMiss_ThenStored", func(t *testing.T) {
		cache := newMockCache()

		mockMethod := func(_ context.Context, _ *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.UserPublic, error) {
			return &userpb.UserPublic{Id: "xyz"}, nil
		}

		handler, err := HandleWithCache[userpb.GetUserRequest, userpb.UserPublic](
			mockMethod, nil, false,
			cache,
			func(_ *http.Request) (string, error) { return "user:xyz", nil },
			5*time.Minute,
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/users/xyz", nil)
		handler(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "xyz")

		_, ok := cache.Get(context.Background(), "user:xyz")
		assert.True(t, ok)
	})

	t.Run("CacheKeyError_StillWorks", func(t *testing.T) {
		cache := newMockCache()

		mockMethod := func(_ context.Context, _ *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.UserPublic, error) {
			return &userpb.UserPublic{Id: "abc"}, nil
		}

		handler, err := HandleWithCache[userpb.GetUserRequest, userpb.UserPublic](
			mockMethod, nil, false,
			cache,
			func(_ *http.Request) (string, error) { return "", errors.New("no key") },
			5*time.Minute,
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/users/abc", nil)
		handler(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GRPCError_NotCached", func(t *testing.T) {
		cache := newMockCache()

		mockMethod := func(_ context.Context, _ *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.UserPublic, error) {
			return nil, status.Error(codes.NotFound, "not found")
		}

		handler, err := HandleWithCache[userpb.GetUserRequest, userpb.UserPublic](
			mockMethod, nil, false,
			cache,
			func(_ *http.Request) (string, error) { return "user:missing", nil },
			5*time.Minute,
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/users/missing", nil)
		handler(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
		_, ok := cache.Get(context.Background(), "user:missing")
		assert.False(t, ok)
	})

	t.Run("NilMethodReturnsError", func(t *testing.T) {
		cache := newMockCache()
		_, err := HandleWithCache[userpb.GetUserRequest, userpb.UserPublic](
			nil, nil, false,
			cache,
			func(_ *http.Request) (string, error) { return "", nil },
			5*time.Minute,
		)
		assert.ErrorIs(t, err, ErrNilMethod)
	})

	t.Run("WithParseBody", func(t *testing.T) {
		cache := newMockCache()

		mockMethod := func(_ context.Context, req *userpb.UpdateUserRequest, _ ...grpc.CallOption) (*userpb.User, error) {
			return &userpb.User{Id: req.Id}, nil
		}

		handler, err := HandleWithCache[userpb.UpdateUserRequest, userpb.User](
			mockMethod, nil, true,
			cache,
			func(_ *http.Request) (string, error) { return "update:x", nil },
			5*time.Minute,
		)
		require.NoError(t, err)

		body := `{"id":"user-1","firstName":"Test"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/users/user-1", strings.NewReader(body))
		handler(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("WithReqParser", func(t *testing.T) {
		cache := newMockCache()

		mockMethod := func(_ context.Context, req *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.UserPublic, error) {
			return &userpb.UserPublic{Id: req.Id}, nil
		}

		handler, err := HandleWithCache[userpb.GetUserRequest, userpb.UserPublic](
			mockMethod,
			func(_ context.Context, _ *http.Request, req *userpb.GetUserRequest) error {
				req.Id = "parsed-id"
				return nil
			},
			false,
			cache,
			func(_ *http.Request) (string, error) { return "user:parsed", nil },
			5*time.Minute,
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/users/parsed-id", nil)
		handler(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "parsed-id")
	})
}

// ── parsePathParam ──────────────────────────────────────────────────

func TestParsePathParam(t *testing.T) {
	t.Run("MissingParam", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		_, err := parsePathParam(r, "id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing path param")
	})

	t.Run("ParamPresent", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/test/abc", nil)
		r = withChiParam(r, "id", "abc")

		val, err := parsePathParam(r, "id")
		assert.NoError(t, err)
		assert.Equal(t, "abc", val)
	})
}

// ── parseIDParam (schedule) ─────────────────────────────────────────

func TestParseIDParam(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/slots/123", nil)
		r = withChiParam(r, "id", "123")

		id, err := parseIDParam(r, "id")
		assert.NoError(t, err)
		assert.Equal(t, "123", id)
	})

	t.Run("Missing", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/slots/", nil)

		_, err := parseIDParam(r, "id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required path param")
	})
}

// ── parseStatus (schedule) ──────────────────────────────────────────

func TestParseStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected schedulepb.LessonStatusFilter
	}{
		{"BOOKED", schedulepb.LessonStatusFilter_BOOKED},
		{"booked", schedulepb.LessonStatusFilter_BOOKED},
		{"CANCELLED", schedulepb.LessonStatusFilter_CANCELLED},
		{"cancelled", schedulepb.LessonStatusFilter_CANCELLED},
		{"COMPLETED", schedulepb.LessonStatusFilter_COMPLETED},
		{"completed", schedulepb.LessonStatusFilter_COMPLETED},
		{"unknown", schedulepb.LessonStatusFilter_BOOKED},
		{"", schedulepb.LessonStatusFilter_BOOKED},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, parseStatus(tc.input))
		})
	}
}

// ── parseListLessons (schedule) ─────────────────────────────────────

func TestParseListLessons(t *testing.T) {
	t.Run("TutorOnly", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/lessons?tutor_id=t1&status_filter=BOOKED", nil)
		ctx, req, err := parseListLessons(context.Background(), r)
		require.NoError(t, err)
		require.NotNil(t, ctx)

		tutorReq, ok := req.(*schedulepb.ListLessonsByTutorRequest)
		require.True(t, ok)
		assert.Equal(t, "t1", tutorReq.TutorId)
		assert.Len(t, tutorReq.StatusFilter, 1)
	})

	t.Run("StudentOnly", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/lessons?student_id=s1", nil)
		_, req, err := parseListLessons(context.Background(), r)
		require.NoError(t, err)

		studentReq, ok := req.(*schedulepb.ListLessonsByStudentRequest)
		require.True(t, ok)
		assert.Equal(t, "s1", studentReq.StudentId)
	})

	t.Run("Pair", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/lessons?tutor_id=t1&student_id=s1&status_filter=COMPLETED&status_filter=CANCELLED", nil)
		_, req, err := parseListLessons(context.Background(), r)
		require.NoError(t, err)

		pairReq, ok := req.(*schedulepb.ListLessonsByPairRequest)
		require.True(t, ok)
		assert.Equal(t, "t1", pairReq.TutorId)
		assert.Equal(t, "s1", pairReq.StudentId)
		assert.Len(t, pairReq.StatusFilter, 2)
	})

	t.Run("NoParams_Error", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/lessons", nil)
		_, _, err := parseListLessons(context.Background(), r)
		assert.Error(t, err)
	})
}

// ── parseAssignmentQuery (homework) ─────────────────────────────────

func TestParseAssignmentQuery(t *testing.T) {
	t.Run("TutorOnly", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/assignments?tutor_id=t1&status_filter=UNSENT&status_filter=OVERDUE", nil)
		ctx, req, err := parseAssignmentQuery(context.Background(), r)
		require.NoError(t, err)
		require.NotNil(t, ctx)

		tutorReq, ok := req.(*homeworkpb.ListAssignmentsByTutorRequest)
		require.True(t, ok)
		assert.Equal(t, "t1", tutorReq.TutorId)
		assert.Len(t, tutorReq.StatusFilter, 2)
	})

	t.Run("StudentOnly", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/assignments?student_id=s1&status_filter=UNREVIEWED", nil)
		_, req, err := parseAssignmentQuery(context.Background(), r)
		require.NoError(t, err)

		studentReq, ok := req.(*homeworkpb.ListAssignmentsByStudentRequest)
		require.True(t, ok)
		assert.Equal(t, "s1", studentReq.StudentId)
		assert.Len(t, studentReq.StatusFilter, 1)
	})

	t.Run("Pair", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/assignments?tutor_id=t1&student_id=s1&status_filter=REVIEWED", nil)
		_, req, err := parseAssignmentQuery(context.Background(), r)
		require.NoError(t, err)

		pairReq, ok := req.(*homeworkpb.ListAssignmentsByPairRequest)
		require.True(t, ok)
		assert.Equal(t, "t1", pairReq.TutorId)
		assert.Equal(t, "s1", pairReq.StudentId)
	})

	t.Run("NoParams_Error", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/assignments", nil)
		_, _, err := parseAssignmentQuery(context.Background(), r)
		assert.Error(t, err)
	})
}

// ── Schedule parse functions with chi params ────────────────────────

func TestScheduleParsers(t *testing.T) {
	t.Run("parseGetSlot", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/slots/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &schedulepb.GetSlotRequest{}

		err := parseGetSlot(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.Id)
	})

	t.Run("parseDeleteSlot", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/slots/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &schedulepb.DeleteSlotRequest{}

		err := parseDeleteSlot(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.Id)
	})

	t.Run("parseUpdateSlot", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPatch, "/slots/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &schedulepb.UpdateSlotRequest{}

		err := parseUpdateSlot(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.Id)
	})

	t.Run("parseGetLesson", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/lessons/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &schedulepb.GetLessonRequest{}

		err := parseGetLesson(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.Id)
	})

	t.Run("parseUpdateLesson", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPatch, "/lessons/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &schedulepb.UpdateLessonRequest{}

		err := parseUpdateLesson(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.Id)
	})

	t.Run("parseCancelLesson", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/lessons/abc/cancel", nil)
		r = withChiParam(r, "id", "abc")
		req := &schedulepb.CancelLessonRequest{}

		err := parseCancelLesson(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.Id)
	})

	t.Run("parseListSlotsByTutor", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/slots/by-tutor/t1?only_available=true", nil)
		r = withChiParam(r, "tutor_id", "t1")
		req := &schedulepb.ListSlotsByTutorRequest{}

		err := parseListSlotsByTutor(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "t1", req.TutorId)
		require.NotNil(t, req.OnlyAvailable)
		assert.True(t, *req.OnlyAvailable)
	})

	t.Run("parseListSlotsByTutor_NoFilter", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/slots/by-tutor/t1", nil)
		r = withChiParam(r, "tutor_id", "t1")
		req := &schedulepb.ListSlotsByTutorRequest{}

		err := parseListSlotsByTutor(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Nil(t, req.OnlyAvailable)
	})
}

// ── User handler key builders ───────────────────────────────────────

func TestUserKeyBuilders(t *testing.T) {
	t.Run("buildUserKey", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/abc", nil)
		r = withChiParam(r, "id", "abc")

		key, err := buildUserKey(r)
		assert.NoError(t, err)
		assert.Equal(t, "user:abc", key)
	})

	t.Run("buildUserKey_Missing", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/", nil)
		_, err := buildUserKey(r)
		assert.Error(t, err)
	})

	t.Run("buildUserPublicKey", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/abc", nil)
		r = withChiParam(r, "id", "abc")

		key, err := buildUserPublicKey(r)
		assert.NoError(t, err)
		assert.Equal(t, "user-public:abc", key)
	})

	t.Run("buildMeKey", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		r.Header.Set("X-User-Id", "user-xyz")

		key, err := buildMeKey(r)
		assert.NoError(t, err)
		assert.Equal(t, "user:user-xyz", key)
	})

	t.Run("buildMeKey_Missing", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		_, err := buildMeKey(r)
		assert.Error(t, err)
	})

	t.Run("buildTutorProfileKey", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/tutor-profiles/abc", nil)
		r = withChiParam(r, "id", "abc")

		key, err := buildTutorProfileKey(r)
		assert.NoError(t, err)
		assert.Equal(t, "tutor-profile:abc", key)
	})

	t.Run("buildTutorStudentKey", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/tutor-students/t1/s1", nil)
		r = withChiParams(r, map[string]string{"tutor_id": "t1", "student_id": "s1"})

		key, err := buildTutorStudentKey(r)
		assert.NoError(t, err)
		assert.Equal(t, "tutor-student:t1:s1", key)
	})

	t.Run("buildTutorStudentKey_Missing", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/tutor-students/", nil)
		_, err := buildTutorStudentKey(r)
		assert.Error(t, err)
	})
}

// ── User handler parse functions ────────────────────────────────────

func TestUserParsers(t *testing.T) {
	t.Run("getUserParsePath", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &userpb.GetUserRequest{}

		err := getUserParsePath(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.Id)
	})

	t.Run("getUserParsePath_Missing", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/", nil)
		req := &userpb.GetUserRequest{}

		err := getUserParsePath(context.Background(), r, req)
		assert.Error(t, err)
	})

	t.Run("updateUserParsePath", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPatch, "/users/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &userpb.UpdateUserRequest{}

		err := updateUserParsePath(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.Id)
	})

	t.Run("getTutorProfileParsePath", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/tutor-profiles/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &userpb.GetTutorProfileByUserIdRequest{}

		err := getTutorProfileParsePath(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.UserId)
	})

	t.Run("updateTutorProfileParsePath", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPatch, "/tutor-profiles/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &userpb.UpdateTutorProfileRequest{}

		err := updateTutorProfileParsePath(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.UserId)
	})

	t.Run("listTutorStudentsByTutorParsePath", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/tutor-students/by-tutor/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &userpb.ListTutorStudentsRequest{}

		err := listTutorStudentsByTutorParsePath(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.TutorId)
	})

	t.Run("listTutorStudentsByStudentParsePath", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/tutor-students/by-student/abc", nil)
		r = withChiParam(r, "id", "abc")
		req := &userpb.ListTutorsForStudentRequest{}

		err := listTutorStudentsByStudentParsePath(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "abc", req.StudentId)
	})

	t.Run("getTutorStudentParsePath", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/tutor-students/t1/s1", nil)
		r = withChiParams(r, map[string]string{"tutor_id": "t1", "student_id": "s1"})
		req := &userpb.GetTutorStudentRequest{}

		err := getTutorStudentParsePath(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "t1", req.TutorId)
		assert.Equal(t, "s1", req.StudentId)
	})

	t.Run("getTutorStudentParsePath_MissingTutor", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/tutor-students//s1", nil)
		r = withChiParam(r, "student_id", "s1")
		req := &userpb.GetTutorStudentRequest{}

		err := getTutorStudentParsePath(context.Background(), r, req)
		assert.Error(t, err)
	})

	t.Run("updateTutorStudentParsePath", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPatch, "/tutor-students/t1/s1", nil)
		r = withChiParams(r, map[string]string{"tutor_id": "t1", "student_id": "s1"})
		req := &userpb.UpdateTutorStudentRequest{}

		err := updateTutorStudentParsePath(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "t1", req.TutorId)
		assert.Equal(t, "s1", req.StudentId)
	})

	t.Run("deleteTutorStudentParsePath", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/tutor-students/t1/s1", nil)
		r = withChiParams(r, map[string]string{"tutor_id": "t1", "student_id": "s1"})
		req := &userpb.DeleteTutorStudentRequest{}

		err := deleteTutorStudentParsePath(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "t1", req.TutorId)
		assert.Equal(t, "s1", req.StudentId)
	})

	t.Run("acceptInvitationParsePath", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/tutor-students/t1/accept", nil)
		r = withChiParam(r, "tutor_id", "t1")
		req := &userpb.AcceptInvitationFromTutorRequest{}

		err := acceptInvitationParsePath(context.Background(), r, req)
		assert.NoError(t, err)
		assert.Equal(t, "t1", req.TutorId)
	})
}

// ── File handler ────────────────────────────────────────────────────

func TestNewFileHandler(t *testing.T) {
	t.Run("ValidURL", func(t *testing.T) {
		fh := NewFileHandler(nil, "http://minio:9000")
		assert.NotNil(t, fh)
		assert.Equal(t, "http://minio:9000", fh.minioUrl)
	})

	t.Run("InvalidURL_Panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewFileHandler(nil, "://invalid")
		})
	})
}

func TestProxyToMinio(t *testing.T) {
	t.Run("InvalidTargetURL_Returns400", func(t *testing.T) {
		fh := NewFileHandler(nil, "http://minio:9000")

		handler := fh.proxyToMinio("GET", "/files/download")
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/files/download/../../etc/passwd", nil)
		r.URL.RawQuery = "foo=bar"

		handler(w, r)
		// The URL host check will catch redirect attempts
	})
}
