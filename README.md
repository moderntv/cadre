[![Go Report Card](https://goreportcard.com/badge/github.com/moderntv/cadre)](https://goreportcard.com/report/github.com/moderntv/cadre)
![Go Version](https://img.shields.io/github/go-mod/go-version/moderntv/cadre)
![Lint Workflow Status](https://github.com/moderntv/cadre/actions/workflows/ci.yml/badge.svg?branch=master)

# Cadre

Cadre is a strongly opinionated library intended to removed boilerplate code from a modern Go application supporting gRPC and HTTP.
It has been build for internal projects needs at [ModernTV](https://www.moderntv.eu).

Cadre makes it easy to create and application with gRPC and/or HTTP interface.
It provides prometheus metrics and application status endpoints, debugging tools, logging and various gRPC utils.

Cadre tries to be flexible but enforces several libraries:

- logging - [zerolog](https://github.com/rs/zerolog)
- http server - [gin](https://github.com/gin-gonic/gin)

See `_examples` folder for usage details.

## HTTP request logging

Every HTTP server gets a logging middleware which logs one entry per handled request with the following fields:

| Field | Note |
|---|---|
| `method`, `path`, `route`, `host`, `proto` | `route` is the gin route template (`/users/:id`), useful for aggregation |
| `status_code`, `latency`, `latency_str` | |
| `request_size`, `response_size` | `request_size` is omitted for requests with an unknown length |
| `ip`, `user_agent`, `referer` | |
| `query` | values of sensitive parameters (`token`, `password`, ...) are redacted |
| `request_id` | first match of `X-Request-Id`/`X-Correlation-Id`, falling back to the `request_id` gin context key |
| `errors` | errors collected by the handlers into `gin.Context` |

Headers and bodies are opt-in because they are expensive and easy to leak secrets with:

```go
b, err := cadre.NewBuilder(
    "example",
    cadre.WithLogger(logger),
    cadre.WithHTTPLoggerOptions(
        middleware.WithLoggingIgnorePaths(`^/metrics$`, `^/status$`),
        // log the request and response body of failed requests only
        middleware.WithRequestBody(middleware.BodyLogOnError),
        middleware.WithResponseBody(middleware.BodyLogOnError),
        // include client errors as well - 5xx only by default
        middleware.WithBodyErrorStatus(http.StatusBadRequest),
        middleware.WithMaxBodySize(8<<10),
        middleware.WithRequestHeaders(),
        middleware.WithResponseHeaders(),
    ),
    cadre.WithHTTP("main_http", cadre.WithHTTPListeningAddress(":8000")),
)
```

Notes:

- bodies are captured for every request as soon as the policy is not `BodyLogNever` - the policy only decides whether
  the captured body ends up in the log entry. The handler always sees the full body; only the logged copy is truncated
  to the configured maximum size (4 KiB by default) and flagged with `<field>_truncated`.
- `BodyLogOnError` logs bodies of `500`+ responses only. Lower the threshold with `middleware.WithBodyErrorStatus`.
- only textual content types are captured (JSON, XML, form data, `text/*`) - see `middleware.WithBodyContentTypes`.
- compressed bodies (any `Content-Encoding` other than `identity`) are never captured.
- valid JSON bodies are embedded into the log entry as JSON instead of a string.
- sensitive headers (`Authorization`, `Cookie`, ...) and query parameters are replaced with `[REDACTED]`.
  The lists are configurable with `middleware.WithRedactedHeaders` and `middleware.WithRedactedQueryParams`.
- `cadre.WithHTTPLoggerOptions` applies to all HTTP servers, `cadre.WithLoggerOptions` to a single one.
- an invalid ignore path pattern is reported as an error from `b.Build()`.

## Disclaimer

Cadre is not production ready. It is under heavy development and its API can be changed at any time.

## Why Cadre?

[Cadre](https://www.wordnik.com/words/cadre)
