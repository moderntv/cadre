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

var logger = grpclog.Component("[CONSUL REGISTRY]")

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
	var consulService string
	alias, ok := r.aliases[service]
	if ok {
		consulService = alias
		logger.Infof("using alias (%s) to resolve service (%s)", alias, service)
	}

	if !ok {
		consulService = service
		logger.Warningf("no alias defined for service (%s)", service)
	}

	logger.Infof("watching changes for service (%s) every (%s)", service, r.refreshPeriod)
	ticker := time.NewTicker(r.refreshPeriod)
	for {
		r.resolveService(service, consulService, ch)
		logger.Infof("checked changes for service (%s)", service)
		select {
		case <-ticker.C:

		case <-ctx.Done():
			logger.Infof("canceled watch for service (%s)", service)
			return
		}
	}
}

func (r *consulRegistry) resolveService(service, consulService string, ch chan<- registry.RegistryChange) {
	q := &consul.QueryOptions{
		Datacenter: r.datacenter,
	}

	catalog, _, err := r.client.Catalog().Service(consulService, "", q)
	if err != nil {
		logger.Errorf("failed listing consul catalog for service (%s): %v", service, err)
		return
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
		logger.Warningf("could not find any instances for service (%s)", service)
	}

	r.writeChanges(service, instances, ch)
}

func (r *consulRegistry) writeChanges(service string, newInstances []registry.Instance, ch chan<- registry.RegistryChange) {
	r.mu.Lock()
	defer r.mu.Unlock()
	oldInstances := r.services[service]
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

	if changed {
		r.services[service] = newInstances
		logger.Infof("updated registry from %d to %d instances for service (%s)", len(oldInstances), len(newInstances), service)
	}
}
