package consul

import (
	"context"
	"errors"
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
	// near is the Consul node instances are sorted by estimated round trip time from.
	near string
	// limit is the maximum number of instances kept per service, 0 means no limit.
	limit int
	mu    sync.RWMutex
	// services maps service names to currently known service instances.
	services map[string][]registry.Instance
}

type options struct {
	near  string
	limit int
}

type Option func(*options) error

// WithNear sorts instances by the estimated round trip time from the given Consul node, nearest first.
// The special node name "_agent" refers to the node of the agent the registry talks to.
func WithNear(node string) Option {
	return func(o *options) error {
		if node == "" {
			return errors.New("near node must not be empty")
		}

		o.near = node

		return nil
	}
}

// WithLimit keeps only the first n instances of every service. It requires WithNear, so that the kept
// instances are the nearest ones.
func WithLimit(n int) Option {
	return func(o *options) error {
		if n <= 0 {
			return fmt.Errorf("limit must be positive, got %d", n)
		}

		o.limit = n

		return nil
	}
}

func NewRegistry(
	address, datacenter string,
	aliases map[string]string,
	refreshPeriod time.Duration,
	opts ...Option,
) (registry.Registry, error) {
	o := &options{}
	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, err
		}
	}

	if o.limit > 0 && o.near == "" {
		return nil, errors.New("limit requires near to be set")
	}

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
		near:          o.near,
		limit:         o.limit,
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
		Near:       cr.near,
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

	if cr.limit > 0 && len(instances) > cr.limit {
		instances = instances[:cr.limit]
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

	sameAddress := func(a, b registry.Instance) bool {
		return a.Address() == b.Address()
	}
	if slices.EqualFunc(oldInstances, newInstances, sameAddress) {
		logger.Infof("no instance changes for (%s)", service)
		return
	}

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

	// The same instances in a different order, e.g. after the round trip time estimates shifted.
	if !changed {
		ch <- registry.RegistryChange{
			Instance: newInstances[0],
			Type:     registry.RCTReordered,
		}
	}

	cr.services[service] = newInstances
	logger.Infof("updated instances from (%s) to (%s) for service (%s)", oldInstances, newInstances, service)
}
