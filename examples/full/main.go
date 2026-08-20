// Command full wires together the pieces a real service usually needs: a pre-built status and metrics
// registry, nested routing groups with per-group middleware, typed domain errors mapped onto HTTP
// responses, request/response body logging, and a graceful shutdown hook.
//
//	curl localhost:8000/api/v1/orders/1        -> 200
//	curl localhost:8000/api/v1/orders/404      -> 404, mapped from errors.ErrNotFound
//	curl localhost:8000/api/v1/orders/boom     -> 400, mapped from errors.ErrInvalidInput
//	curl localhost:7000/status                 -> component health report
//	curl localhost:7000/metrics                -> Prometheus exposition
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moderntv/cadre"
	cerrors "github.com/moderntv/cadre/errors"
	"github.com/moderntv/cadre/http"
	"github.com/moderntv/cadre/http/middleware"
	"github.com/moderntv/cadre/http/responses"
	"github.com/moderntv/cadre/metrics"
	"github.com/moderntv/cadre/status"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

const version = "1.0.0"

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	// Status - every component reports into the shared status, and the worst one wins. The /status endpoint
	// answers 503 as soon as any component is ERROR, which makes it usable as a readiness probe.
	appStatus := status.NewStatus(version)

	dbStatus, err := appStatus.Register("database")
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot register database status component")
	}

	dbStatus.SetStatus(status.OK, "connected")

	// Metrics - a Cadre registry wraps a Prometheus one and keys collectors by a name of your choosing, so
	// they can be looked up again later instead of being passed around.
	metricsRegistry, err := metrics.NewRegistry("example", nil)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot create metrics registry")
	}

	ordersServed, err := metricsRegistry.RegisterNewCounterVec(
		"orders_served",
		prometheus.CounterOpts{
			Subsystem: "orders",
			Name:      "served_total",
			Help:      "Number of orders served",
		},
		[]string{"result"},
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot register orders metric")
	}

	b, err := cadre.NewBuilder(
		"example",
		cadre.WithLogger(logger),
		cadre.WithStatus(appStatus),
		cadre.WithMetricsRegistry(metricsRegistry),

		// Keep the internal endpoints off the public port.
		cadre.WithMetricsListeningAddress(":7000"),
		cadre.WithStatusListeningAddress(":7000"),

		// Applies to every HTTP server, including the internal one.
		cadre.WithHTTPLoggerOptions(
			middleware.WithLoggingIgnorePaths(`^/metrics$`, `^/status$`),
			middleware.WithRequestBody(middleware.BodyLogOnError),
			middleware.WithResponseBody(middleware.BodyLogOnError),
		),

		// Called on SIGINT/SIGTERM before the servers are torn down.
		cadre.WithFinisher(func(sig os.Signal) {
			logger.Info().Str("signal", sig.String()).Msg("shutting down, draining work")
			dbStatus.SetStatus(status.WARN, "shutting down")
			time.Sleep(100 * time.Millisecond)
		}),

		cadre.WithHTTP(
			"main_http",
			cadre.WithHTTPListeningAddress(":8000"),
			// Label metrics by route template (/orders/:id) instead of the concrete path.
			cadre.WithMetricsAggregation(),
			cadre.WithRoutingGroup(http.RoutingGroup{
				Base: "/api",
				Groups: []http.RoutingGroup{
					{
						Base:       "/v1",
						Middleware: []gin.HandlerFunc{requireAPIKey},
						Routes: map[string]map[string][]gin.HandlerFunc{
							"/orders/:id": {
								"GET": {getOrder(ordersServed)},
							},
						},
					},
				},
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

	// Start blocks until the process is signalled (or Shutdown is called) and every server has stopped.
	err = c.Start()
	if err != nil {
		logger.Fatal().Err(err).Msg("cadre failed")
	}
}

// requireAPIKey is a per-group middleware - it only guards the routes of the group it is attached to.
func requireAPIKey(c *gin.Context) {
	if c.GetHeader("X-Api-Key") == "" {
		// The Authorization and X-Api-Key headers are redacted by the logging middleware by default.
		responses.Unauthorized(c, responses.Error{
			Type:    "MISSING_API_KEY",
			Message: "X-Api-Key header is required",
		})

		return
	}

	c.Next()
}

func getOrder(served *prometheus.CounterVec) gin.HandlerFunc {
	return func(c *gin.Context) {
		order, err := findOrder(c.Param("id"))
		if err != nil {
			served.WithLabelValues("error").Inc()

			// FromError unwraps the typed error and picks the matching status code - no mapping in handlers.
			responses.FromError(c, err)

			return
		}

		served.WithLabelValues("ok").Inc()
		responses.Ok(c, order)
	}
}

type order struct {
	ID string `json:"id"`
}

// findOrder stands in for a repository. Domain errors are wrapped in a Cadre error type so that the
// transport layer can map them without knowing anything about the domain.
func findOrder(id string) (*order, error) {
	switch id {
	case "boom":
		return nil, cerrors.NewTyped(cerrors.ErrInvalidInput, fmt.Errorf("order id %q is not numeric", id))
	case "404":
		return nil, cerrors.NewTyped(cerrors.ErrNotFound, fmt.Errorf("order %q does not exist", id))
	default:
		return &order{ID: id}, nil
	}
}
