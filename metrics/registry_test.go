package metrics

import (
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	testNameSimple    = "simple"
	testCounterMetric = "simple_counter"
)

// DISABLED: deepequal cannot compare pointers
// func TestNewRegistry(t *testing.T) {
//     simplePrometheusRegistry := prometheus.NewRegistry()
//     goCollector := prometheus.NewGoCollector()
//     processCollector := prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{})
//     simplePrometheusRegistry.MustRegister()
//     simpleRegistry := &Registry{
//         prometheusRegistry: simplePrometheusRegistry,
//         metrics: map[string]prometheus.Collector{
//             "go":      goCollector,
//             "process": processCollector,
//         },
//     }
//
//     tests := []struct {
//         name         string
//         wantRegistry *Registry
//         wantErr      bool
//     }{
//         {
//             name:         "simple",
//             wantRegistry: simpleRegistry,
//             wantErr:      false,
//         },
//     }
//     for _, tt := range tests {
//         t.Run(tt.name, func(t *testing.T) {
//             gotRegistry, err := NewRegistry()
//             if (err != nil) != tt.wantErr {
//                 t.Errorf("NewRegistry() error = %v, wantErr %v", err, tt.wantErr)
//                 return
//             }
//             if !reflect.DeepEqual(gotRegistry, tt.wantRegistry) {
//                 t.Errorf("NewRegistry() = %v, want %v", gotRegistry, tt.wantRegistry)
//             }
//         })
//     }
// }

func TestRegistry_Register(t *testing.T) {
	type fields struct {
		prometheusRegistry *prometheus.Registry
		metrics            map[string]prometheus.Collector
	}

	type args struct {
		name string
		c    prometheus.Collector
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "empty name",
			fields: fields{
				prometheusRegistry: prometheus.NewRegistry(),
				metrics:            make(map[string]prometheus.Collector),
			},
			args: args{
				"",
				prometheus.NewCounter(prometheus.CounterOpts{}),
			},
			wantErr: true,
		},
		{
			name: "nil metric",
			fields: fields{
				prometheusRegistry: prometheus.NewRegistry(),
				metrics:            make(map[string]prometheus.Collector),
			},
			args: args{
				"nil_metric",
				nil,
			},
			wantErr: true,
		},
		{
			name: testNameSimple,
			fields: fields{
				prometheusRegistry: prometheus.NewRegistry(),
				metrics:            make(map[string]prometheus.Collector),
			},
			args: args{
				testCounterMetric,
				prometheus.NewCounter(prometheus.CounterOpts{Name: testCounterMetric}),
			},
			wantErr: false,
		},
		{
			name: "re-register",
			fields: fields{
				prometheusRegistry: prometheus.NewRegistry(),
				metrics: map[string]prometheus.Collector{
					testCounterMetric: prometheus.NewCounter(prometheus.CounterOpts{}),
				},
			},
			args: args{
				testCounterMetric,
				prometheus.NewCounter(prometheus.CounterOpts{}),
			},
			wantErr: true,
		},
		{
			name: "invalid collector",
			fields: fields{
				prometheusRegistry: prometheus.NewRegistry(),
				metrics:            map[string]prometheus.Collector{},
			},
			args: args{
				testCounterMetric,
				prometheus.NewCounter(prometheus.CounterOpts{}),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &Registry{
				prometheusRegistry: tt.fields.prometheusRegistry,
				metrics:            tt.fields.metrics,
			}

			err := registry.Register(tt.args.name, tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("Registry.Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistry_Unregister(t *testing.T) {
	type fields struct {
		prometheusRegistry *prometheus.Registry
		metrics            map[string]prometheus.Collector
	}

	type args struct {
		name string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: testNameSimple,
			fields: fields{
				prometheusRegistry: prometheus.NewRegistry(),
				metrics: map[string]prometheus.Collector{
					testCounterMetric: prometheus.NewCounter(prometheus.CounterOpts{Name: testCounterMetric}),
				},
			},
			args: args{
				testCounterMetric,
			},
			wantErr: false,
		},
		{
			name: "nonexistent",
			fields: fields{
				prometheusRegistry: prometheus.NewRegistry(),
				metrics: map[string]prometheus.Collector{
					testCounterMetric: prometheus.NewCounter(prometheus.CounterOpts{Name: testCounterMetric}),
				},
			},
			args: args{
				"wat?",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &Registry{
				prometheusRegistry: tt.fields.prometheusRegistry,
				metrics:            tt.fields.metrics,
			}

			err := registry.Unregister(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("Registry.Unregister() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistry_Get(t *testing.T) {
	collectors := map[string]prometheus.Collector{
		testCounterMetric: prometheus.NewCounter(prometheus.CounterOpts{Name: testCounterMetric}),
	}

	type fields struct {
		prometheusRegistry *prometheus.Registry
		metrics            map[string]prometheus.Collector
	}

	type args struct {
		name string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantC   prometheus.Collector
		wantErr bool
	}{
		{
			name: testNameSimple,
			fields: fields{
				prometheusRegistry: prometheus.NewRegistry(),
				metrics: map[string]prometheus.Collector{
					testCounterMetric: collectors[testCounterMetric],
				},
			},
			args: args{
				testCounterMetric,
			},
			wantC:   collectors[testCounterMetric],
			wantErr: false,
		},
		{
			name: "nonexistent",
			fields: fields{
				prometheusRegistry: prometheus.NewRegistry(),
				metrics: map[string]prometheus.Collector{
					testCounterMetric: prometheus.NewCounter(prometheus.CounterOpts{Name: testCounterMetric}),
				},
			},
			args: args{
				"wat?",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &Registry{
				prometheusRegistry: tt.fields.prometheusRegistry,
				metrics:            tt.fields.metrics,
			}

			gotC, err := registry.Get(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("Registry.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(gotC, tt.wantC) {
				t.Errorf("Registry.Get() = %+v, want %+v", gotC, tt.wantC)
			}
		})
	}
}
