package middleware

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// BodyLogPolicy controls when request/response bodies are included in the log entry.
type BodyLogPolicy uint8

const (
	// BodyLogNever disables body logging completely. Bodies are not even captured.
	BodyLogNever BodyLogPolicy = iota
	// BodyLogOnError captures the body of every request but only writes it to the log
	// when the response status code is at least the error threshold - see WithBodyErrorStatus.
	BodyLogOnError
	// BodyLogAlways writes the body to the log of every request which is not ignored.
	BodyLogAlways
)

const (
	// DefaultMaxBodySize is the maximum number of body bytes kept for logging. Longer bodies are truncated.
	DefaultMaxBodySize = 4 << 10 // 4 KiB

	// DefaultBodyErrorStatus is the lowest status code considered an error by the BodyLogOnError policy.
	// Client errors are not included by default - they are usually caused by the caller, not by a bug worth debugging.
	DefaultBodyErrorStatus = http.StatusInternalServerError

	// RequestIDContextKey is the gin context key checked for a request ID
	// when none of the configured request ID headers is present.
	RequestIDContextKey = "request_id"

	// RedactedValue replaces the value of redacted headers and query parameters.
	RedactedValue = "[REDACTED]"
)

// DefaultRedactedHeaders are the headers whose values are never logged verbatim.
var DefaultRedactedHeaders = []string{
	"Authorization",
	"Proxy-Authorization",
	"Cookie",
	"Set-Cookie",
	"X-Api-Key",
	"X-Auth-Token",
	"X-Csrf-Token",
}

// DefaultRedactedQueryParams are the query parameters whose values are never logged verbatim.
var DefaultRedactedQueryParams = []string{
	"access_token",
	"api_key",
	"apikey",
	"auth",
	"authorization",
	"client_secret",
	"id_token",
	"password",
	"refresh_token",
	"secret",
	"sig",
	"signature",
	"token",
}

// DefaultBodyContentTypes are the content type prefixes whose bodies are eligible for logging.
// Anything else (images, video, octet-stream, ...) is skipped even when body logging is enabled.
var DefaultBodyContentTypes = []string{
	"application/json",
	"application/problem+json",
	"application/x-www-form-urlencoded",
	"application/xml",
	"text/",
}

// DefaultRequestIDHeaders are the headers searched for a request ID, in order.
var DefaultRequestIDHeaders = []string{
	"X-Request-Id",
	"X-Correlation-Id",
}

type loggerOptions struct {
	ignorePatterns []*regexp.Regexp

	requestBodyPolicy  BodyLogPolicy
	responseBodyPolicy BodyLogPolicy
	bodyErrorStatus    int
	maxBodySize        int
	bodyContentTypes   []string

	logRequestHeaders  bool
	logResponseHeaders bool
	redactedHeaders    map[string]struct{}
	redactedQuery      map[string]struct{}

	requestIDHeaders []string
}

func defaultLoggerOptions() *loggerOptions {
	return &loggerOptions{
		requestBodyPolicy:  BodyLogNever,
		responseBodyPolicy: BodyLogNever,
		bodyErrorStatus:    DefaultBodyErrorStatus,
		maxBodySize:        DefaultMaxBodySize,
		bodyContentTypes:   DefaultBodyContentTypes,

		redactedHeaders: toLowerSet(DefaultRedactedHeaders),
		redactedQuery:   toLowerSet(DefaultRedactedQueryParams),

		requestIDHeaders: DefaultRequestIDHeaders,
	}
}

// LoggerOption configures the HTTP logging middleware created by NewLogger.
type LoggerOption func(*loggerOptions)

// WithIgnorePatterns skips logging of requests whose URL path matches any of the patterns.
func WithIgnorePatterns(patterns ...*regexp.Regexp) LoggerOption {
	return func(o *loggerOptions) {
		o.ignorePatterns = append(o.ignorePatterns, patterns...)
	}
}

// WithRequestBody configures logging of the request body.
// Note that any policy other than BodyLogNever makes the middleware buffer up to
// the configured maximum body size (see WithMaxBodySize) for every handled request.
func WithRequestBody(policy BodyLogPolicy) LoggerOption {
	return func(o *loggerOptions) {
		o.requestBodyPolicy = policy
	}
}

// WithResponseBody configures logging of the response body.
// Note that any policy other than BodyLogNever makes the middleware buffer up to
// the configured maximum body size (see WithMaxBodySize) for every handled request.
func WithResponseBody(policy BodyLogPolicy) LoggerOption {
	return func(o *loggerOptions) {
		o.responseBodyPolicy = policy
	}
}

