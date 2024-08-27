package registry

import (
	"context"
	"time"

	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
)

const Scheme = "registry"

var logger = grpclog.Component("registry_resolver")

type resolverBuilder struct {
	registry Registry
}

func NewResolverBuilder(registry Registry) resolver.Builder {
	return &resolverBuilder{
		registry: registry,
	}
}

func (rb *resolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := newResolver(target, rb.registry, cc)
	go r.start()

	return r, nil
}

func (rb *resolverBuilder) Scheme() string {
	return Scheme
}

// registryResolver is a Resolver(https://godoc.org/google.golang.org/grpc/resolver#Resolver).
type registryResolver struct {
	ctx       context.Context
	ctxCancel func()

	// service  resolver.Target
	service  Service
	cc       resolver.ClientConn
	registry Registry
}

func newResolver(target resolver.Target, registry Registry, cc resolver.ClientConn) (res *registryResolver) {
	res = &registryResolver{
		service:  &service{name: target.Endpoint()},
		registry: registry,
		cc:       cc,
	}

	res.ctx, res.ctxCancel = context.WithCancel(context.Background())

	return
}

func (rr *registryResolver) start() {
	c, stop := rr.registry.Watch(rr.service.Name())
	rr.updateAddressesFromRegistry()
	for {
		select {
		case <-c:
			// TODO: implement some update instead of replacing the whole array?
			if logger.V(3) {
				logger.Infoln("got services update from registry")
			}

			rr.updateAddressesFromRegistry()
		case <-rr.ctx.Done():
			stop()
			return
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (rr *registryResolver) updateAddressesFromRegistry() {
	is := rr.registry.Instances(rr.service.Name())
	addrs := []resolver.Address{}
	for _, i := range is {
		addrs = append(addrs, resolver.Address{Addr: i.Address()})
	}

	if logger.V(2) {
		logger.Infof("setting new service (`%v`) addresses from registry: `%v` from raw instances `%v`\n", rr.service.Name(), is, addrs)
	}
	err := rr.cc.UpdateState(resolver.State{
		Addresses: addrs,
	})
	if err != nil {
		logger.Errorf("service (`%v`) connection update failed from registry: %v", rr.service.Name(), err)
		return
	}
}

func (rr *registryResolver) ResolveNow(o resolver.ResolveNowOptions) {
	rr.updateAddressesFromRegistry()
}

func (rr *registryResolver) Close() {
	rr.ctxCancel()
}
