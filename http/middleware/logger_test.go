package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

func TestNewLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		ignorePatterns []*regexp.Regexp
		requestPath    string
		expectLog      bool
	}{
		{
			name:        "logs normal path",
			requestPath: "/api/users",
			expectLog:   true,
		},
		{
			name: "ignores exact status path",
			ignorePatterns: []*regexp.Regexp{
				regexp.MustCompile(`^/status$`),
			},
			requestPath: "/status",
			expectLog:   false,
		},
		{
			name: "ignores exact metrics path",
			ignorePatterns: []*regexp.Regexp{
				regexp.MustCompile(`^/metrics$`),
			},
			requestPath: "/metrics",
			expectLog:   false,
		},
		{
			name: "does not ignore partial match",
			ignorePatterns: []*regexp.Regexp{
				regexp.MustCompile(`^/status$`),
			},
			requestPath: "/status/details",
			expectLog:   true,
		},
		{
			name: "ignores with multiple patterns",
			ignorePatterns: []*regexp.Regexp{
				regexp.MustCompile(`^/status$`),
				regexp.MustCompile(`^/metrics$`),
			},
			requestPath: "/metrics",
			expectLog:   false,
		},
		{
			name:           "no ignore patterns logs everything",
			ignorePatterns: nil,
			requestPath:    "/status",
			expectLog:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

			handler := NewLogger(logger, WithIgnorePatterns(tt.ignorePatterns...))

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(handler)
			r.GET(tt.requestPath, func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.requestPath, nil)
			r.ServeHTTP(w, c.Request)

			logged := buf.Len() > 0
			if tt.expectLog {
				assert.True(t, logged)
			} else {
				assert.False(t, logged)
			}
		})
	}
}

func TestNewLoggerFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(NewLogger(logger))
	r.GET("/api/users/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})

	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/api/users/42?filter=active&token=supersecret",
		nil,
	)
	c.Request.Header.Set("User-Agent", "cadre-test")
	c.Request.Header.Set("Referer", "https://example.com/list")
	c.Request.Header.Set("X-Request-Id", "req-123")
	r.ServeHTTP(w, c.Request)

	entry := decodeLogEntry(t, buf.Bytes())

	assert.Equal(t, "http", entry["module"])
	assert.Equal(t, http.MethodGet, entry["method"])
	assert.Equal(t, "/api/users/42", entry["path"])
	assert.Equal(t, "/api/users/:id", entry["route"])
	assert.Equal(t, "HTTP/1.1", entry["proto"])
	assert.Equal(t, "example.com", entry["host"])
	assert.InEpsilon(t, float64(http.StatusOK), entry["status_code"], 0)
	assert.Equal(t, "cadre-test", entry["user_agent"])
	assert.Equal(t, "https://example.com/list", entry["referer"])
	assert.Equal(t, "req-123", entry["request_id"])
	assert.InEpsilon(t, float64(w.Body.Len()), entry["response_size"], 0)
	// the sensitive query parameter must not leak into the log
	assert.Equal(t, "filter=active&token="+RedactedValue, entry["query"])
	assert.NotContains(t, buf.String(), "supersecret")
	// bodies and headers are opt-in
	assert.NotContains(t, entry, "response_body")
	assert.NotContains(t, entry, "request_headers")
}

func TestNewLoggerRequestIDFromContext(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(NewLogger(logger))
	r.GET("/api/users", func(c *gin.Context) {
		c.Set(RequestIDContextKey, "ctx-42")
		c.Status(http.StatusOK)
	})

	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/users", nil)
	r.ServeHTTP(w, c.Request)

	entry := decodeLogEntry(t, buf.Bytes())
	assert.Equal(t, "ctx-42", entry["request_id"])
}

func TestNewLoggerBodyPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		policy      BodyLogPolicy
		errorStatus int
		statusCode  int
		expectBody  bool
	}{
		{name: "never", policy: BodyLogNever, statusCode: http.StatusInternalServerError, expectBody: false},
		{name: "always on success", policy: BodyLogAlways, statusCode: http.StatusOK, expectBody: true},
		{name: "on error with success", policy: BodyLogOnError, statusCode: http.StatusOK, expectBody: false},
		{
			// client errors are above the default error status threshold
			name:       "on error with client error",
			policy:     BodyLogOnError,
			statusCode: http.StatusBadRequest,
			expectBody: false,
		},
		{
			name:        "on error with client error and lowered threshold",
			policy:      BodyLogOnError,
			errorStatus: http.StatusBadRequest,
			statusCode:  http.StatusBadRequest,
			expectBody:  true,
		},
		{
			name:        "on error with success and lowered threshold",
			policy:      BodyLogOnError,
			errorStatus: http.StatusBadRequest,
			statusCode:  http.StatusOK,
			expectBody:  false,
		},
		{
			name:       "on error with server error",
			policy:     BodyLogOnError,
			statusCode: http.StatusInternalServerError,
			expectBody: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

			loggerOpts := []LoggerOption{WithRequestBody(tt.policy), WithResponseBody(tt.policy)}
			if tt.errorStatus != 0 {
				loggerOpts = append(loggerOpts, WithBodyErrorStatus(tt.errorStatus))
			}

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(NewLogger(logger, loggerOpts...))
			r.POST("/api/users", func(c *gin.Context) {
				body, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				// the handler must still see the whole request body
				assert.JSONEq(t, `{"name":"jane"}`, string(body))

				c.JSON(tt.statusCode, gin.H{"error": "nope"})
			})

			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/api/users",
				strings.NewReader(`{"name":"jane"}`),
			)
			c.Request.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, c.Request)

			entry := decodeLogEntry(t, buf.Bytes())
			if !tt.expectBody {
				assert.NotContains(t, entry, "request_body")
				assert.NotContains(t, entry, "response_body")

				return
			}

			assert.Equal(t, map[string]any{"name": "jane"}, entry["request_body"])
			assert.Equal(t, map[string]any{"error": "nope"}, entry["response_body"])
		})
	}
}

func TestNewLoggerBodyTruncation(t *testing.T) {
	t.Parallel()

	const limit = 16

	var buf bytes.Buffer

	logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

	body := strings.Repeat("a", limit*2)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(NewLogger(
		logger,
		WithRequestBody(BodyLogAlways),
		WithResponseBody(BodyLogAlways),
		WithMaxBodySize(limit),
	))
	r.POST("/api/echo", func(c *gin.Context) {
		received, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		// truncation must only affect the logged copy
		assert.Equal(t, body, string(received))

		c.String(http.StatusOK, body)
	})

	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/api/echo",
		strings.NewReader(body),
	)
	c.Request.Header.Set("Content-Type", "text/plain")
	r.ServeHTTP(w, c.Request)

	entry := decodeLogEntry(t, buf.Bytes())
	assert.Equal(t, strings.Repeat("a", limit), entry["request_body"])
	assert.Equal(t, true, entry["request_body_truncated"])
	assert.Equal(t, strings.Repeat("a", limit), entry["response_body"])
	assert.Equal(t, true, entry["response_body_truncated"])
}

func TestNewLoggerBodyContentTypeFilter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(NewLogger(logger, WithRequestBody(BodyLogAlways), WithResponseBody(BodyLogAlways)))
	r.POST("/api/upload", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)

		c.Data(http.StatusOK, "application/octet-stream", []byte{0x1, 0x2, 0x3})
	})

	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/api/upload",
		bytes.NewReader([]byte{0x4, 0x5, 0x6}),
	)
	c.Request.Header.Set("Content-Type", "application/octet-stream")
	r.ServeHTTP(w, c.Request)

	entry := decodeLogEntry(t, buf.Bytes())
	assert.NotContains(t, entry, "request_body")
	assert.NotContains(t, entry, "response_body")
}

func TestNewLoggerHeaders(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(NewLogger(logger, WithRequestHeaders(), WithResponseHeaders()))
	r.GET("/api/users", func(c *gin.Context) {
		c.Header("X-Custom", "value")
		c.Header("Set-Cookie", "session=secretcookie")
		c.Status(http.StatusOK)
	})

	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/users", nil)
	c.Request.Header.Set("Authorization", "Bearer secrettoken")
	c.Request.Header.Set("X-Trace", "abc")
	r.ServeHTTP(w, c.Request)

	entry := decodeLogEntry(t, buf.Bytes())

	requestHeaders, ok := entry["request_headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, RedactedValue, requestHeaders["Authorization"])
	assert.Equal(t, "abc", requestHeaders["X-Trace"])

	responseHeaders, ok := entry["response_headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, RedactedValue, responseHeaders["Set-Cookie"])
	assert.Equal(t, "value", responseHeaders["X-Custom"])

	assert.NotContains(t, buf.String(), "secrettoken")
	assert.NotContains(t, buf.String(), "secretcookie")
}

func TestNewLoggerHandlerErrors(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(NewLogger(logger))
	r.GET("/api/users", func(c *gin.Context) {
		_ = c.Error(assert.AnError)
		c.Status(http.StatusInternalServerError)
	})

	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/users", nil)
	r.ServeHTTP(w, c.Request)

	entry := decodeLogEntry(t, buf.Bytes())

	errs, ok := entry["errors"].([]any)
	require.True(t, ok)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0], assert.AnError.Error())
	assert.Equal(t, zerolog.LevelErrorValue, entry["level"])
}

