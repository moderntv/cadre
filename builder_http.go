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

	globalMiddleware []gin.HandlerFunc
	routingGroups    map[string]http.RoutingGroup
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

		globalMiddleware: append(h.globalMiddleware, other.globalMiddleware...),
		routingGroups:    h.routingGroups,
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
	var serverMiddlewares = []gin.HandlerFunc{}
	{
		if h.enableMetricsMiddleware {
			var metricsMiddleware gin.HandlerFunc
			metricsMiddleware, err = middleware.NewMetrics(metricsRegistry, h.serverName)
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
func WithHTTP(serverName string, myHttpOptions ...HTTPOption) Option {
	return func(options *Builder) error {
		if options.httpOptions == nil {
			options.httpOptions = []*httpOptions{}
		}

		thisHttpServerOptions := defaultHTTPOptions()
		thisHttpServerOptions.serverName = serverName
		thisHttpServerOptions.services = append(thisHttpServerOptions.services, serverName)

		for _, option := range myHttpOptions {
			err := option(thisHttpServerOptions)
			if err != nil {
				return err
			}
		}

		options.httpOptions = append(options.httpOptions, thisHttpServerOptions)

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

// WithGlobalMiddleware adds new global middleware to the HTTP server
// default - metrics, recovery and logging
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
			h.routingGroups[group.Base] = group
			return nil
		}

		h.routingGroups[group.Base], err = mergeRoutingGroups(g, group)
		if err != nil {
			return err
		}

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
func mergeRoutingGroups(old, _new http.RoutingGroup) (merged http.RoutingGroup, err error) {
	var ok bool
	// TODO: deduplicate middleware
	old.Middleware = append(old.Middleware, _new.Middleware...)

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
