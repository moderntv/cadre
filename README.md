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

## Contents

- [Installation](#installation)
- [Quick start](#quick-start)
- [Server topologies](#server-topologies)
- [Routing](#routing)
- [Responses and errors](#responses-and-errors)
- [Status](#status)
- [Metrics](#metrics)
- [HTTP request logging](#http-request-logging)
- [gRPC server](#grpc-server)
- [Graceful shutdown](#graceful-shutdown)
- [Service registry](#service-registry)
- [Shard load balancer](#shard-load-balancer)
- [Configuration](#configuration)
- [Examples](#examples)

## Installation

```bash
go get github.com/moderntv/cadre
```

## Quick start

Everything is assembled through a builder. Options configure it, `Build()` validates the configuration and
returns a server, and `Start()` blocks until the process is signalled.

```go
package main

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/moderntv/cadre"
	"github.com/moderntv/cadre/http"
	"github.com/moderntv/cadre/http/responses"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	b, err := cadre.NewBuilder(
		"my-service",
		cadre.WithLogger(logger),
		cadre.WithHTTP(
			"main_http",
			cadre.WithHTTPListeningAddress(":8000"),
			cadre.WithRoute("GET", "/hello", func(c *gin.Context) {
				responses.Ok(c, gin.H{"hello": "world"})
			}),
		),
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot configure cadre")
	}

	c, err := b.Build()
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot build cadre")
	}

	err = c.Start()
	if err != nil {
		logger.Fatal().Err(err).Msg("cadre failed")
	}
}
```

This already gives you:

```
GET http://localhost:8000/hello     your route
GET http://localhost:8000/metrics   prometheus exposition
GET http://localhost:8000/status    component health report
```

Watch the import: `github.com/moderntv/cadre/http` shadows the standard library's `net/http`. Alias the
standard one as `stdhttp` when a file needs both.

### Builder option scopes

There are three levels of options, and they are not interchangeable:

| Type | Applies to | Passed to |
| --- | --- | --- |
| `cadre.Option` | the whole application | `cadre.NewBuilder` |
| `cadre.HTTPOption` | one HTTP server | `cadre.WithHTTP` |
| `cadre.GRPCOption` | the gRPC server | `cadre.WithGRPC` |

Options only record intent. Nothing is validated or constructed until `Build()`, so configuration mistakes
surface there rather than at the call site.

## Server topologies

Cadre can run any number of HTTP servers and at most one gRPC server. **HTTP servers are identified by their
listening address**: configuring two with the same address merges them into a single server, combining their
routes, middleware and logger options. A path+method registered twice by the merged servers is a `Build()`
error.

This merging is what places the internal endpoints. `/metrics` and `/status` default to the address of the
first configured HTTP server; giving them their own address moves them onto a separate internal server.

### HTTP only

```go
cadre.NewBuilder("example",
	cadre.WithHTTP("main_http", cadre.WithHTTPListeningAddress(":8000")),
)
// :8000 - routes + /metrics + /status
```

### Internal endpoints on their own port

```go
cadre.NewBuilder("example",
	cadre.WithMetricsListeningAddress(":7000"),
	cadre.WithStatusListeningAddress(":7000"),
	cadre.WithHTTP("main_http", cadre.WithHTTPListeningAddress(":8000")),
)
// :8000 - routes
// :7000 - /metrics + /status  (same address -> merged into one internal server)
```

Pass different addresses to put them on separate servers, or omit one to leave it on the main server.

### HTTP and gRPC on separate ports

```go
cadre.NewBuilder("example",
	cadre.WithGRPC(
		cadre.WithGRPCListeningAddress(":9000"),
		cadre.WithService("example.GreeterService", registrator),
	),
	cadre.WithHTTP("main_http", cadre.WithHTTPListeningAddress(":8000")),
)
```

### HTTP and gRPC on one port

`WithGRPCMultiplex()` drops the standalone gRPC listener and serves both protocols on the HTTP server's
address, dispatching per request on the HTTP/2 + `application/grpc` content type. Cleartext HTTP/2 is
enabled on that server so plain (non-TLS) gRPC clients work.

```go
cadre.NewBuilder("example",
	cadre.WithGRPC(
		cadre.WithGRPCMultiplex(),
		cadre.WithService("example.GreeterService", registrator),
	),
	cadre.WithHTTP("main_http", cadre.WithHTTPListeningAddress(":9000")),
)
```

```bash
curl localhost:9000/hello
grpcurl -plaintext -d '{"name":"Alice"}' localhost:9000 example.GreeterService/SayHi
```

`WithGRPCMultiplex` and `WithGRPCListeningAddress` are mutually exclusive.

## Routing

`cadre.WithRoute` registers a single route. For anything larger, describe the tree with a
`http.RoutingGroup` - groups nest, carry their own middleware, and are merged when two servers share an
address.

```go
cadre.WithHTTP(
	"main_http",
	cadre.WithHTTPListeningAddress(":8000"),
	cadre.WithRoutingGroup(http.RoutingGroup{
		Base: "/api",
		Groups: []http.RoutingGroup{
			{
				Base:       "/v1",
				Middleware: []gin.HandlerFunc{requireAPIKey}, // guards this group only
				Routes: map[string]map[string][]gin.HandlerFunc{
					"/orders":     {"GET": {listOrders}, "POST": {createOrder}},
					"/orders/:id": {"GET": {getOrder}},
				},
			},
		},
		Static: []http.StaticRoute{
			{Path: "/assets", Root: "./public"},   // or FS: embeddedFS
		},
	}),
)
```

Notes:

- registering `GET` for a path automatically registers `HEAD` with the same handlers;
- unmatched requests get a JSON `404` with error type `NO_ROUTE`;
- `WithGlobalMiddleware` adds middleware to the whole server, after metrics, logging and recovery;
- a `StaticRoute` must set exactly one of `Root` and `FS`.

## Responses and errors

`http/responses` writes a consistent JSON envelope. Successes are wrapped in `data`, failures in `errors`:

```go
responses.Ok(c, order)                       // 200 {"data": {...}}
responses.OkWithMeta(c, orders, pagination)  // 200 {"data": [...], "metadata": {...}}
responses.Created(c, order)                  // 201
responses.BadRequest(c, responses.NewError(err))
responses.Unauthorized(c, responses.Error{Type: "MISSING_API_KEY", Message: "X-Api-Key is required"})
```

Available helpers: `Ok`, `OkWithMeta`, `Created`, `BadRequest`, `CannotBind`, `Unauthorized`, `Forbidden`,
`NotFound`, `Timeout`, `Conflict`, `InternalError`, `Unavailable`.

Rather than mapping errors to status codes in every handler, tag them in the domain layer with one of the
sentinel types from `cadre/errors` and let `responses.FromError` translate:

```go
import cerrors "github.com/moderntv/cadre/errors"

func (r *repository) Find(id string) (*Order, error) {
	// ...
	return nil, cerrors.NewTyped(cerrors.ErrNotFound, fmt.Errorf("order %q does not exist", id))
}

func getOrder(c *gin.Context) {
	order, err := repo.Find(c.Param("id"))
	if err != nil {
		responses.FromError(c, err) // -> 404
		return
	}

	responses.Ok(c, order)
}
```

| Sentinel | Status |
| --- | --- |
| `errors.ErrInvalidInput` | 400 |
| `errors.ErrNotAllowed` | 403 |
| `errors.ErrNotFound` | 404 |
| `errors.ErrTemporaryUnavailable` | 503 |
| `errors.ErrInternalError` | 500 |
| anything else | 500 |

`NewTyped` keeps the cause as the error message and exposes the sentinel through `Unwrap`, so `errors.Is`
works on it.

## Status

`/status` reports one entry per registered component. The overall status is the worst component status
(`OK` < `WARN` < `ERROR`), and the endpoint answers **503 once any component is `ERROR`**, which makes it
usable directly as a readiness probe.

```go
appStatus := status.NewStatus("1.0.0") // version reported in the response

dbStatus, err := appStatus.Register("database")
if err != nil {
	return err
}

// components start out ERROR/"uninitialized" until something reports on them
dbStatus.SetStatus(status.OK, "connected")

b, err := cadre.NewBuilder("example", cadre.WithStatus(appStatus), /* ... */)
```

```json
{
  "data": {
    "version": "1.0.0",
    "hostname": "app-01",
    "status": "OK",
    "components": {
      "database": { "status": "OK", "message": "connected", "updated_at": "2026-08-20T12:54:21+02:00" }
    }
  }
}
```

Use `RegisterOrGet` when several call sites may register the same component. If the gRPC health service is
enabled, Cadre polls the status every 5 seconds and moves the health server between serving and not-serving
to match.

## Metrics

`metrics.Registry` wraps a Prometheus registry and additionally keys every collector by a name of your
choosing, so collectors can be looked up later instead of being threaded through the application.

```go
metricsRegistry, err := metrics.NewRegistry("myservice", nil) // namespace, existing *prometheus.Registry
if err != nil {
	return err
}

ordersServed, err := metricsRegistry.RegisterNewCounterVec(
	"orders_served",
	prometheus.CounterOpts{Subsystem: "orders", Name: "served_total", Help: "Number of orders served"},
	[]string{"result"},
)

b, err := cadre.NewBuilder("myservice", cadre.WithMetricsRegistry(metricsRegistry), /* ... */)
```

There are `New*`, `RegisterNew*` and `RegisterOrGetNew*` variants for `Counter`, `CounterVec`, `Gauge`,
`GaugeVec`, `Histogram`, `HistogramVec` and `SummaryVec`. Go runtime and process collectors are registered
automatically.

Pass either `WithMetricsRegistry` or `WithPrometheusRegistry`, never both - `Build()` rejects that.

Every HTTP server records request counts and durations. By default they are labelled with the raw request
path; `cadre.WithMetricsAggregation()` labels them with the gin route template (`/orders/:id`) instead,
which is what you want whenever paths contain identifiers.

## HTTP request logging

Every HTTP server gets a logging middleware which logs one entry per handled request with the following fields:

| Field                                      | Note                                                                                               |
| ------------------------------------------ | -------------------------------------------------------------------------------------------------- |
| `method`, `path`, `route`, `host`, `proto` | `route` is the gin route template (`/users/:id`), useful for aggregation                           |
| `status_code`, `latency`, `latency_str`    |                                                                                                    |
| `request_size`, `response_size`            | `request_size` is omitted for requests with an unknown length                                      |
| `ip`, `user_agent`, `referer`              |                                                                                                    |
| `query`                                    | values of sensitive parameters (`token`, `password`, ...) are redacted                             |
| `request_id`                               | first match of `X-Request-Id`/`X-Correlation-Id`, falling back to the `request_id` gin context key |
| `errors`                                   | errors collected by the handlers into `gin.Context`                                                |

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
  Per-server options are applied last and therefore win.
- an invalid ignore path pattern is reported as an error from `b.Build()`.
- `cadre.WithoutLoggingMiddleware()` and `cadre.WithoutMetricsMiddleware()` disable the middleware per server.

## gRPC server

Services are registered through a registrator function, which keeps Cadre independent of your generated code:

```go
cadre.WithGRPC(
	cadre.WithGRPCListeningAddress(":9000"),
	cadre.WithService("example.GreeterService", func(s *grpc.Server) {
		greeterpb.RegisterGreeterServiceServer(s, greeterSvc)
	}),
)
```

Enabled by default: the health service, server reflection, zerolog request logging, panic recovery and
Prometheus metrics. Turn them off with `WithoutReflection()`, `WithoutLogging()` and `WithoutRecovery()`, or
configure them with `WithLoggingOptions()` and `WithRecoveryOptions()`.

Interceptors are chained in a fixed order:

```
ctxtags -> logging -> metrics -> your interceptors -> recovery
```

`WithUnaryInterceptors()` and `WithStreamInterceptors()` append to the "your interceptors" slot, so recovery
stays outermost and always catches panics from your code.

`WithChannelz(":8192")` exposes [channelz](https://github.com/rantav/go-grpc-channelz) on its own HTTP
server for connection-level debugging.

## Graceful shutdown

`Start()` blocks until the process receives `SIGINT` or `SIGTERM` (pass your own list as extra arguments to
`WithFinisher`), then cancels the internal context. Each HTTP server is `Shutdown()` and the gRPC server is `GracefulStop()`ed; `Start()` returns once
they have all stopped.

```go
cadre.WithFinisher(func(sig os.Signal) {
	logger.Info().Str("signal", sig.String()).Msg("draining")
	dbStatus.SetStatus(status.WARN, "shutting down")
	// stop accepting work, flush queues, ...
})
```

The finisher runs *before* the servers are torn down, so it is the place to drain in-flight work or flip
the status to `WARN`/`ERROR` and let load balancers take the instance out of rotation. A third signal
finishes the process regardless of whether the callback has returned.

`WithContext(ctx)` ties the whole server to a context you own, and `Shutdown()` triggers the same path
programmatically.

## Service registry

`registry.Registry` abstracts service discovery and doubles as a gRPC resolver, so client connections follow
the registry:

```go
import (
	"github.com/moderntv/cadre/registry"
	registry_file "github.com/moderntv/cadre/registry/file"
)

r, err := registry_file.NewRegistry("./registry.yaml", registry_file.WithWatch())
if err != nil {
	return err
}

resolver.Register(registry.NewResolverBuilder(r))

cc, err := grpc.NewClient(
	"registry:///aggregator", // registry://<authority>/<service name>
	grpc.WithTransportCredentials(insecure.NewCredentials()),
)
```

Backends:

| Backend | Constructor | Notes |
| --- | --- | --- |
| `registry/static` | `static.NewRegistry(map[string][]string{...})` | fixed addresses, no watching |
| `registry/file` | `file.NewRegistry(path, file.WithWatch())` | YAML, optionally reloaded on change |
| `registry/consul` | `consul.NewRegistry(addr, dc, aliases, refreshPeriod)` | polls the Consul catalog |

The file backend expects a service-name-to-addresses mapping:

```yaml
---
aggregator:
  - aggregator1.moderntv.eu
  - aggregator2.moderntv.eu

ingest:
  - ingest.moderntv.eu
```

`Watch(service)` returns a channel of `RegistryChange` values plus a cancel function. Note that the `file`
and `consul` backends are read-only - `Register`/`Deregister` are not supported.

## Shard load balancer

`lb/shard` is a consistent-hashing gRPC balancer built on [hashring](https://github.com/moderntv/hashring).
It routes every call to the instance owning the call's shard key, which keeps requests for the same entity
on the same backend. Importing the package registers the balancer under the name `shard`.

```go
import (
	"github.com/moderntv/cadre/lb/shard"
)

cc, err := grpc.NewClient(
	"registry:///aggregator",
	grpc.WithTransportCredentials(insecure.NewCredentials()),
	grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"shard": {}}]}`),
)

// the key is read from the call context
ctx = context.WithValue(ctx, shard.DefaultShardKeyName, channelID)
res, err := client.GetChannel(ctx, req)
```

If the key is missing from the context the balancer falls back to the literal string `NOT_FOUND`, which
sends every such call to one instance - make sure the key is always set.

The package exposes a `WithShardKeyFunc` option for reading the key from somewhere else (gRPC metadata, for
example), but `NewBuilder`/`NewNamedBuilder` do not currently accept options, so the context lookup above is
the only supported behaviour.

## Configuration

`config.Manager` loads a struct from an ordered list of sources - later sources overwrite earlier ones - and
can notify you when a source changes.

```go
import (
	"github.com/moderntv/cadre/config"
	"github.com/moderntv/cadre/config/encoder/yaml"
	"github.com/moderntv/cadre/config/source/file"
)

type Config struct {
	Addr string `yaml:"addr"`
}

// PostLoad satisfies config.Config. Manager.Load takes an `any` and does not call it for you - invoke it
// yourself after loading.
func (c *Config) PostLoad() error { return nil }

src, err := file.NewSource("./config.yaml", yaml.NewEncoder())
if err != nil {
	return err
}

manager, err := config.NewManager(config.WithSource(src))
if err != nil {
	return err
}

var cfg Config

err = manager.Load(&cfg)
if err != nil {
	return err
}

err = cfg.PostLoad()
if err != nil {
	return err
}

changes, err := manager.Subscribe() // chan source.ConfigChange
```

Encoders live in `config/encoder/{json,yaml}`; implement `source.Source` to add another backend.

## Examples

Runnable examples live in [`examples/`](./examples), a separate module wired to the parent with a `replace`
directive. Each subdirectory is one command:

| Example | What it shows |
| --- | --- |
| [`httponly`](./examples/httponly) | single HTTP server, internal endpoints merged into it |
| [`grpconly`](./examples/grpconly) | gRPC only, with reflection and health checking |
| [`http-grpc`](./examples/http-grpc) | HTTP and gRPC on separate ports, internal endpoints on a third |
| [`http-grpc-multiplex`](./examples/http-grpc-multiplex) | both protocols on one port |
| [`separate-metrics-status`](./examples/separate-metrics-status) | placing `/metrics` and `/status` |
| [`full`](./examples/full) | status components, custom metrics, nested routing groups, typed errors, body logging, graceful shutdown |
| [`cli`](./examples/cli) | a gRPC client for the greeter service used by the examples above |

```bash
cd examples
go run ./httponly
go run ./cli -name Alice   # against grpconly, http-grpc or http-grpc-multiplex
```

Some examples listen on `:7000`. On macOS that port is taken by the AirPlay receiver, which you can turn off
in *System Settings > General > AirDrop & Handoff*.

## Disclaimer

Cadre is not production ready. It is under heavy development and its API can be changed at any time.

## Why Cadre?

[Cadre](https://www.wordnik.com/words/cadre)
