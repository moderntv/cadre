// Package registry abstracts service discovery and plugs it into gRPC as a resolver.
//
// A [Registry] maps a service name to the addresses of its currently known instances, and lets callers
// watch that mapping for changes. The concrete backends live in the sub-packages: static for a fixed map,
// file for a YAML file that can be reloaded on change, and consul for the Consul catalog.
//
// # gRPC resolver
//
// [NewResolverBuilder] turns any Registry into a gRPC resolver registered under the [Scheme] scheme, so
// client connections follow the registry as instances come and go:
//
//	r, err := file.NewRegistry("./registry.yaml", file.WithWatch())
//	if err != nil {
//		return err
//	}
//
//	resolver.Register(registry.NewResolverBuilder(r))
//
//	cc, err := grpc.NewClient("registry:///aggregator", // registry://<authority>/<service>
//		grpc.WithTransportCredentials(insecure.NewCredentials()))
//
// The service name is taken from the target's endpoint, so the authority is left empty in practice.
//
// # Watching
//
// [Registry.Watch] returns a channel of [RegistryChange] values, each reporting an [Instance] that was
// registered or deregistered, together with a function that stops the watch and closes the channel.
//
// Not every backend supports the whole interface: file and consul are read-only views of an external source
// of truth, so their Register and Deregister methods are either no-ops or panic. Check the backend's own
// documentation before calling them.
package registry
