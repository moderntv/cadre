// Package http provides the gin-based HTTP server used by Cadre together with a declarative way of
// describing its routes.
//
// [HttpServer] is a thin wrapper around a gin engine. It is normally created for you by the Cadre builder
// rather than directly - the builder attaches the metrics, logging and recovery middleware and registers the
// routing groups. Every server answers unmatched requests with a JSON 404 carrying the error type
// "NO_ROUTE". Importing this package puts gin into release mode.
//
// # Routing groups
//
// [RoutingGroup] describes a route tree instead of a sequence of registration calls, which lets the builder
// merge the routes of two servers that share a listening address:
//
//	http.RoutingGroup{
//		Base: "/api",
//		Groups: []http.RoutingGroup{
//			{
//				Base:       "/v1",
//				Middleware: []gin.HandlerFunc{requireAPIKey},
//				Routes: map[string]map[string][]gin.HandlerFunc{
//					"/orders":     {"GET": {listOrders}, "POST": {createOrder}},
//					"/orders/:id": {"GET": {getOrder}},
//				},
//			},
//		},
//	}
//
// Routes are keyed by path and then by method. Middleware attached to a group guards that group and its
// sub-groups only. Groups nest arbitrarily deep and are merged recursively, so two groups sharing a Base
// are combined rather than colliding.
//
// A [StaticRoute] serves files from a directory or from an fs.FS - set exactly one of Root and FS.
//
// Note that this package shadows the standard library's net/http. Alias the standard one, for example as
// stdhttp, in files that need both.
package http
