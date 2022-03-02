package shard

import (
	"github.com/serialx/hashring"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

type pickerBuilder struct {
	ring      *hashring.HashRing
	options   builderOptions
	lastConns map[string]bool
}

func newPickerBuilder(options builderOptions) (base.PickerBuilder, error) {
	ring := hashring.New([]string{})

	return &pickerBuilder{
		ring:      ring,
		options:   options,
		lastConns: map[string]bool{},
	}, nil
}

func (pb *pickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	// grpclog.Infoln("shard balancer: building new picker: ", info)
	if len(info.ReadySCs) <= 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	newLastConns := map[string]bool{}
	addr2sc := map[string]balancer.SubConn{}
	for sc, sci := range info.ReadySCs {
		newLastConns[sci.Address.Addr] = true
		addr2sc[sci.Address.Addr] = sc
		if _, ok := pb.lastConns[sci.Address.Addr]; !ok {
			pb.ring.AddNode(sci.Address.Addr)
		}
	}
	for addr := range pb.lastConns {
		if _, ok := newLastConns[addr]; !ok {
			pb.ring.RemoveNode(addr)
		}

	}
	pb.lastConns = newLastConns

	return newShardPicker(pb.ring, addr2sc, pb.options.shardKeyFunc)
}
