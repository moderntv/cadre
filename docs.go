// Package cadre removes the boilerplate from a Go service that exposes an HTTP and/or a gRPC interface.
//
// A Cadre application is assembled with a builder and started as a single process:
//
//	b, err := cadre.NewBuilder("my-service",
//		cadre.WithLogger(logger),
//		cadre.WithHTTP("main_http", cadre.WithHTTPListeningAddress(":8000")),
//	)
//	c, err := b.Build()
//	err = c.Start()
//
// # Option scopes
//
// Configuration comes in three levels, and they are not interchangeable. [Option] values configure the
// application as a whole and are passed to [NewBuilder]. [HTTPOption] values configure one HTTP server and
// are passed to [WithHTTP]. [GRPCOption] values configure the gRPC server and are passed to [WithGRPC].
//
// Options only record intent - nothing is validated or constructed until [Builder.Build], so configuration
// mistakes are reported from there rather than from the option call itself.
//
// # Servers
//
// An application may run any number of HTTP servers but at most one gRPC server. HTTP servers are
// identified by their listening address: two servers configured on the same address are merged into one,
// combining their routes, middleware and logger options. Registering the same path and method twice across
// merged servers is a [Builder.Build] error.
//
// That merging is how the internal endpoints are placed. The Prometheus endpoint (/metrics) and the status
// endpoint (/status) default to the address of the first configured HTTP server, so a plain HTTP
// application serves them alongside its own routes. [WithMetricsListeningAddress] and
// [WithStatusListeningAddress] move them onto their own internal server, which is what you want when the
// main server is exposed publicly.
//
// [WithGRPCMultiplex] drops the standalone gRPC listener and serves both protocols on the HTTP server's
// address, dispatching each request by protocol: HTTP/2 requests carrying an "application/grpc" content
// type reach the gRPC server, everything else reaches gin. It is mutually exclusive with
// [WithGRPCListeningAddress].
//
// # Middleware
//
// Each HTTP server is wrapped in metrics, logging and panic recovery middleware, in that order, followed by
// any middleware added with [WithGlobalMiddleware]. Logger options accumulate: those given to
// [WithHTTPLoggerOptions] apply to every server, and those given to the per-server [WithLoggerOptions] are
// applied afterwards and therefore win.
//
// gRPC interceptors are chained as ctxtags, logging, metrics, the interceptors added with
// [WithUnaryInterceptors] and [WithStreamInterceptors], and finally recovery. Recovery stays outermost so
// that it always catches panics raised by service code.
//
// # Lifecycle
//
// [Cadre.Start] runs every configured server and blocks until the process is signalled - SIGINT and SIGTERM
// unless [WithFinisher] overrides the list - or until the context given to [WithContext] is cancelled. The
// callback registered with [WithFinisher] runs first, giving the application a chance to drain in-flight
// work or flip its status, after which each HTTP server is shut down and the gRPC server is stopped
// gracefully. Start returns once they have all stopped. A third signal finishes the process regardless of
// whether the callback has returned.
//
// Sub-packages provide the individual building blocks: routing and response helpers in
// [github.com/moderntv/cadre/http], component health in [github.com/moderntv/cadre/status], named
// Prometheus collectors in [github.com/moderntv/cadre/metrics], service discovery in
// [github.com/moderntv/cadre/registry] and layered configuration in
// [github.com/moderntv/cadre/config].
package cadre
