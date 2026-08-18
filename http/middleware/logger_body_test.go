package middleware

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBodyLimit = 8

	// payloads around the testBodyLimit boundary.
	payloadBelowLimit = "abcdefg"
	payloadAtLimit    = "abcdefgh"
)

// TestCaptureRequestBodyIsReplayed is the core guarantee of the request body capture:
// whatever the middleware reads for logging must still be readable in full by the handler.
func TestCaptureRequestBodyIsReplayed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		payload       string
		wantCaptured  string
		wantTruncated bool
	}{
		{name: "empty body", payload: "", wantCaptured: ""},
		{name: "single byte", payload: "a", wantCaptured: "a"},
		{name: "below limit", payload: payloadBelowLimit, wantCaptured: payloadBelowLimit},
		{name: "exactly limit", payload: payloadAtLimit, wantCaptured: payloadAtLimit},
		{
			name:          "one byte over limit",
			payload:       payloadAtLimit + "i",
			wantCaptured:  payloadAtLimit,
			wantTruncated: true,
		},
		{
			name:          "far over limit",
			payload:       strings.Repeat("0123456789", 100),
			wantCaptured:  "01234567",
			wantTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/",
				strings.NewReader(tt.payload),
			)

			captured := captureRequestBody(request, testBodyLimit)

			// the handler must see the payload in its entirety, capture or not
			replayed, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.payload, string(replayed), "handler read a different body than was sent")

			if tt.payload == "" {
				assert.Nil(t, captured)

				return
			}

			require.NotNil(t, captured)
			assert.Equal(t, tt.wantCaptured, string(captured.data))
			assert.Equal(t, tt.wantTruncated, captured.truncated)
		})
	}
}

// the handler may read the replayed body in arbitrarily small pieces, including across
// the boundary between the copy held in memory and the untouched remainder of the stream.
func TestCaptureRequestBodyChunkedReads(t *testing.T) {
	t.Parallel()

	for _, chunkSize := range []int{1, 3, testBodyLimit, testBodyLimit + 1, 64} {
		t.Run(strings.Repeat("x", chunkSize), func(t *testing.T) {
			t.Parallel()

			payload := "0123456789abcdefghij"

			request := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/",
				strings.NewReader(payload),
			)

			captured := captureRequestBody(request, testBodyLimit)
			require.NotNil(t, captured)

			var replayed bytes.Buffer

			chunk := make([]byte, chunkSize)

			for {
				n, err := request.Body.Read(chunk)
				replayed.Write(chunk[:n])

				if errors.Is(err, io.EOF) {
					break
				}

				require.NoError(t, err)
			}

			assert.Equal(t, payload, replayed.String())
		})
	}
}

// a body much larger than the limit must survive byte for byte - only the logged copy is capped.
func TestCaptureRequestBodyLargePayloadIntegrity(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("cadre payload chunk "), 100_000) // ~2 MiB
	want := sha256.Sum256(payload)

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(payload))

	captured := captureRequestBody(request, testBodyLimit)
	require.NotNil(t, captured)
	assert.Len(t, captured.data, testBodyLimit)
	assert.True(t, captured.truncated)

	replayed, err := io.ReadAll(request.Body)
	require.NoError(t, err)
	assert.Equal(t, want, sha256.Sum256(replayed), "large body was corrupted by the capture")
	assert.Len(t, replayed, len(payload))
}

func TestCaptureRequestBodyNotCaptured(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		body  io.ReadCloser
		limit int
	}{
		{name: "nil body", body: nil, limit: testBodyLimit},
		{name: "no body", body: http.NoBody, limit: testBodyLimit},
		{name: "zero limit", body: io.NopCloser(strings.NewReader("abc")), limit: 0},
		{name: "negative limit", body: io.NopCloser(strings.NewReader("abc")), limit: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
			request.Body = tt.body

			assert.Nil(t, captureRequestBody(request, tt.limit))
			// the body must be left exactly as it was, not replaced by an empty reader
			assert.Equal(t, tt.body, request.Body)
		})
	}
}

// the replayed body must still close the original one - otherwise connections leak.
func TestCaptureRequestBodyCloses(t *testing.T) {
	t.Parallel()

	original := &countingBody{Reader: strings.NewReader("abcdefghij")}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	request.Body = original

	require.NotNil(t, captureRequestBody(request, testBodyLimit))
	require.NoError(t, request.Body.Close())

	assert.Equal(t, 1, original.closes, "the original request body was not closed exactly once")
}

// a failing body must not be swallowed: the handler has to see the read error.
func TestCaptureRequestBodyReadError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("connection reset")

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	request.Body = &countingBody{Reader: io.MultiReader(strings.NewReader("abc"), errReader{err: wantErr})}

	assert.Nil(t, captureRequestBody(request, testBodyLimit), "a failed read must not be logged")

	replayed, err := io.ReadAll(request.Body)
	require.ErrorIs(t, err, wantErr)
	assert.Equal(t, "abc", string(replayed), "bytes read before the error must still reach the handler")
}

type countingBody struct {
	io.Reader

	closes int
}

func (cb *countingBody) Close() error {
	cb.closes++

	return nil
}

type errReader struct {
	err error
}

func (er errReader) Read([]byte) (int, error) {
	return 0, er.err
}
