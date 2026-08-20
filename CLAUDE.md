# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

Cadre (`github.com/moderntv/cadre`) is a public, standalone Go library — a boilerplate-removal framework for
ModernTV services that need gRPC and/or HTTP interfaces. It is a git repository of its own nested inside the
ModernTV monorepo checkout; the monorepo-level `CLAUDE.md` (Makefile `.mks/` includes, Nomad deploys, Debian
packaging, DDD project layout) does **not** apply here. Cadre has no `cmd/`, no services, no deployment.

The API is explicitly unstable — the README states Cadre is not production ready and its API can change at
any time. Breaking changes are acceptable, but must be marked with `!` in the commit type
(`feat(middleware)!: ...`).

## Commands

```bash
make test    # go test -race -timeout 3m -coverprofile cp.out ./...
make lint    # golangci-lint v2, run via `go run` (no separate install)
```

Single test:

```bash
go test -race -run '^TestNewLogger$' ./http/middleware/...
```

`make lint` writes `golangci-lint.out` / `golangci-lint.out.html` and has `issues.fix: true`, so it rewrites
files in place. Both targets are what CI (`.github/workflows/ci.yml`, runs on every push) executes.

Regenerating the example protobufs (rarely needed) is `make proto` inside `examples/`; it needs `protoc`
plus `protoc-gen-go` and `protoc-gen-go-grpc`.

## Architecture

### Builder → cadre

Everything is assembled through a functional-options builder and started as one process:

```
cadre.NewBuilder(name, ...Option) → *Builder → b.Build() → *cadre → c.Start() / c.Shutdown()
```

Three option layers, each with its own file:

| File | Scope | Type |
|------|-------|------|
| `builder_options.go` | app-wide (context, logger, status, metrics, finisher) | `Option` — `func(*Builder) error` |
| `builder_http.go` | one HTTP server (`cadre.WithHTTP(name, ...)`) | `HTTPOption` — `func(*httpOptions) error` |
| `builder_grpc.go` | the single gRPC server (`cadre.WithGRPC(...)`) | `GRPCOption` — `func(*grpcOptions) error` |

Options only mutate the builder — nothing is validated or constructed until `Build()`. Validation lives in
`Builder.ensure()` and the per-server `ensure()` methods, so **new configuration errors belong in `ensure()`,
not in the option closure** (option closures only reject things knowable in isolation, like double
registration).

Key `Build()`/`ensure()` behaviours that are easy to break:

- **Servers are keyed and merged by listening address.** `buildHTTP` merges every `httpOptions` sharing an
  address via `httpOptions.merge`, then merges their routing groups; a duplicate path+method across merged
  servers is a `Build()` error. This is how metrics/status/channelz endpoints get folded into the user's
  main HTTP server.
- **Metrics and status default to the first HTTP server's address** (`b.httpOptions[0]`) unless
  `WithMetricsListeningAddress` / `WithStatusListeningAddress` give them their own. They are then registered
  as ordinary internal HTTP servers named `metrics_http` / `status_http` (also `channelz_http`).
- **gRPC/HTTP multiplexing** (`WithGRPCMultiplex`) sets the gRPC address to the first HTTP address, drops the
  gRPC listener (`c.grpcListener = nil`) and wraps the gin handler in a `http.HandlerFunc` that dispatches on
  `ProtoMajor == 2 && Content-Type: application/grpc`. Multiplex and an explicit gRPC listening address are
  mutually exclusive.
- Building with neither HTTP nor gRPC configured is an error.

### Runtime (`cadre.go`)

`Start()` spawns every HTTP server plus the gRPC server as goroutines tracked by a `sync.WaitGroup`, and
blocks on the cadre context or the finisher. Shutdown is context-cancellation driven: each server goroutine
watches `c.ctx.Done()` and calls `Shutdown()` / `GracefulStop()` itself. The signal handler runs the
`Finisher` callback once; a third signal force-finishes. When both the gRPC health service and status are
enabled, `healthServerCheck` polls the status report every 5s and resumes/shuts the gRPC health server.

### Cross-cutting middleware order

- **HTTP** (`httpOptions.build`): metrics → logging → `gin.Recovery()` → user global middleware.
  Logger options compose as `global (WithHTTPLoggerOptions) + per-server (WithLoggerOptions)` — the
  per-server ones come last and therefore win.
