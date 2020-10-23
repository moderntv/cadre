package shard

import (
	"context"
	"fmt"

	"github.com/moderntv/hashring"
	"github.com/cespare/xxhash/v2"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/grpclog"
)

func NewBuilder(opts ...builderOption) (balancer.Builder, error) {
	options := defaultBuilderOptions()
	pickerBuilder, err := newPickerBuilder(options)
	if err != nil {
		return nil, fmt.Errorf("cannot create picker builder: %v", err)
	}

	return base.NewBalancerBuilder(Name, pickerBuilder, base.Config{HealthCheck: true}), nil
}

type builderOption func(*builderOptions) error
type builderOptions struct {
	shardKeyFunc func(context.Context) string
}

func defaultBuilderOptions() builderOptions {
	return builderOptions{
		shardKeyFunc: func(ctx context.Context) string {
			key, ok := ctx.Value(DefaultShardKeyName).(string)
			if !ok {
				return "NOT_FOUND"
			}
			return key
		},
	}
}

type pickerBuilder struct {
	options   builderOptions
	ring      *hashring.Ring
	lastConns map[string]bool
}

func newPickerBuilder(options builderOptions) (base.PickerBuilder, error) {
	ring, err := hashring.New(hashring.WithHashFunc(xxhash.New()))
	if err != nil {
		return nil, fmt.Errorf("cannot create ring for picker builder: %v", err)
	}

	return &pickerBuilder{
		ring:      ring,
		options:   options,
		lastConns: map[string]bool{},
	}, nil
}

func (this *pickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	grpclog.Infoln("shard balancer: building new picker: ", info)
	if len(info.ReadySCs) <= 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	newLastConns := map[string]bool{}
	addr2sc := map[string]balancer.SubConn{}
	for sc, sci := range info.ReadySCs {
		newLastConns[sci.Address.Addr] = true
		addr2sc[sci.Address.Addr] = sc
		if _, ok := this.lastConns[sci.Address.Addr]; !ok {
			this.ring.AddNode(sci.Address.Addr)
		}
	}
	for addr, _ := range this.lastConns {
		if _, ok := newLastConns[addr]; !ok {
			this.ring.DeleteNode(addr)
		}

	}
	this.lastConns = newLastConns

	return newShardPicker(this.ring, addr2sc, this.options.shardKeyFunc)
}

