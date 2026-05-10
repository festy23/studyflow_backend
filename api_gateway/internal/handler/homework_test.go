package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	homeworkpb "homework_service/pkg/api"
)

// BUG-048: When Handle returns (nil, err), calling the nil handler panics.
// handlerInitError must instead respond with 500 and not panic.
func TestHandlerInitError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)

	assert.NotPanics(t, func() {
		handlerInitError(w, r, errors.New("boom"))
	})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Verifies that passing a nil grpc method to Handle returns ErrNilMethod
// (not a nil handler that would panic on call). This is the exact failure
// mode BUG-048 fixes in homework.go.
func TestHandle_NilMethod_HomeworkType(t *testing.T) {
	handler, err := Handle[homeworkpb.CreateAssignmentRequest, homeworkpb.Assignment](nil, nil, true)
	assert.Nil(t, handler)
	assert.ErrorIs(t, err, ErrNilMethod)
}
