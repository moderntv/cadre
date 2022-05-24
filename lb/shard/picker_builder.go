package shard

import (
	"fmt"

	"github.com/cespare/xxhash/v2"
	"github.com/moderntv/hashring"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/grpclog"
)

type pickerBuilder struct {
	ring      *hashring.Ring
	options   builderOptions
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

func (pb *pickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	grpclog.Infoln("shard balancer: building new picker: ", info)
	if len(info.ReadySCs) <= 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	newLastConns := map[string]bool{}
	addr2sc := map[string]balancer.SubConn{}
	for sc, sci := range info.ReadySCs {
		newLastConns[sci.Address.Addr] = true
		addr2sc[sci.Address.Addr] = sc
		if _, ok := pb.lastConns[sci.Address.Addr]; !ok {
			err := pb.ring.AddNode(sci.Address.Addr)
			if err != nil {
				err = fmt.Errorf("cannot add node to ring: %w", err)
				return base.NewErrPicker(err)
			}
		}
	}
	for addr := range pb.lastConns {
		if _, ok := newLastConns[addr]; !ok {
			err := pb.ring.DeleteNode(addr)
			if err != nil {
				err = fmt.Errorf("cannot remove node from ring: %w", err)
				return base.NewErrPicker(err)
			}
		}

	}
	pb.lastConns = newLastConns

	return newShardPicker(pb.ring, addr2sc, pb.options.shardKeyFunc)
}
