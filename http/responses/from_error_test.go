package responses

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	cerrors "github.com/moderntv/cadre/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFromError pins the mapping between the sentinel error types and the HTTP status codes. It is the
// contract the domain layer relies on to stay free of transport concerns, so a change here is a breaking
// change for every service using it.
func TestFromError(t *testing.T) {
	t.Parallel()

	cause := "the underlying cause"

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantData   string
	}{
		{
			name:       "invalid input",
			err:        cerrors.NewTyped(cerrors.ErrInvalidInput, fmt.Errorf("%s", cause)),
			wantStatus: http.StatusBadRequest,
			wantData:   cause,
		},
		{
			name:       "not allowed",
			err:        cerrors.NewTyped(cerrors.ErrNotAllowed, fmt.Errorf("%s", cause)),
			wantStatus: http.StatusForbidden,
			wantData:   cause,
		},
		{
			name:       "not found",
			err:        cerrors.NewTyped(cerrors.ErrNotFound, fmt.Errorf("%s", cause)),
			wantStatus: http.StatusNotFound,
			wantData:   cause,
		},
		{
			name:       "temporary unavailable",
			err:        cerrors.NewTyped(cerrors.ErrTemporaryUnavailable, fmt.Errorf("%s", cause)),
			wantStatus: http.StatusServiceUnavailable,
			wantData:   cause,
		},
		{
			name:       "internal error",
			err:        cerrors.NewTyped(cerrors.ErrInternalError, fmt.Errorf("%s", cause)),
			wantStatus: http.StatusInternalServerError,
			wantData:   cause,
		},
		{
			name:       "the sentinel itself, without a cause",
			err:        cerrors.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantData:   "not found",
		},
		{
			name:       "an untyped error falls back to 500",
			err:        fmt.Errorf("%s", cause),
			wantStatus: http.StatusInternalServerError,
			wantData:   cause,
		},
		{
			name:       "a typed error wrapped again is still recognised",
			err:        fmt.Errorf("loading order: %w", cerrors.NewTyped(cerrors.ErrNotFound, fmt.Errorf("%s", cause))),
			wantStatus: http.StatusNotFound,
			wantData:   cause,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			FromError(c, tt.err)

			assert.Equal(t, tt.wantStatus, w.Code)

			var body ErrorResponse

			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Len(t, body.Errors, 1)
			// the cause is preserved in the payload so that the caller learns what actually went wrong
			assert.Contains(t, body.Errors[0].Data, tt.wantData)
		})
	}
}

// TestTypedErrorUnwrap covers the half of the contract that lives in the errors package - a TypedError has
// to report the sentinel through errors.Is while keeping the cause as its message.
func TestTypedErrorUnwrap(t *testing.T) {
	t.Parallel()

	err := cerrors.NewTyped(cerrors.ErrNotFound, fmt.Errorf("order %q does not exist", "42"))

	require.ErrorIs(t, err, cerrors.ErrNotFound)
	require.NotErrorIs(t, err, cerrors.ErrInvalidInput)
	assert.Equal(t, `order "42" does not exist`, err.Error())
}
