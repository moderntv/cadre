package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ErrNameEmpty           = errors.New("metric name cannot be empty")
	ErrMetricNil           = errors.New("metric cannot be nil")
	ErrMetricAlreadyExists = errors.New("metric already exists")
	ErrMetricNotFound      = errors.New("metric not found")
)

type Registry struct {
	namespace string

	prometheusRegistry *prometheus.Registry

	metrics map[string]prometheus.Collector
}

func NewRegistry(namespace string, prometheusRegistry *prometheus.Registry) (registry *Registry, err error) {
	if prometheusRegistry == nil {
		prometheusRegistry = prometheus.NewRegistry()
	}

	registry = &Registry{
		namespace:          namespace,
		prometheusRegistry: prometheusRegistry,
		metrics:            map[string]prometheus.Collector{},
	}

	err = registry.Register("go", prometheus.NewGoCollector())
	if err != nil {
		err = fmt.Errorf("cannot register go collector: %w", err)
		return
	}
	err = registry.Register("process", prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
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

func (registry *Registry) Unregister(name string) (err error) {
	c, ok := registry.metrics[name]
	if !ok {
		err = ErrMetricNotFound
		return
	}

	registry.prometheusRegistry.Unregister(c) // ignore return value - it only tells us the collector doesn't exist in prometheus registry
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
