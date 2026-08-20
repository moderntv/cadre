// Package shard provides a consistent-hashing gRPC load balancer.
//
// Instead of spreading calls over the available instances, the balancer routes every call to the instance
// that owns the call's shard key, so requests concerning the same entity consistently reach the same
// backend. It is built on [github.com/moderntv/hashring] with an xxhash hash function, which keeps the
// reshuffling caused by an instance appearing or disappearing to a minimum.
//
// Importing the package registers the balancer under the name [Name], so it can be selected through a
// service config:
//
//	cc, err := grpc.NewClient("registry:///aggregator",
//		grpc.WithTransportCredentials(insecure.NewCredentials()),
//		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"shard": {}}]}`),
//	)
//
// # Shard keys
//
// The key is read from the call context under [DefaultShardKeyName]:
//
//	ctx = context.WithValue(ctx, shard.DefaultShardKeyName, channelID)
//	res, err := client.GetChannel(ctx, req)
//
// A call whose context carries no key falls back to the literal string "NOT_FOUND" and therefore always
// lands on the same instance, so make sure the key is set on every call.
//
// [WithShardKeyFunc] exists for reading the key from somewhere else, such as gRPC metadata, but neither
// [NewBuilder] nor [NewNamedBuilder] currently accepts options, so the context lookup above is the only
// reachable behaviour. Use [NewNamedBuilder] to register the same balancer under an additional name.
package shard
