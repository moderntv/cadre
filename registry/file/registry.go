package file

import (
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/moderntv/cadre/registry"
	"github.com/spf13/viper"
)

type fileRegistry struct {
	v *viper.Viper

	// sLock protects services and instances
	sLock     sync.RWMutex
	services  servicesMap
	instances map[string]map[string]struct{}

	// wLock protects watchers
	wLock    sync.RWMutex
	watchers map[string][]chan registry.RegistryChange
}

type options struct {
	watch bool
}

func newOptions() *options {
	return &options{
		watch: false,
	}
}

type option func(*options) error

// WithWatch will make the registry watch the input file for changes and update accordingly. Default is false.
func WithWatch(watchA ...bool) option {
	watch := true
	if len(watchA) > 0 {
		watch = watchA[0]
	}

	return func(options *options) error {
		options.watch = watch
		return nil
	}
}

func NewRegistry(filePath string, opts ...option) (registry.Registry, error) {
	options := newOptions()
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	v := viper.New()
	r := fileRegistry{
		v:         v,
		services:  servicesMap{},
		instances: map[string]map[string]struct{}{},
		watchers:  map[string][]chan registry.RegistryChange{},
	}

	// r.v.AddConfigPath(".")
	// r.v.AddConfigPath("../config")
	// r.v.SetConfigName("registry")
	// r.v.SetConfigType("yaml")
	r.v.SetConfigFile(filePath)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	if options.watch {
		r.v.WatchConfig()
		r.v.OnConfigChange(func(e fsnotify.Event) {
			log.Println("registry updated", e)
			if err := r.loadInstancesFromViper(); err != nil {
				log.Println("error reloading file registry config:", err)
			}
		})
	}

	return &r, r.loadInstancesFromViper()
}

func (this *fileRegistry) loadInstancesFromViper() error {
	changes := []registry.RegistryChange{}

	rawRegistryData := map[string][]string{}
	if err := this.v.Unmarshal(&rawRegistryData); err != nil {
		return err
	}

	this.sLock.RLock()
	newServices := servicesMap{}
	newInstances := map[string]map[string]struct{}{}
	for service, addrs := range rawRegistryData {
		newServices[service] = []registry.Instance{}
		newInstances[service] = map[string]struct{}{}

		for _, addr := range addrs {
			i := &instance{
				serviceName: service,
				addr:        addr,
			}
			newServices[service] = append(newServices[service], i)
			newInstances[service][addr] = struct{}{}

			// detect newly registered instances
			if _, ok := this.instances[service]; ok {
				if _, ok := this.instances[service][addr]; !ok {
					changes = append(changes, registry.RegistryChange{
						Instance: i,
						Type:     registry.RCTRegistered,
					})
				}
			} else {
				changes = append(changes, registry.RegistryChange{
					Instance: i,
					Type:     registry.RCTRegistered,
				})
			}
		}
	}
	// detect deregistered instances
	for serviceName := range this.instances {
		if _, ok := newInstances[serviceName]; ok {
			// service exists in both new and old instances
			for _, instance := range this.services[serviceName] {
				if _, ok := newInstances[instance.Address()]; !ok {
					// instance doesn't exist in new instances
					changes = append(changes, registry.RegistryChange{
						Instance: instance,
						Type:     registry.RCTDeregistered,
					})
				}
			}
		} else {
			// service doesn't exist anymore => send deregistered change for all instances
			for _, oldInstance := range this.services[serviceName] {
				changes = append(changes, registry.RegistryChange{
					Instance: oldInstance,
					Type:     registry.RCTDeregistered,
				})
			}
		}
	}
	this.sLock.RUnlock()

	this.sLock.Lock()
	this.services = newServices
	this.instances = newInstances
	this.sLock.Unlock()

	this.wLock.RLock()
	for _, change := range changes {
		service := change.Instance.ServiceName()
		serviceWatchers, ok := this.watchers[service]
		if ok {
			for _, watcher := range serviceWatchers {
				watcher <- change
			}
		}
	}
	this.wLock.RUnlock()

	return nil
}

func (this *fileRegistry) Register(serviceInstance registry.Instance) error {
	panic("not implemented")
	// this.sLock.Lock()
	// defer this.sLock.Unlock()
	// return nil
}

func (this *fileRegistry) Deregister(serviceInstance registry.Instance) error {
	panic("not implemented")
	// this.sLock.Lock()
	// defer this.sLock.Unlock()
	// return nil
}

func (this *fileRegistry) Instances(service string) []registry.Instance {
	this.sLock.RLock()
	defer this.sLock.RUnlock()

	is, ok := this.services[service]
	if !ok {
		return []registry.Instance{}
	}
	return is
}

func (this *fileRegistry) Watch(service string) (<-chan registry.RegistryChange, func()) {
	this.wLock.Lock()
	defer this.wLock.Unlock()

	c := make(chan registry.RegistryChange)
	if _, ok := this.watchers[service]; !ok {
		this.watchers[service] = []chan registry.RegistryChange{}
	}
	p := len(this.watchers[service])
	this.watchers[service] = append(this.watchers[service], c)
	f := func() {
		this.wLock.Lock()
		defer this.wLock.Unlock()

		this.watchers[service] = append(this.watchers[service][:p], this.watchers[service][p+1:]...)

		close(c)
	}

	return c, f
}
