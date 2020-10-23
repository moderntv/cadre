package registry

import (
	"time"

	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
)

const Scheme = "registry"

type resolverBuilder struct {
	registry Registry
}

func NewResolverBuilder(registry Registry) resolver.Builder {
	return &resolverBuilder{
		registry: registry,
	}
}

func (this *resolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := newResolver(target, this.registry, cc)
	go r.start()

	return r, nil
}

func (this *resolverBuilder) Scheme() string {
	return "registry"
}

// registryResolver is a Resolver(https://godoc.org/google.golang.org/grpc/resolver#Resolver).
type registryResolver struct {
	// service  resolver.Target
	service  Service
	cc       resolver.ClientConn
	registry Registry
	quit     chan bool
}

func newResolver(target resolver.Target, registry Registry, cc resolver.ClientConn) *registryResolver {
	service := &service{name: target.Endpoint}
	return &registryResolver{
		service:  service,
		registry: registry,
		cc:       cc,
	}
}

func (this *registryResolver) start() {
	this.updateAddressesFromRegistry()

	c, stop := this.registry.Watch(this.service.Name())
	for {
		select {
		case <-c:
			// TODO: implement some update instead of replacing the whole array?
			grpclog.Infoln("[RESOLVER] got services update from registry")
			this.updateAddressesFromRegistry()
		case <-this.quit:
			stop()
			return
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (this *registryResolver) updateAddressesFromRegistry() {
	is := this.registry.Instances(this.service.Name())
	addrs := []resolver.Address{}
	for _, i := range is {
		addrs = append(addrs, resolver.Address{Addr: i.Address()})
	}
	grpclog.Infof("[RESOLVER] setting new service (`%v`) addresses from registry: `%v` from raw instances `%v`\n", this.service.Name(), is, addrs)
	this.cc.UpdateState(resolver.State{
		Addresses: addrs,
	})
}

func (this *registryResolver) ResolveNow(o resolver.ResolveNowOptions) {
	this.updateAddressesFromRegistry()
}

func (this *registryResolver) Close() {
	this.quit <- true
}