// the handler must be able to consume a captured request body in every usual way.
func TestNewLoggerRequestBodyStaysUsable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		payload     string
		read        func(*testing.T, *gin.Context)
	}{
		{
			name:        "bind json",
			contentType: contentTypeJSON,
			payload:     `{"name":"jane","tags":["a","b"]}`,
			read: func(t *testing.T, c *gin.Context) {
				t.Helper()

				var payload struct {
					Name string   `json:"name"`
					Tags []string `json:"tags"`
				}

				require.NoError(t, c.ShouldBindJSON(&payload))
				assert.Equal(t, "jane", payload.Name)
				assert.Equal(t, []string{"a", "b"}, payload.Tags)
			},
		},
		{
			name:        "post form",
			contentType: "application/x-www-form-urlencoded",
			payload:     "name=jane&city=prague",
			read: func(t *testing.T, c *gin.Context) {
				t.Helper()

				assert.Equal(t, "jane", c.PostForm("name"))
				assert.Equal(t, "prague", c.PostForm("city"))
			},
		},
		{
			name:        "raw data",
			contentType: contentTypeJSON,
			payload:     `{"name":"jane"}`,
			read: func(t *testing.T, c *gin.Context) {
				t.Helper()

				raw, err := c.GetRawData()
				require.NoError(t, err)
				assert.JSONEq(t, `{"name":"jane"}`, string(raw))
			},
		},
		{
			name:        "second read is empty",
			contentType: contentTypeJSON,
			payload:     `{"name":"jane"}`,
			read: func(t *testing.T, c *gin.Context) {
				t.Helper()

				first, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				assert.JSONEq(t, `{"name":"jane"}`, string(first))

				// a request body is a stream - draining it twice yields nothing, capture or not
				second, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				assert.Empty(t, second)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			// a limit far below the payload size, so every case replays across the truncation boundary
			r.Use(newTestLogger(t, logger, WithRequestBody(BodyLogAlways), WithMaxBodySize(4)))
			r.POST("/api/users", func(c *gin.Context) {
				tt.read(t, c)
				c.Status(http.StatusOK)
			})

			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/api/users",
				strings.NewReader(tt.payload),
			)
			c.Request.Header.Set("Content-Type", tt.contentType)
			r.ServeHTTP(w, c.Request)

			entry := decodeLogEntry(t, buf.Bytes())
			assert.Equal(t, tt.payload[:4], entry["request_body"])
			assert.Equal(t, true, entry["request_body_truncated"])
		})
	}
}

// a compressed body would only add binary garbage to the log.
func TestNewLoggerSkipsEncodedBodies(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

	compressed := []byte{0x1f, 0x8b, 0x08, 0x00, 0xff, 0xfe}

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(newTestLogger(t, logger, WithRequestBody(BodyLogAlways), WithResponseBody(BodyLogAlways)))
	r.POST("/api/gzip", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)

		c.Header("Content-Encoding", "gzip")
		c.Data(http.StatusOK, "application/json", compressed)
	})

	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/api/gzip",
		bytes.NewReader(compressed),
	)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Content-Encoding", "gzip")
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, compressed, w.Body.Bytes(), "the client response was altered")

	entry := decodeLogEntry(t, buf.Bytes())
	assert.NotContains(t, entry, "request_body")
	assert.NotContains(t, entry, "response_body")
}

// a streaming response must keep flowing while only its first bytes are copied.
func TestNewLoggerStreamingResponse(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := zerolog.New(&buf).Level(zerolog.TraceLevel)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.Use(newTestLogger(t, logger, WithResponseBody(BodyLogAlways), WithMaxBodySize(9)))
	r.GET("/api/stream", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")

		for i := range 3 {
			c.String(http.StatusOK, "data: %d\n\n", i)
			c.Writer.Flush()
		}
	})

	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/stream", nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, "data: 0\n\ndata: 1\n\ndata: 2\n\n", w.Body.String(), "the stream was altered")

	entry := decodeLogEntry(t, buf.Bytes())
	assert.Equal(t, "data: 0\n\n", entry["response_body"])
	assert.Equal(t, true, entry["response_body_truncated"])
	assert.InEpsilon(t, float64(27), entry["response_size"], 0)
}

func newTestLogger(t *testing.T, logger zerolog.Logger, opts ...LoggerOption) gin.HandlerFunc {
	t.Helper()

	handler, err := NewLogger(logger, opts...)
	require.NoError(t, err)

	return handler
}

func decodeLogEntry(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	require.NotEmpty(t, raw, "expected a log entry")

	entry := map[string]any{}
	require.NoError(t, json.Unmarshal(raw, &entry))

	return entry
}
