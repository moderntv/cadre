package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ErrNameEmpty           = errors.New("metric name cannot be empty")
	ErrMetricNil           = errors.New("metric cannot be nil")
	ErrMetricAlreadyExists = errors.New("metric already exists")
	ErrMetricNotFound      = errors.New("metric not found")
	ErrInvalidType         = errors.New("metric of invalid type found")
)

type Registry struct {
	namespace string

	prometheusRegistry *prometheus.Registry

	metrics map[string]prometheus.Collector
}

type registryOptions struct {
	goCollector prometheus.Collector
}

type Option func(*registryOptions) error

// WithGoCollector replaces the default Go runtime collector registered under the name "go". Use it to expose
// runtime/metrics that collectors.NewGoCollector leaves out by default:
//
//	metrics.WithGoCollector(collectors.NewGoCollector(
//		collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{
//			Matcher: regexp.MustCompile(`^/cpu/classes/`),
//		}),
//	))
func WithGoCollector(c prometheus.Collector) Option {
	return func(options *registryOptions) error {
		if c == nil {
			return ErrMetricNil
		}

		options.goCollector = c

		return nil
	}
}

func NewRegistry(
	namespace string,
	prometheusRegistry *prometheus.Registry,
	opts ...Option,
) (registry *Registry, err error) {
	options := &registryOptions{}
	for _, opt := range opts {
		err = opt(options)
		if err != nil {
			return
		}
	}

	if options.goCollector == nil {
		options.goCollector = collectors.NewGoCollector()
	}

	if prometheusRegistry == nil {
		prometheusRegistry = prometheus.NewRegistry()
	}

	registry = &Registry{
		namespace:          namespace,
		prometheusRegistry: prometheusRegistry,
		metrics:            map[string]prometheus.Collector{},
	}

	err = registry.Register("go", options.goCollector)
	if err != nil {
		err = fmt.Errorf("cannot register go collector: %w", err)
		return
	}

	err = registry.Register("process", collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if err != nil {
		err = fmt.Errorf("cannot register process collector: %w", err)
		return
	}

	return
}

func (registry *Registry) Register(name string, c prometheus.Collector) (err error) {
	if name == "" {
		err = ErrNameEmpty
		return
	}

	if c == nil {
		err = ErrMetricNil
		return
	}

	if _, ok := registry.metrics[name]; ok {
		err = ErrMetricAlreadyExists
		return
	}

	err = registry.prometheusRegistry.Register(c)
	if err != nil {
		return
	}

	registry.metrics[name] = c

	return
}

func (registry *Registry) RegisterOrGet(
	name string,
	c prometheus.Collector,
) (cRegistered prometheus.Collector, err error) {
	cRegistered, err = registry.Get(name)
	if err == nil {
		return
	}

	if !errors.Is(err, ErrMetricNotFound) {
		return
	}

	err = registry.Register(name, c)
	if err != nil {
		return
	}

	return c, nil
}

func (registry *Registry) Unregister(name string) (err error) {
	c, ok := registry.metrics[name]
	if !ok {
		err = ErrMetricNotFound
		return
	}

	registry.prometheusRegistry.Unregister(
		c,
	) // ignore return value - it only tells us the collector doesn't exist in prometheus registry
	delete(registry.metrics, name)

	return
}

func (registry *Registry) Get(name string) (c prometheus.Collector, err error) {
	var ok bool

	c, ok = registry.metrics[name]
	if !ok {
		err = ErrMetricNotFound
	}

	return
}

func (registry *Registry) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(registry.prometheusRegistry, promhttp.HandlerOpts{
		Timeout: 1 * time.Second,
	})
}

func (registry *Registry) GetPrometheusRegistry() *prometheus.Registry {
	return registry.prometheusRegistry
}
