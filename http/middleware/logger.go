package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func NewLogger(baseLogger zerolog.Logger) func(*gin.Context) {
	logger := baseLogger.With().Str("module", "http").Logger()

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		path := c.Request.URL.Path
		dumplogger := logger.With().
			Str("method", c.Request.Method).
			Str("path", path).
			Dur("latency_ns", latency).
			Int("status_code", c.Writer.Status()).
			Str("ip", c.ClientIP()).Logger()

		switch {
		case c.Writer.Status() >= http.StatusBadRequest && c.Writer.Status() < http.StatusInternalServerError:
			dumplogger.Warn().Msg("HTTP request handled")
		case c.Writer.Status() >= http.StatusInternalServerError:
			dumplogger.Error().Msg("HTTP request handled")
		default:
			dumplogger.Trace().Msg("HTTP request handled")
		}
	}
}
