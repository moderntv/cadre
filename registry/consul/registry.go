package consul

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	consul "github.com/hashicorp/consul/api"
	"github.com/moderntv/cadre/registry"
	"google.golang.org/grpc/grpclog"
)

var logger = grpclog.Component("consul_registry")

var _ registry.Registry = &consulRegistry{}

type consulRegistry struct {
	client        *consul.Client
	datacenter    string
	refreshPeriod time.Duration
	aliases       map[string]string
	mu            sync.RWMutex
	services      map[string][]registry.Instance
}

func NewRegistry(address, datacenter string, aliases map[string]string, refreshPeriod time.Duration) (registry.Registry, error) {
	config := consul.DefaultConfig()
	config.Address = address
	c, err := consul.NewClient(config)
	if err != nil {
		return nil, err
	}

	r := &consulRegistry{
		client:        c,
		datacenter:    datacenter,
		refreshPeriod: refreshPeriod,
		aliases:       aliases,
		services:      make(map[string][]registry.Instance),
	}
	return r, nil
}

func (r *consulRegistry) Register(serviceInstance registry.Instance) error {
	return nil
}

func (r *consulRegistry) Deregister(serviceInstance registry.Instance) error {
	return nil
}

func (r *consulRegistry) Instances(service string) []registry.Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	is, ok := r.services[service]
	if !ok {
		return []registry.Instance{}
	}

	return is
}

func (r *consulRegistry) Watch(service string) (<-chan registry.RegistryChange, func()) {
	ch := make(chan registry.RegistryChange)
	ctx, cancel := context.WithCancel(context.Background())
	go r.watch(ctx, service, ch)
	closeFn := func() {
		close(ch)
		cancel()
	}
	return ch, closeFn
}

func (r *consulRegistry) watch(ctx context.Context, service string, ch chan<- registry.RegistryChange) {
	ticker := time.NewTicker(r.refreshPeriod)
	q := &consul.QueryOptions{
		Datacenter: r.datacenter,
	}
	for {
		name, ok := r.aliases[service]
		if !ok {
			name = service
			logger.Warningf("[CONSUL REGISTRY] no alias defined for service (%s)", service)
		}

		catalog, _, err := r.client.Catalog().Service(name, "", q)
		if err != nil {
			logger.Errorf("[CONSUL REGISTRY] failed listing consul catalog for service (%s): %v", name, err)
			continue
		}

		instances := make([]registry.Instance, 0, len(catalog))
		for _, s := range catalog {
			i := instance{
				serviceName: service,
				addr:        fmt.Sprintf("%s:%d", s.Node, s.ServicePort),
			}
			instances = append(instances, i)
		}

		if len(instances) == 0 {
			logger.Warningf("[CONSUL REGISTRY] could not find any instances for service (%s)", service)
		}

		changed := r.writeChanges(r.services[service], instances, ch)
		if changed {
			r.mu.Lock()
			r.services[service] = instances
			r.mu.Unlock()
			logger.Infof("[CONSUL REGISTRY] updated registry to %d instances for service (%s)", len(instances), service)
		}

		select {
		case <-ticker.C:

		case <-ctx.Done():
			return
		}
	}
}

func (r *consulRegistry) writeChanges(oldInstances, newInstances []registry.Instance, ch chan<- registry.RegistryChange) bool {
	changed := false
	for _, oldInstance := range oldInstances {
		containsFunc := func(i registry.Instance) bool {
			return i.Address() == oldInstance.Address()
		}
		if !slices.ContainsFunc(newInstances, containsFunc) {
			changed = true
			ch <- registry.RegistryChange{
				Instance: oldInstance,
				Type:     registry.RCTDeregistered,
			}
		}
	}

	for _, newInstance := range newInstances {
		containsFunc := func(i registry.Instance) bool {
			return i.Address() == newInstance.Address()
		}
		if !slices.ContainsFunc(oldInstances, containsFunc) {
			changed = true
			ch <- registry.RegistryChange{
				Instance: newInstance,
				Type:     registry.RCTRegistered,
			}
		}
	}

	return changed
}
