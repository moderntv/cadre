package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const contentTypeJSON = "application/json"

// newTestBodyWriter wraps a recorder the way the middleware wraps gin's writer.
func newTestBodyWriter(
	t *testing.T,
	limit int,
	contentType string,
) (writer *bodyWriter, inner gin.ResponseWriter, recorder *httptest.ResponseRecorder) {
	t.Helper()

	recorder = httptest.NewRecorder()

	c, _ := gin.CreateTestContext(recorder)
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}

	writer = &bodyWriter{
		ResponseWriter:     c.Writer,
		limit:              limit,
		contentTypeAllowed: defaultLoggerOptions().contentTypeAllowed,
	}

	return writer, c.Writer, recorder
}

// whatever the writer copies for logging must also reach the client untouched.
func TestBodyWriterPassesResponseThrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		writes        []string
		useWriteStr   bool
		wantCaptured  string
		wantTruncated bool
	}{
		{name: "single write below limit", writes: []string{"abc"}, wantCaptured: "abc"}, //nolint:goconst
		{name: "single write at limit", writes: []string{payloadAtLimit}, wantCaptured: payloadAtLimit},
		{
			name:          "single write over limit",
			writes:        []string{payloadAtLimit + "ij"},
			wantCaptured:  payloadAtLimit,
			wantTruncated: true,
		},
		{
			name:         "many small writes below limit",
			writes:       []string{"a", "b", "c"},
			wantCaptured: "abc",
		},
		{
			// the limit is crossed in the middle of a write - the copy must stop mid-write
			name:          "many small writes crossing the limit",
			writes:        []string{"abcd", "efgh", "ijkl"},
			wantCaptured:  payloadAtLimit,
			wantTruncated: true,
		},
		{
			name:          "write strings crossing the limit",
			writes:        []string{payloadBelowLimit, "hijklmn"},
			useWriteStr:   true,
			wantCaptured:  payloadAtLimit,
			wantTruncated: true,
		},
		{
			name:          "huge single write",
			writes:        []string{strings.Repeat("z", 1<<20)},
			wantCaptured:  strings.Repeat("z", testBodyLimit),
			wantTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			writer, _, recorder := newTestBodyWriter(t, testBodyLimit, contentTypeJSON)

			var want strings.Builder

			for _, data := range tt.writes {
				want.WriteString(data)

				var (
					n   int
					err error
				)

				if tt.useWriteStr {
					n, err = writer.WriteString(data)
				} else {
					n, err = writer.Write([]byte(data))
				}

				require.NoError(t, err)
				assert.Equal(t, len(data), n, "the writer must report every byte as written")
			}

			assert.Equal(t, want.String(), recorder.Body.String(), "the client response was altered")
			assert.Equal(t, want.Len(), writer.Size(), "Size() must count all bytes, not just the captured ones")

			captured := writer.body()
			require.NotNil(t, captured)
			assert.Equal(t, tt.wantCaptured, string(captured.data))
			assert.Equal(t, tt.wantTruncated, captured.truncated)
		})
	}
}

func TestBodyWriterSkipsCapture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		contentType     string
		contentEncoding string
		limit           int
	}{
		{name: "binary content type", contentType: "application/octet-stream", limit: testBodyLimit},
		{name: "image content type", contentType: "image/png", limit: testBodyLimit},
		{name: "missing content type", contentType: "", limit: testBodyLimit},
		{
			name:            "compressed body",
			contentType:     contentTypeJSON,
			contentEncoding: "gzip",
			limit:           testBodyLimit,
		},
		{name: "zero limit", contentType: contentTypeJSON, limit: 0},
		{name: "negative limit", contentType: contentTypeJSON, limit: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			writer, _, recorder := newTestBodyWriter(t, tt.limit, tt.contentType)
			if tt.contentEncoding != "" {
				writer.Header().Set("Content-Encoding", tt.contentEncoding)
			}

			_, err := writer.Write([]byte("payload"))
			require.NoError(t, err)

			assert.Equal(t, "payload", recorder.Body.String(), "the client response was altered")
			assert.Nil(t, writer.body())
		})
	}
}

// an identity encoding is not a compressed one and must not disable capturing.
func TestBodyWriterIdentityEncoding(t *testing.T) {
	t.Parallel()

	writer, _, _ := newTestBodyWriter(t, testBodyLimit, contentTypeJSON)
	writer.Header().Set("Content-Encoding", "identity")

	_, err := writer.Write([]byte(`{"a":1}`))
	require.NoError(t, err)

	captured := writer.body()
	require.NotNil(t, captured)
	assert.JSONEq(t, `{"a":1}`, string(captured.data))
}

// the content type is decided once, on the first write - a later change must not splice the copy.
func TestBodyWriterContentTypeDecidedOnce(t *testing.T) {
	t.Parallel()

	writer, _, recorder := newTestBodyWriter(t, 64, contentTypeJSON)

	_, err := writer.Write([]byte(`{"a":`))
	require.NoError(t, err)

	writer.Header().Set("Content-Type", "application/octet-stream")

	_, err = writer.Write([]byte(`1}`))
	require.NoError(t, err)

	assert.Equal(t, `{"a":1}`, recorder.Body.String())

	captured := writer.body()
	require.NotNil(t, captured)
	assert.JSONEq(t, `{"a":1}`, string(captured.data))
}

// the wrapper must not hide the interfaces gin and net/http rely on.
func TestBodyWriterInterfaces(t *testing.T) {
	t.Parallel()

	writer, inner, _ := newTestBodyWriter(t, testBodyLimit, "text/plain")

	var wrapped gin.ResponseWriter = writer

	assert.Implements(t, (*http.Flusher)(nil), wrapped)
	assert.Implements(t, (*http.Hijacker)(nil), wrapped)
	assert.Implements(t, (*http.ResponseWriter)(nil), wrapped)

	// http.ResponseController reaches the underlying writer through Unwrap
	unwrapper, ok := wrapped.(interface{ Unwrap() http.ResponseWriter })
	require.True(t, ok, "bodyWriter must implement Unwrap for http.ResponseController")
	assert.Equal(t, inner, unwrapper.Unwrap())
}
