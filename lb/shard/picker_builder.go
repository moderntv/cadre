package shard

import (
	"fmt"
	"log"

	"github.com/cespare/xxhash/v2"
	"github.com/moderntv/hashring"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
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
	// grpclog.Infoln("shard balancer: building new picker: ", info)
	log.Printf("shard balancer: building new picker: %+v", info)
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
			pb.ring.DeleteNode(addr)
		}

	}
	pb.lastConns = newLastConns

	return newShardPicker(pb.ring, addr2sc, pb.options.shardKeyFunc)
}
