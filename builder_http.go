package cadre

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/moderntv/cadre/http"
)

// HTTP Options
type httpOptions struct {
	listeningAddress string

	globalMiddleware []gin.HandlerFunc
	routingGroups    map[string]http.RoutingGroup
}

func (g *httpOptions) ensure() (err error) {
	return
}

func defaultHTTPOptions() *httpOptions {
	return &httpOptions{
		globalMiddleware: []gin.HandlerFunc{},
		routingGroups:    map[string]http.RoutingGroup{},
	}
}

type HTTPOption func(*httpOptions) error

// WithHTTP enables HTTP server
func WithHTTP(httpOptions ...HTTPOption) Option {
	return func(options *Builder) error {
		if options.httpOptions == nil {
			options.httpOptions = defaultHTTPOptions()
		}

		for _, option := range httpOptions {
			err := option(options.httpOptions)
			if err != nil {
				return err
			}
		}

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
	return func(h *httpOptions) error {
		_, ok := h.routingGroups[""]
		if !ok {
			h.routingGroups[""] = http.RoutingGroup{Base: "", Routes: map[string]map[string][]gin.HandlerFunc{}}
		}
		_, ok = h.routingGroups[""].Routes[path]
		if !ok {
			h.routingGroups[""].Routes[path] = map[string][]gin.HandlerFunc{}
		}
		_, ok = h.routingGroups[""].Routes[path][method]
		if ok {
			return fmt.Errorf("path already registered")
		}

		h.routingGroups[""].Routes[path][method] = handlers

		return nil
	}
}

// WithRoutingGroup adds a new routing group to the HTTP server
// may cause gin configuration eror at runtime. use with care
func WithRoutingGroup(group http.RoutingGroup) HTTPOption {
	return func(h *httpOptions) error {
		g, ok := h.routingGroups[group.Base]
		if !ok {
			h.routingGroups[group.Base] = group
			return nil
		}

		// TODO: middleware
		g.Middleware = append(h.routingGroups[group.Base].Middleware, group.Middleware...)

		for path, methodHandlers := range group.Routes {
			_, ok = h.routingGroups[group.Base].Routes[path]
			if !ok {
				h.routingGroups[group.Base].Routes[path] = map[string][]gin.HandlerFunc{}
			}

			for method, handlers := range methodHandlers {
				_, ok = h.routingGroups[group.Base].Routes[path][method]
				if !ok {
					h.routingGroups[group.Base].Routes[path][method] = []gin.HandlerFunc{}
				}

				h.routingGroups[group.Base].Routes[path][method] = append(h.routingGroups[group.Base].Routes[path][method], handlers...)
			}
		}

		return nil
	}
}
