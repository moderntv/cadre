package cadre

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/moderntv/cadre/http"
	"github.com/moderntv/cadre/http/middleware"
	"github.com/moderntv/cadre/metrics"
)

// HTTP Options
type httpOptions struct {
	serverName       string
	services         []string
	listeningAddress string

	enableLoggingMiddleware bool
	enableMetricsMiddleware bool

	globalMiddleware   []gin.HandlerFunc
	metricsAggregation bool
	routingGroups      map[string]http.RoutingGroup
}

func (h *httpOptions) ensure() (err error) {
	if h.listeningAddress == "" {
		return fmt.Errorf("no listening address for http server `%s`", h.serverName)
	}

	return
}

func (h *httpOptions) merge(other *httpOptions) (hh *httpOptions, err error) {
	log.Printf("merging %s into %s", other.serverName, h.serverName)

	hh = &httpOptions{
		serverName:       h.serverName,
		services:         append(h.services, other.services...),
		listeningAddress: h.listeningAddress,

		enableLoggingMiddleware: h.enableLoggingMiddleware,
		enableMetricsMiddleware: h.enableMetricsMiddleware,

		globalMiddleware:   append(h.globalMiddleware, other.globalMiddleware...),
		metricsAggregation: h.metricsAggregation,
		routingGroups:      h.routingGroups,
	}

	for _, othersRoutingGroup := range other.routingGroups {
		err = WithRoutingGroup(othersRoutingGroup)(hh)
		if err != nil {
			return
		}
	}

	return
}

func (h *httpOptions) build(cadreContext context.Context, logger zerolog.Logger, metricsRegistry *metrics.Registry) (httpServer *http.HttpServer, err error) {
	serverMiddlewares := []gin.HandlerFunc{}
	{
		if h.enableMetricsMiddleware {
			var metricsMiddleware gin.HandlerFunc
			metricsMiddleware, err = middleware.NewMetrics(metricsRegistry, h.serverName, h.metricsAggregation)
			if err != nil {
				return
			}

			serverMiddlewares = append(serverMiddlewares, metricsMiddleware)
		}

		if h.enableLoggingMiddleware {
			serverMiddlewares = append(serverMiddlewares, middleware.NewLogger(logger))
		}

		serverMiddlewares = append(serverMiddlewares, gin.Recovery())
		serverMiddlewares = append(serverMiddlewares, h.globalMiddleware...)
	}

	httpServer, err = http.NewHttpServer(cadreContext, h.serverName, h.listeningAddress, logger, serverMiddlewares...)
	if err != nil {
		return
	}

	for _, group := range h.routingGroups {
		err = httpServer.RegisterRouteGroup(group)
		if err != nil {
			return
		}
	}

	return
}

func defaultHTTPOptions() *httpOptions {
	return &httpOptions{
		services:                []string{},
		enableLoggingMiddleware: true,
		enableMetricsMiddleware: true,

		globalMiddleware: []gin.HandlerFunc{},
		routingGroups:    map[string]http.RoutingGroup{},
	}
}

type HTTPOption func(*httpOptions) error

// WithHTTP enables HTTP server
func WithHTTP(serverName string, myHTTPOptions ...HTTPOption) Option {
	return func(options *Builder) error {
		if options.httpOptions == nil {
			options.httpOptions = []*httpOptions{}
		}

		thisHTTPServerOptions := defaultHTTPOptions()
		thisHTTPServerOptions.serverName = serverName
		thisHTTPServerOptions.services = append(thisHTTPServerOptions.services, serverName)

		for _, option := range myHTTPOptions {
			err := option(thisHTTPServerOptions)
			if err != nil {
				return err
			}
		}

		options.httpOptions = append(options.httpOptions, thisHTTPServerOptions)

		return nil
	}
}

// WithHTTPListeningAddress configures the HTTP server's listening address
func WithHTTPListeningAddress(addr string) HTTPOption {
	return func(h *httpOptions) error {
		h.listeningAddress = addr

		return nil
	}
}

// WithMetricsAggregation enables path aggregation of endpoint.
// For example when using asterisk (*) in path and endpoint unpacks all possible values
// it will aggregate it back to asterisk (*)
func WithMetricsAggregation(metricsAggregation bool) HTTPOption {
	return func(h *httpOptions) error {
		h.metricsAggregation = metricsAggregation
		return nil
	}
}

// WithGlobalMiddleware adds new global middleware to the HTTP server
// default - metrics, logging and recovery (in this order)
func WithGlobalMiddleware(middlware ...gin.HandlerFunc) HTTPOption {
	return func(h *httpOptions) error {
		h.globalMiddleware = append(h.globalMiddleware, middlware...)

		return nil
	}
}

// WithRoute adds new route to the HTTP server
// returns an error if the path-method combo is already registered
func WithRoute(method, path string, handlers ...gin.HandlerFunc) HTTPOption {
	return WithRoutingGroup(http.RoutingGroup{
		Base: "",
		Routes: map[string]map[string][]gin.HandlerFunc{
			path: {
				method: handlers,
			},
		},
	})
}

// WithRoutingGroup adds a new routing group to the HTTP server
// may cause gin configuration eror at runtime. use with care
func WithRoutingGroup(group http.RoutingGroup) HTTPOption {
	return func(h *httpOptions) (err error) {
		g, ok := h.routingGroups[group.Base]
		if !ok {
			automaticMethods(group)
			h.routingGroups[group.Base] = group
			return nil
		}
		group, err = mergeRoutingGroups(g, group)
		if err != nil {
			return err
		}
		automaticMethods(group)
		h.routingGroups[group.Base] = group

		return nil
	}
}

func WithoutLoggingMiddleware() HTTPOption {
	return func(h *httpOptions) error {
		h.enableLoggingMiddleware = false
		return nil
	}
}

func WithoutMetricsMiddleware() HTTPOption {
	return func(h *httpOptions) error {
		h.enableMetricsMiddleware = false
		return nil
	}
}

// ---------------------------------------------------------------
func automaticMethods(group http.RoutingGroup) {
	for path, methodHandlers := range group.Routes {
		getHandlers, ok := group.Routes[path]["GET"]
		if ok {
			_, ok = methodHandlers["HEAD"]
			if !ok {
				methodHandlers["HEAD"] = getHandlers
			}
		}
	}
}

func mergeRoutingGroups(old, _new http.RoutingGroup) (merged http.RoutingGroup, err error) {
	var ok bool
	// TODO: deduplicate middleware
	old.Middleware = append(old.Middleware, _new.Middleware...)
	old.Static = append(old.Static, _new.Static...)

	for path, methodHandlers := range _new.Routes {
		_, ok = old.Routes[path]
		if !ok {
			old.Routes[path] = map[string][]gin.HandlerFunc{}
		}

		for method, handlers := range methodHandlers {
			_, ok = old.Routes[path][method]
			if ok {
				err = fmt.Errorf("conflicting path already registered: path = `%s`; method = `%s`", path, method)
				return
			}

			old.Routes[path][method] = handlers
		}
	}

outer_loop:
	for _, newSubGroup := range _new.Groups {
		for i, oldSubGroup := range old.Groups {
			if oldSubGroup.Base != newSubGroup.Base {
				continue // not it => try next
			}

			old.Groups[i], err = mergeRoutingGroups(oldSubGroup, newSubGroup)
			if err != nil {
				return
			}

			continue outer_loop // merged => continue with new newSubGroup
		}

		old.Groups = append(old.Groups, newSubGroup)
	}

	merged = old

	return
}
