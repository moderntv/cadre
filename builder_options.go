package cadre

import (
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

// WithLogger allows configuring cadre with custom zerolog logger.
// If not used Cadre will be configured with zerolog.Nop()
func WithLogger(logger *zerolog.Logger) Option {
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

// WithMetrics allows replacing the default metrics registry with a custom pre-configured one.
// If used, the prometheus registry from metrics Registry will replace the default prometheus registry.
// Do not use with WithPrometheus
func WithMetrics(metrics *metrics.Registry) Option {
	return func(options *Builder) error {
		options.metrics = metrics

		return nil
	}
}

// WithPrometheus configures cadre to use a specific prometheus registry.
// This prometheus registry will be used to create metrics registry.
func WithPrometheus(registry *prometheus.Registry) Option {
	return func(options *Builder) error {
		options.prometheusRegistry = registry

		return nil
	}
}

// WithPrometheusListeningAddress is meant to configure cadre to use
// a separate http server for prometheus - useful for putting it behind firewall
func WithPrometheusListeningAddress(serverListeningAddress string) Option {
	return func(options *Builder) error {
		options.prometheusHttpServerAddr = serverListeningAddress

		return nil
	}
}
