package shard

import (
	"context"
	"errors"
	"sync"

	"github.com/moderntv/hashring"
	"google.golang.org/grpc/balancer"
)

type shardPicker struct {
	sync.RWMutex

	ring       *hashring.Ring
	addr2sc    map[string]balancer.SubConn
	shardKeyFn func(context.Context) string
}

func newShardPicker(ring *hashring.Ring, addr2sc map[string]balancer.SubConn, shardKeyFunc func(context.Context) string) balancer.Picker {
	return &shardPicker{
		ring:       ring,
		addr2sc:    addr2sc,
		shardKeyFn: shardKeyFunc,
	}
}

func (picker *shardPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	shardKey := picker.shardKeyFn(info.Ctx)
	// logger.Infoln("shard balancer picker: picking new conn: ", shardKey)

	addr, err := picker.ring.GetNode(shardKey)
	if err != nil {
		return balancer.PickResult{}, err
	}

	sc, ok := picker.addr2sc[addr]
	if !ok {
		return balancer.PickResult{}, errors.New("dafuq?")
	}
	// logger.Infoln("shard balancer picker: picking new conn: picked addr: ", addr, "for key", shardKey)

	return balancer.PickResult{
		SubConn: sc,
	}, nil
}
