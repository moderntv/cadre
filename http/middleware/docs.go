// Package middleware holds the gin middleware every Cadre HTTP server is wrapped in.
//
// [NewLogger] returns the request logger. It writes one zerolog entry per handled request with the method,
// path, gin route template, status code, latency, sizes, client information, request ID and any errors the
// handlers collected into the gin context. [NewMetrics] returns the Prometheus middleware, which records a
// request counter and a duration summary per server.
//
// # Logger options
//
// The logger is configured with [LoggerOption] values, passed either to every server through
// cadre.WithHTTPLoggerOptions or to a single one through cadre.WithLoggerOptions:
//
//	middleware.WithLoggingIgnorePaths(`^/metrics$`, `^/status$`)
//	middleware.WithRequestBody(middleware.BodyLogOnError)
//	middleware.WithResponseBody(middleware.BodyLogOnError)
//	middleware.WithMaxBodySize(8 << 10)
//	middleware.WithRequestHeaders()
//
// Headers and bodies are opt-in because they are expensive and easy to leak secrets with. Sensitive headers
// and query parameters - see [DefaultRedactedHeaders] and [DefaultRedactedQueryParams] - are replaced with
// [RedactedValue] whenever they are logged.
//
// # Body logging
//
// A [BodyLogPolicy] decides when a captured body reaches the log entry: never, only when the response
// status is at least [DefaultBodyErrorStatus] (lower the threshold with [WithBodyErrorStatus]), or always.
// Bodies are captured for every request as soon as the policy is not [BodyLogNever]; the policy only
// controls whether the copy is written out. Handlers always see the full body - only the logged copy is
// truncated to [DefaultMaxBodySize] and flagged with a "<field>_truncated" key.
//
// Only textual content types are eligible, see [DefaultBodyContentTypes], and a body with any
// Content-Encoding other than identity is never captured. Valid JSON is embedded into the entry as JSON
// rather than as a string.
//
// An invalid ignore-path pattern is reported as an error from [NewLogger], which the Cadre builder surfaces
// from Build.
package middleware
