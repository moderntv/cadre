package etcd

import (
	"time"

	"go.etcd.io/etcd/clientv3"

	"github.com/moderntv/cadre/registry"
)

type etcdRegistry struct {
	client   *clientv3.Client
	kv       *clientv3.KV
	watchers map[string]chan registry.RegistryChange
}

func NewRegistry(cfg clientv3.Config, opts ...Option) (*etcdRegistry, error) {
	options := defaultOptions()
	for _, option := range opts {
		if err := option(options); err != nil {
			return nil, err
		}
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	kv := clientv3.NewKV(client)

	r := etcdRegistry{
		client: client,
		kv:     &kv,
	}

	return &r, nil
}

func (this *etcdRegistry) Register(serviceInstance registry.Instance) error {
	return nil
}

func (this *etcdRegistry) Deregister(serviceInstance registry.Instance) error {
	return nil
}

func (this *etcdRegistry) Instances(service registry.Service) []registry.Instance {
	return []registry.Instance{}
}

func (this *etcdRegistry) Watch(service string) (<-chan registry.RegistryChange, func()) {
	c := make(chan registry.RegistryChange)
	f := func() {
		close(c)
	}

	return c, f
}
