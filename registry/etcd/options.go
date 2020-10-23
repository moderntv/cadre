package etcd

import (
	"time"
)

type etcdOptions struct {
	ttl time.Duration
}

func defaultOptions() *etcdOptions {
	return &etcdOptions{
		ttl: 30 * time.Second,
	}
}

type Option func(*etcdOptions) error

func WithTTL(ttl time.Duration) Option {
	return func(options *etcdOptions) error {
		options.ttl = ttl

		return nil
	}
}
