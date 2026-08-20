// Package file implements a [github.com/moderntv/cadre/registry.Registry] backed by a YAML file mapping
// service names to instance addresses.
//
//	---
//	aggregator:
//	  - aggregator1.moderntv.eu
//	  - aggregator2.moderntv.eu
//
//	ingest:
//	  - ingest.moderntv.eu
//
// The file is read once at construction. Pass [WithWatch] to also watch it and reload the instances
// whenever it changes, publishing the difference to everyone watching a service:
//
//	r, err := file.NewRegistry("./registry.yaml", file.WithWatch())
//
// The file is a read-only view of the topology, so Register and Deregister are not implemented and panic if
// they are called. Use this backend to point a service at its dependencies without running a discovery
// service.
package file
