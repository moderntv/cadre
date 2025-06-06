package static

import (
	"github.com/moderntv/cadre/registry"
)

type staticRegistry map[string][]registry.Instance

func NewRegistry(cfg map[string][]string) (registry.Registry, error) {
	r := staticRegistry{}

	for service, instances := range cfg {
		r[service] = []registry.Instance{}
		for _, addr := range instances {
			r[service] = append(r[service], &instance{
				serviceName: service,
				addr:        addr,
			})
		}
	}

	return &r, nil
}

func (sr *staticRegistry) Register(serviceInstance registry.Instance) error {
	return nil
}

func (sr *staticRegistry) Deregister(serviceInstance registry.Instance) error {
	return nil
}

func (sr *staticRegistry) Instances(service string) []registry.Instance {
	if sr == nil {
		return []registry.Instance{}
	}

	is, ok := (*sr)[service]
	if !ok {
		return []registry.Instance{}
	}
	return is
}

func (sr *staticRegistry) Watch(service string) (<-chan registry.RegistryChange, func()) {
	c := make(chan registry.RegistryChange)
	f := func() {
		close(c)
	}

	return c, f
}
