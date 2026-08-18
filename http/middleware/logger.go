package middleware

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// NewLogger creates a gin middleware which logs every handled HTTP request into the given zerolog logger.
//
// Request metadata (method, path, route, status, latency, sizes, query, user agent, ...) is logged by default.
// Headers and bodies are opt-in - see WithRequestHeaders, WithResponseHeaders, WithRequestBody and WithResponseBody.
func NewLogger(log zerolog.Logger, opts ...LoggerOption) (handler gin.HandlerFunc, err error) {
	options := defaultLoggerOptions()

	for _, opt := range opts {
		err = opt(options)
		if err != nil {
			return nil, fmt.Errorf("cannot apply logger option: %w", err)
		}
	}

	log = log.With().Str("module", "http").Logger()

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if options.isIgnored(path) {
			c.Next()

			return
		}

		var requestBody *body
		if options.requestBodyPolicy != BodyLogNever &&
			options.contentTypeAllowed(c.ContentType()) &&
			!isEncoded(c.GetHeader("Content-Encoding")) {
			requestBody = captureRequestBody(c.Request, options.maxBodySize)
		}

		var responseWriter *bodyWriter
		if options.responseBodyPolicy != BodyLogNever {
			responseWriter = &bodyWriter{
				ResponseWriter:     c.Writer,
				limit:              options.maxBodySize,
				contentTypeAllowed: options.contentTypeAllowed,
			}
			c.Writer = responseWriter
		}

		start := time.Now()

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		logCtx := log.With().
			Str("method", c.Request.Method).
			Str("path", path).
			Str("host", c.Request.Host).
			Str("proto", c.Request.Proto).
			Dur("latency", latency).
			// keep this for log aggregation, where number is better than user-readable string
			Str("latency_str", latency.String()). // user-readable latency
			Int("status_code", statusCode).
			Int("response_size", max(c.Writer.Size(), 0)).
			Str("ip", c.ClientIP())

		logCtx = withRequestFields(logCtx, c, options)

		if options.logRequestHeaders {
			logCtx = withHeaders(logCtx, "request_headers", c.Request.Header, options)
		}

		if options.logResponseHeaders {
			logCtx = withHeaders(logCtx, "response_headers", c.Writer.Header(), options)
		}

		if options.shouldLog(options.requestBodyPolicy, statusCode) {
			logCtx = withBody(logCtx, "request_body", requestBody)
		}

		if responseWriter != nil && options.shouldLog(options.responseBodyPolicy, statusCode) {
			logCtx = withBody(logCtx, "response_body", responseWriter.body())
		}

		log := logCtx.Logger()

		const msg = "HTTP request handled"

		switch {
		case statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError:
			log.Warn().Msg(msg)
		case statusCode >= http.StatusInternalServerError:
			log.Error().Msg(msg)
		default:
			log.Trace().Msg(msg)
		}
	}, nil
}

// withRequestFields adds the request-scoped fields which are only logged when they carry a value.
func withRequestFields(ctx zerolog.Context, c *gin.Context, options *loggerOptions) zerolog.Context {
	route := c.FullPath()
	if route != "" {
		ctx = ctx.Str("route", route)
	}

	query := options.redactQuery(c.Request.URL.RawQuery)
	if query != "" {
		ctx = ctx.Str("query", query)
	}

	userAgent := c.Request.UserAgent()
	if userAgent != "" {
		ctx = ctx.Str("user_agent", userAgent)
	}

	referer := c.Request.Referer()
	if referer != "" {
		ctx = ctx.Str("referer", referer)
	}

	if c.Request.ContentLength >= 0 {
		ctx = ctx.Int64("request_size", c.Request.ContentLength)
	}

	requestID := options.requestID(c.Request.Header)
	if requestID == "" {
		requestID = c.GetString(RequestIDContextKey)
	}

	if requestID != "" {
		ctx = ctx.Str("request_id", requestID)
	}

	if len(c.Errors) > 0 {
		handlerErrors := ctx.CreateArray()
		for _, err := range c.Errors {
			handlerErrors = handlerErrors.Str(err.Error())
		}

		ctx = ctx.Array("errors", handlerErrors)
	}

	return ctx
}

// withHeaders adds headers - sorted for stable output - with the values of the sensitive ones redacted.
func withHeaders(ctx zerolog.Context, key string, header http.Header, options *loggerOptions) zerolog.Context {
	dict := ctx.CreateDict()

	for _, name := range slices.Sorted(maps.Keys(header)) {
		if options.isRedactedHeader(name) {
			dict = dict.Str(name, RedactedValue)

			continue
		}

		dict = dict.Str(name, strings.Join(header[name], ", "))
	}

	return ctx.Dict(key, dict)
}

// withBody adds a captured body under the given key. Valid JSON is embedded as-is to keep the log entry structured.
func withBody(ctx zerolog.Context, key string, body *body) zerolog.Context {
	if body == nil || len(body.data) == 0 {
		return ctx
	}

	if json.Valid(body.data) {
		ctx = ctx.RawJSON(key, body.data)
	} else {
		ctx = ctx.Bytes(key, body.data)
	}

	if body.truncated {
		ctx = ctx.Bool(key+"_truncated", true)
	}

	return ctx
}
