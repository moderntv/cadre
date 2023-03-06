package shard

import (
	"fmt"

	"github.com/cespare/xxhash/v2"
	"github.com/moderntv/hashring"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

type pickerBuilder struct {
	options builderOptions
}

func newPickerBuilder(options builderOptions) base.PickerBuilder {
	return &pickerBuilder{
		options: options,
	}
}

func (pb *pickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if logger.V(2) {
		logger.Infof("shard balancer: building new picker: %v", info)
	}

	if len(info.ReadySCs) <= 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	ring, err := hashring.New(hashring.WithHashFunc(xxhash.New()))
	if err != nil {
		return base.NewErrPicker(fmt.Errorf("cannot create new hashring: %w", err))
	}

	addr2sc := map[string]balancer.SubConn{}
	for sc, sci := range info.ReadySCs {
		addr2sc[sci.Address.Addr] = sc

		err := ring.AddNode(sci.Address.Addr)
		if err != nil {
			err = fmt.Errorf("cannot add node to ring: %w", err)
			return base.NewErrPicker(err)
		}
	}

	return newShardPicker(ring, addr2sc, pb.options.shardKeyFunc)
}
