// Package static implements a [github.com/moderntv/cadre/registry.Registry] backed by a fixed map of
// service names to addresses.
//
//	r, err := static.NewRegistry(map[string][]string{
//		"aggregator": {"aggregator1.moderntv.eu:9000", "aggregator2.moderntv.eu:9000"},
//		"ingest":     {"ingest.moderntv.eu:9000"},
//	})
//
// The set of instances is fixed at construction time. Register and Deregister are accepted but do nothing,
// and the channel returned by Watch never produces a change - it exists only to satisfy the interface.
//
// This is the backend to reach for in tests and in deployments where the topology is known up front and
// comes from configuration.
package static
