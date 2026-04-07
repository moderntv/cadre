package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
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

			handler := NewLogger(logger, tt.ignorePatterns)

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
