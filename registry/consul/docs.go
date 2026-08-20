// Package consul implements a [github.com/moderntv/cadre/registry.Registry] on top of the Consul catalog.
//
//	r, err := consul.NewRegistry(
//		"consul.moderntv.eu:8500",
//		"dc1",
//		map[string]string{"example.GreeterService": "greeter"}, // gRPC service name -> Consul service name
//		10*time.Second,
//	)
//
// Polling is driven by Watch: calling it resolves the service once, then re-resolves it every refreshPeriod
// and reports the difference as registrations and deregistrations. Only passing instances are returned, and
// an instance address is composed from the Consul node name and the service port. Because the instance
// cache is filled by the watch, Instances reports nothing for a service nobody is watching yet.
//
// The aliases map translates gRPC service names into the names used in Consul, for the common case where
// the two differ. A service with no alias is looked up under its own name.
//
// Registration is expected to be handled outside the application - by the Consul agent or the scheduler -
// so this backend is a read-only view: Register and Deregister are accepted but do nothing.
package consul
