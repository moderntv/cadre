package cadre

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"golang.org/x/net/context"

	"github.com/moderntv/cadre/metrics"
	"github.com/moderntv/cadre/status"
)

type Option func(*Builder) error

// WithContext supplies custom context for the cadre server. This is useful for graceful shutdown
func WithContext(ctx context.Context) Option {
	return func(options *Builder) error {
		options.ctx = ctx

		return nil
	}
}

// WithFinisher adds a callback to be called for various signals (SIGINT, SIGTERM by default) which can be optionally set
func WithFinisher(cb Finisher, handledSigs ...os.Signal) Option {
	return func(options *Builder) error {
		options.finisherCallback = cb
		if len(handledSigs) > 0 {
			options.handledSigs = handledSigs
		}

		return nil
	}
}

// WithLogger allows configuring cadre with custom zerolog logger.
// If not used Cadre will be configured with zerolog.Nop()
func WithLogger(logger zerolog.Logger) Option {
	return func(options *Builder) error {
		options.logger = logger

		return nil
	}
}

// WithStatus allows replacing the default status with a custom pre-configured one
func WithStatus(status *status.Status) Option {
	return func(options *Builder) error {
		options.status = status

		return nil
	}
}

// WithStatusListeningAddress is meant to configure cadre to use
// a separate http server for status endpoint - useful for putting it behind firewall
// Default is to use the first HTTP server's listening address merging them together. This may cause problems
// when a conflicting route is configured
func WithStatusListeningAddress(serverListeningAddress string) Option {
	return func(options *Builder) error {
		options.statusHTTPServerAddr = serverListeningAddress

		return nil
	}
}

// WithMetricsRegistry allows replacing the default metrics registry with a custom pre-configured one.
// If used, the prometheus registry from metrics Registry will replace the default prometheus registry.
// Do not use with WithPrometheusRegistry
func WithMetricsRegistry(metrics *metrics.Registry) Option {
	return func(options *Builder) error {
		options.metrics = metrics

		return nil
	}
}

// WithPrometheusRegistry configures cadre to use a specific prometheus registry.
// This prometheus registry will be used to create metrics registry.
func WithPrometheusRegistry(registry *prometheus.Registry) Option {
	return func(options *Builder) error {
		options.prometheusRegistry = registry

		return nil
	}
}

// WithPrometheusListeningAddress is meant to configure cadre to use
// a separate http server for prometheus - useful for putting it behind firewall
// Default is to use the first HTTP server's listening address merging them together. This may cause problems
// when a conflicting route is configured
func WithMetricsListeningAddress(serverListeningAddress string) Option {
	return func(options *Builder) error {
		options.metricsHTTPServerAddr = serverListeningAddress

		return nil
	}
}
