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
	client *consul.Client
	// datacenter represents Consul datacenter.
	datacenter string
	// refreshPeriod determines period for Consul catalog API call.
	refreshPeriod time.Duration
	// aliases is used for mapping gRPC service names to Consul service names.
	aliases map[string]string
	mu      sync.RWMutex
	// services maps service names to currently known service instances.
	services map[string][]registry.Instance
}

func NewRegistry(
	address, datacenter string,
	aliases map[string]string,
	refreshPeriod time.Duration,
) (registry.Registry, error) {
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

func (cr *consulRegistry) Register(serviceInstance registry.Instance) error {
	return nil
}

func (cr *consulRegistry) Deregister(serviceInstance registry.Instance) error {
	return nil
}

func (cr *consulRegistry) Instances(service string) []registry.Instance {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	is, ok := cr.services[service]
	if !ok {
		return []registry.Instance{}
	}

	return is
}

func (cr *consulRegistry) Watch(service string) (<-chan registry.RegistryChange, func()) {
	// FIX: The change propagation (i.e. the channel communication) has to be somehow reworked.
	// I currently "resolved" the issue by using buffered channel.
	// Please note that if any service has the "delta" of instances (number of changes processed by `writeChanges()`
	// method) greater than the channel buffer size it will deadlock. This is caused by the resolver which calls
	// the `Instances()` method for each change which tries to read-lock the registry mutex.
	// This is problematic in conjunction with the locking in the `writeChanges()` method which writes to this channel
	// under write lock.
	changesCh := make(chan registry.RegistryChange, 1024)
	initializedCh := make(chan bool)
	ctx, cancel := context.WithCancel(context.Background())
	go cr.watch(ctx, service, changesCh, initializedCh)
	closeFn := func() {
		close(changesCh)
		cancel()
	}
	<-initializedCh
	return changesCh, closeFn
}

func (cr *consulRegistry) watch(
	ctx context.Context,
	service string,
	changesCh chan<- registry.RegistryChange,
	initializedCh chan<- bool,
) {
	var consulService string
	alias, ok := cr.aliases[service]
	if ok {
		consulService = alias
		logger.Infof("using alias (%s) to resolve service (%s)", alias, service)
	} else {
		consulService = service
		logger.Warningf("no alias defined for service (%s)", service)
	}

	cr.resolveService(service, consulService, changesCh)
	logger.Infof("initialized registry for service (%s)", service)
	initializedCh <- true

	logger.Infof("watching changes for service (%s) every (%s)", service, cr.refreshPeriod)
	ticker := time.NewTicker(cr.refreshPeriod)
	for {
		select {
		case <-ticker.C:
			cr.resolveService(service, consulService, changesCh)

		case <-ctx.Done():
			logger.Infof("canceled watch for service (%s)", service)
			return
		}
	}
}

func (cr *consulRegistry) resolveService(service, consulService string, ch chan<- registry.RegistryChange) {
	q := &consul.QueryOptions{
		Datacenter: cr.datacenter,
	}

	entries, _, err := cr.client.Health().Service(consulService, "", true, q)
	if err != nil {
		logger.Errorf("failed listing consul health catalog for service (%s): %v", service, err)
		return
	}

	instances := make([]registry.Instance, 0, len(entries))
	for _, entry := range entries {
		i := instance{
			serviceName: service,
			addr:        fmt.Sprintf("%s:%d", entry.Node.Node, entry.Service.Port),
		}
		instances = append(instances, i)
	}

	if len(instances) == 0 {
		logger.Warningf("could not find any instances for service (%s)", service)
	}

	cr.writeChanges(service, instances, ch)
}

func (cr *consulRegistry) writeChanges(
	service string,
	newInstances []registry.Instance,
	ch chan<- registry.RegistryChange,
) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	oldInstances := cr.services[service]
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
		cr.services[service] = newInstances
		logger.Infof("updated instances from (%s) to (%s) for service (%s)", oldInstances, newInstances, service)
	} else {
		logger.Infof("no instance changes for (%s)", service)
	}
}
