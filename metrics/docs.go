// Package metrics wraps a Prometheus registry and additionally keys every collector by a name of your
// choosing, so that collectors can be looked up again later instead of being threaded through the
// application.
//
//	registry, err := metrics.NewRegistry("myservice", nil) // namespace, existing *prometheus.Registry
//	if err != nil {
//		return err
//	}
//
//	served, err := registry.RegisterNewCounterVec(
//		"orders_served",
//		prometheus.CounterOpts{Subsystem: "orders", Name: "served_total", Help: "Number of orders served"},
//		[]string{"result"},
//	)
//
// [NewRegistry] creates a Prometheus registry when none is passed in, and always registers the Go runtime
// and process collectors. Pass an existing one to share it with code that does not go through this package.
//
// # Collector helpers
//
// For each of Counter, CounterVec, Gauge, GaugeVec, Histogram, HistogramVec and SummaryVec there are three
// helpers. The New* form only constructs the collector, the RegisterNew* form also registers it under a
// name, and the RegisterOrGetNew* form returns the collector already registered under that name if there is
// one - useful when several call sites may set up the same metric.
//
// [Registry.Register] reports [ErrMetricAlreadyExists] for a duplicate name, and [Registry.Get] reports
// [ErrMetricNotFound] for an unknown one. [Registry.Get] returns a prometheus.Collector, so the caller has
// to assert it back to the concrete type.
//
// Cadre serves [Registry.HTTPHandler] on /metrics. A registry can be handed to the builder with
// cadre.WithMetricsRegistry; pass either that or cadre.WithPrometheusRegistry, never both.
package metrics
