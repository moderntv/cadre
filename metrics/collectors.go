package metrics

import "github.com/prometheus/client_golang/prometheus"

// NewSummaryVec creates new Summary Vector for Prometheus registry.
func (registry *Registry) NewSummaryVec(opts prometheus.SummaryOpts, labels []string) *prometheus.SummaryVec {
	opts.Namespace = registry.namespace

	return prometheus.NewSummaryVec(opts, labels)
}

// RegisterNewSummaryVec registers new Summary Vector for Prometheus registry.
func (registry *Registry) RegisterNewSummaryVec(name string, opts prometheus.SummaryOpts, labels []string) (c *prometheus.SummaryVec, err error) {
	c = registry.NewSummaryVec(opts, labels)
	err = registry.Register(name, c)

	return
}

// RegisterOrGetNewSummaryVec registers new Summary Vector.
// If a Summary Vec with the same name is already registered, it returns the Summary Vec instead.
func (registry *Registry) RegisterOrGetNewSummaryVec(name string, opts prometheus.SummaryOpts, labels []string) (c *prometheus.SummaryVec, err error) {
	c = registry.NewSummaryVec(opts, labels)
	cReturned, err := registry.RegisterOrGet(name, c)
	if err != nil {
		return
	}

	c, ok := cReturned.(*prometheus.SummaryVec)
	if !ok {
		err = ErrInvalidType
		return
	}

	return
}

// NewCounter creates new Prometheus Counter.
func (registry *Registry) NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	opts.Namespace = registry.namespace

	return prometheus.NewCounter(opts)
}

// RegisterNewCounter registers new Counter into Prometheus registry.
func (registry *Registry) RegisterNewCounter(name string, opts prometheus.CounterOpts) (c prometheus.Counter, err error) {
	c = registry.NewCounter(opts)
	err = registry.Register(name, c)

	return
}

// RegisterOrGetNewCounter registers new Counter into Prometheus registry.
// If a Counter with the same name is already registered, it returns the Counter instead.
func (registry *Registry) RegisterOrGetNewCounter(name string, opts prometheus.CounterOpts) (c prometheus.Counter, err error) {
	c = registry.NewCounter(opts)
	cReturned, err := registry.RegisterOrGet(name, c)
	if err != nil {
		return
	}

	c, ok := cReturned.(prometheus.Counter)
	if !ok {
		err = ErrInvalidType
		return
	}

	return
}

// NewCounterVec creates new Counter Vector.
func (registry *Registry) NewCounterVec(opts prometheus.CounterOpts, labels []string) *prometheus.CounterVec {
	opts.Namespace = registry.namespace

	return prometheus.NewCounterVec(opts, labels)
}

// RegisterNewCounterVec registers new Counter Vector.
func (registry *Registry) RegisterNewCounterVec(name string, opts prometheus.CounterOpts, labels []string) (c *prometheus.CounterVec, err error) {
	c = registry.NewCounterVec(opts, labels)
	err = registry.Register(name, c)

	return
}

// RegisterOrGetNewCounterVec registers new Counter Vector.
// If a Counter Vector with the same name is already registered, it returns the Counter Vector instead.
func (registry *Registry) RegisterOrGetNewCounterVec(name string, opts prometheus.CounterOpts, labels []string) (c *prometheus.CounterVec, err error) {
	c = registry.NewCounterVec(opts, labels)
	cReturned, err := registry.RegisterOrGet(name, c)
	if err != nil {
		return
	}

	c, ok := cReturned.(*prometheus.CounterVec)
	if !ok {
		err = ErrInvalidType
		return
	}

	return
}

// NewGauge creates new Gauge.
func (registry *Registry) NewGauge(opts prometheus.GaugeOpts) prometheus.Gauge {
	opts.Namespace = registry.namespace

	return prometheus.NewGauge(opts)
}

// RegisterNewGauge registers new Gauge.
func (registry *Registry) RegisterNewGauge(name string, opts prometheus.GaugeOpts) (c prometheus.Gauge, err error) {
	c = registry.NewGauge(opts)
	err = registry.Register(name, c)

	return
}

// RegisterOrGetNewGauge registers new Gauge.
// If a Gauge with the same name is already registered, it returns the Gauge instead.
func (registry *Registry) RegisterOrGetNewGauge(name string, opts prometheus.GaugeOpts) (c prometheus.Gauge, err error) {
	c = registry.NewGauge(opts)
	cReturned, err := registry.RegisterOrGet(name, c)
	if err != nil {
		return
	}

	c, ok := cReturned.(prometheus.Gauge)
	if !ok {
		err = ErrInvalidType
		return
	}

	return
}

// NewGaugeVec creates new Gauge Vector.
func (registry *Registry) NewGaugeVec(opts prometheus.GaugeOpts, labels []string) *prometheus.GaugeVec {
	opts.Namespace = registry.namespace

	return prometheus.NewGaugeVec(opts, labels)
}

// RegisterNewGaugeVec registers new Gauge Vector.
func (registry *Registry) RegisterNewGaugeVec(name string, opts prometheus.GaugeOpts, labels []string) (c *prometheus.GaugeVec, err error) {
	c = registry.NewGaugeVec(opts, labels)
	err = registry.Register(name, c)

	return
}

// RegisterOrGetNewGaugeVec registers new Gauge Vector.
// If a Gauge Vector with the same name is already registered, it returns the Gauge Vector instead.
func (registry *Registry) RegisterOrGetNewGaugeVec(name string, opts prometheus.GaugeOpts, labels []string) (c *prometheus.GaugeVec, err error) {
	c = registry.NewGaugeVec(opts, labels)
	cReturned, err := registry.RegisterOrGet(name, c)
	if err != nil {
		return
	}

	c, ok := cReturned.(*prometheus.GaugeVec)
	if !ok {
		err = ErrInvalidType
		return
	}

	return
}