- **gRPC** (`Builder.buildGrpc`): ctxtags → zerolog logging → metrics → user interceptors → recovery.
  Recovery is deliberately last. gRPC middleware comes from the `rkollar/go-grpc-middleware` fork, not the
  upstream grpc-ecosystem one.

### Supporting packages

- `status/` — `Status` aggregates named `ComponentStatus` values (`OK`/`WARN`/`ERROR`, worst wins) into a
  `Report` served on `/status`; a report of `ERROR` makes the endpoint answer 503.
- `metrics/` — thin wrapper over a `prometheus.Registry` keyed by string name, with `RegisterOrGet` and
  typed `RegisterNew*Vec` helpers. Pass either `WithMetricsRegistry` or `WithPrometheusRegistry`, never both.
- `http/` — `HttpServer` wraps a gin engine; `RoutingGroup` is a declarative, recursively mergeable route
  tree (`Base` / `Middleware` / `Routes[path][method]` / `Groups` / `Static`). `WithRoute` is sugar for a
  single-route group. A registered `GET` automatically gets a matching `HEAD`.
- `http/responses/` — canonical JSON envelopes (`{"data": ...}` / `{"errors": [...]}`), plus
  `responses.FromError`, which maps the sentinel types in `errors/` (`ErrInvalidInput`, `ErrNotAllowed`,
  `ErrNotFound`, `ErrTemporaryUnavailable`, `ErrInternalError`) onto HTTP status codes. Domain code should
  wrap causes with `errors.NewTyped(typ, cause)` so `errors.Is` reaches the sentinel through `Unwrap`.
- `http/middleware/` — the request logger (see README for the full field list and body-logging policies) and
  the Prometheus metrics middleware. `WithMetricsAggregation` labels by `c.FullPath()` (the route template)
  instead of the raw path, to avoid cardinality blow-up.
- `registry/` — `Registry` interface (register/deregister/instances/watch) with `consul`, `file` and `static`
  backends, plus a gRPC `resolver.Builder` under the `registry://` scheme.
- `lb/shard/` — consistent-hashing gRPC balancer (`moderntv/hashring`) registered as balancer name `shard`;
  the shard key is read from the call context under `DefaultShardKeyName`.
- `config/` — `Manager` loads layered `source.Source`s into a struct implementing `Config` (with `PostLoad`)
  and can publish change notifications; `source/file` + `encoder/{json,yaml}` are the built-ins.

## Conventions

- **Named return values with naked-ish assignment** are the house style throughout
  (`func X() (y T, err error)` then `err = ...; return`). Match it in existing files.
- No inline assignment in `if` — assign first, then check.
- `depguard` runs in **strict allowlist mode**: only `$gostd`, `golang.org/x/net`, `git.moderntv.eu`,
  `github.com/moderntv`, and the explicitly listed third-party modules in `.golangci.yaml` may be imported.
  Adding a dependency means editing that allowlist too, and Cadre deliberately keeps the list short.
- Max line length 120 (golines); formatters `gci`, `gofmt`, `gofumpt`, `goimports`, `golines` all auto-fix.
- `default: all` linters with a curated `disable` list — check `.golangci.yaml` before adding `//nolint`.
- Tests use `t.Parallel()`, table-driven subtests, `testify` `assert`/`require`, and `httptest`.
- Conventional Commits; `.chglog/` generates the changelog from them, so scope and type matter.

## `examples/`

`examples/` is a **separate Go module** (`github.com/moderntv/cadre/examples`) with
`replace github.com/moderntv/cadre => ../`, so the root module's `./...` never reaches it and
`.golangci.yaml` excludes `examples$` as well. Each subdirectory is one runnable command
(`httponly`, `grpconly`, `http-grpc`, `http-grpc-multiplex`, `separate-metrics-status`, `full`, `cli`);
shared gRPC service implementations live in `examples/greeter_services.go` (`package examples`).

`go build ./...` and `go vet ./...` pass inside that module, so keep it that way - the README links to these
directories and every snippet in it is derived from code that compiles. When adding or changing a builder
option that alters topology, add or update the matching example. Regenerate the protobufs with
`make proto` (needs `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`).

Note that `:7000`, used by several examples for the internal metrics/status server, collides with the macOS
AirPlay receiver.
