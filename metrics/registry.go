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

// Registry represents common registry that is created with prometheus.Registry and metrics that are
// gathered using prometheus Collectors.
type Registry struct {
	namespace string

	prometheusRegistry *prometheus.Registry

	metrics map[string]prometheus.Collector
}

// NewRegistry registers new registry with given namespace and prometheusRegistry. When prometheusRegistry not provided, it creates new registry.
// Default collectors for each go program are GoCollector (goroutines, version etc.) and ProcessCollector (CPU, VMEM, MEM).
func NewRegistry(namespace string, prometheusRegistry *prometheus.Registry) (registry *Registry, err error) {
	if prometheusRegistry == nil {
		prometheusRegistry = prometheus.NewRegistry()
	}

	registry = &Registry{
		namespace:          namespace,
		prometheusRegistry: prometheusRegistry,
		metrics:            map[string]prometheus.Collector{},
	}

	err = registry.Register("go", collectors.NewGoCollector())
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

// Register registrates new prometheus collector into registry.
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

// RegisterOrGet registers new prometheus collector. When already exists, return the collector instead.
func (registry *Registry) RegisterOrGet(name string, c prometheus.Collector) (cRegistered prometheus.Collector, err error) {
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

// Unregister deregisters collector with given name.
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

// Get returns collector with given name.
func (registry *Registry) Get(name string) (c prometheus.Collector, err error) {
	var ok bool

	c, ok = registry.metrics[name]
	if !ok {
		err = ErrMetricNotFound
	}

	return
}

// HTTPHandler returns handler for prometheus registry.
func (registry *Registry) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(registry.prometheusRegistry, promhttp.HandlerOpts{
		Timeout: 1 * time.Second,
	})
}

// GetPrometheusRegistry returns prometheus registry.
func (registry *Registry) GetPrometheusRegistry() *prometheus.Registry {
	return registry.prometheusRegistry
}
