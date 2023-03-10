package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
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

	metrics sync.Map
}

func NewRegistry(namespace string, prometheusRegistry *prometheus.Registry) (registry *Registry, err error) {
	if prometheusRegistry == nil {
		prometheusRegistry = prometheus.NewRegistry()
	}

	registry = &Registry{
		namespace:          namespace,
		prometheusRegistry: prometheusRegistry,
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

func (registry *Registry) Register(name string, c prometheus.Collector) (err error) {
	if name == "" {
		err = ErrNameEmpty
		return
	}
	if c == nil {
		err = ErrMetricNil
		return
	}
	if _, ok := registry.metrics.Load(name); ok {
		err = ErrMetricAlreadyExists
		return
	}

	err = registry.prometheusRegistry.Register(c)
	if err != nil {
		return
	}
	registry.metrics.Store(name, c)

	return
}

func (registry *Registry) RegisterOrGet(name string, c prometheus.Collector) (cRegistered prometheus.Collector, err error) {
	cRegistered, err = registry.Get(name)
	if err == nil {
		return
	}
	if err != nil && err != ErrMetricNotFound {
		return
	}
	// err = ErrMetricNotFound

	err = registry.Register(name, c)
	if err != nil {
		return
	}

	return c, nil
}

func (registry *Registry) Unregister(name string) (err error) {
	v, ok := registry.metrics.Load(name)
	if !ok {
		err = ErrMetricNotFound
		return
	}
	c := v.(prometheus.Collector)

	registry.prometheusRegistry.Unregister(c) // ignore return value - it only tells us the collector doesn't exist in prometheus registry
	registry.metrics.Delete(name)

	return
}

func (registry *Registry) Get(name string) (c prometheus.Collector, err error) {
	v, ok := registry.metrics.Load(name)
	if !ok {
		err = ErrMetricNotFound
	}
	c = v.(prometheus.Collector)

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