// WithMaxBodySize sets the maximum number of body bytes kept for logging.
// Longer bodies are truncated and flagged with a `<body_field>_truncated` field.
func WithMaxBodySize(size int) LoggerOption {
	return func(o *loggerOptions) {
		o.maxBodySize = size
	}
}

// WithBodyErrorStatus sets the lowest status code considered an error by the BodyLogOnError policy.
func WithBodyErrorStatus(statusCode int) LoggerOption {
	return func(o *loggerOptions) {
		o.bodyErrorStatus = statusCode
	}
}

// WithBodyContentTypes replaces the content type prefixes whose bodies may be logged.
// Passing no prefixes allows any content type.
func WithBodyContentTypes(prefixes ...string) LoggerOption {
	return func(o *loggerOptions) {
		o.bodyContentTypes = prefixes
	}
}

// WithRequestHeaders enables logging of the request headers.
func WithRequestHeaders() LoggerOption {
	return func(o *loggerOptions) {
		o.logRequestHeaders = true
	}
}

// WithResponseHeaders enables logging of the response headers.
func WithResponseHeaders() LoggerOption {
	return func(o *loggerOptions) {
		o.logResponseHeaders = true
	}
}

// WithRedactedHeaders replaces the set of headers whose values are redacted - see DefaultRedactedHeaders.
func WithRedactedHeaders(names ...string) LoggerOption {
	return func(o *loggerOptions) {
		o.redactedHeaders = toLowerSet(names)
	}
}

// WithRedactedQueryParams replaces the set of query parameters
// whose values are redacted - see DefaultRedactedQueryParams.
func WithRedactedQueryParams(names ...string) LoggerOption {
	return func(o *loggerOptions) {
		o.redactedQuery = toLowerSet(names)
	}
}

// WithRequestIDHeaders replaces the headers searched for a request ID - see DefaultRequestIDHeaders.
// The RequestIDContextKey gin context key is used as a fallback when no header matches.
func WithRequestIDHeaders(names ...string) LoggerOption {
	return func(o *loggerOptions) {
		o.requestIDHeaders = names
	}
}

func (lo *loggerOptions) isIgnored(path string) bool {
	for _, pattern := range lo.ignorePatterns {
		if pattern.MatchString(path) {
			return true
		}
	}

	return false
}

func (lo *loggerOptions) contentTypeAllowed(contentType string) bool {
	if len(lo.bodyContentTypes) == 0 {
		return true
	}

	contentType = strings.ToLower(contentType)
	for _, prefix := range lo.bodyContentTypes {
		if strings.HasPrefix(contentType, strings.ToLower(prefix)) {
			return true
		}
	}

	return false
}

// shouldLog reports whether a body captured under the given policy is written to the log entry.
func (lo *loggerOptions) shouldLog(policy BodyLogPolicy, statusCode int) bool {
	switch policy {
	case BodyLogAlways:
		return true
	case BodyLogOnError:
		return statusCode >= lo.bodyErrorStatus
	case BodyLogNever:
		return false
	}

	return false
}

// isEncoded reports whether a Content-Encoding header means the body bytes are compressed,
// in which case capturing them would only put binary garbage into the log.
func isEncoded(contentEncoding string) bool {
	encoding := strings.TrimSpace(strings.ToLower(contentEncoding))

	return encoding != "" && encoding != "identity"
}

func (lo *loggerOptions) isRedactedHeader(name string) bool {
	_, ok := lo.redactedHeaders[strings.ToLower(name)]

	return ok
}

// redactQuery returns the raw query string with the values of sensitive parameters replaced.
// The query is not parsed into url.Values so that the original formatting and escaping are kept.
func (lo *loggerOptions) redactQuery(rawQuery string) string {
	if rawQuery == "" || len(lo.redactedQuery) == 0 {
		return rawQuery
	}

	params := strings.Split(rawQuery, "&")

	for i, param := range params {
		escapedName, _, hasValue := strings.Cut(param, "=")
		if !hasValue {
			continue
		}

		name, err := url.QueryUnescape(escapedName)
		if err != nil {
			name = escapedName
		}

		_, sensitive := lo.redactedQuery[strings.ToLower(name)]
		if sensitive {
			params[i] = escapedName + "=" + RedactedValue
		}
	}

	return strings.Join(params, "&")
}

// requestID looks the request ID up in the configured headers.
func (lo *loggerOptions) requestID(header http.Header) string {
	for _, name := range lo.requestIDHeaders {
		id := header.Get(name)
		if id != "" {
			return id
		}
	}

	return ""
}

func toLowerSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[strings.ToLower(value)] = struct{}{}
	}

	return set
}
