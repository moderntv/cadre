package http

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/moderntv/cadre/http/responses"
	"github.com/rs/zerolog"
)

type HttpServer struct {
	name string
	addr string
	log  zerolog.Logger

	router *gin.Engine
}

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// NewHttpServer creates new HttpServer using gin router. Automatically registrates given middlewares and exposes also 404 NotFound route when accessing non existent handlers.
func NewHttpServer(
	ctx context.Context,
	name, addr string,
	log zerolog.Logger,
	options []gin.OptionFunc,
	middlewares ...gin.HandlerFunc,
) (server *HttpServer, err error) {
	server = &HttpServer{
		name: name,
		addr: addr,
		log:  log.With().Str("component", "http/"+name).Logger(),

		router: gin.New(options...),
	}

	server.router.Use(middlewares...)
	server.router.NoRoute(func(c *gin.Context) {
		responses.NotFound(c, responses.Error{
			Type:    "NO_ROUTE",
			Message: "No such route",
			Data:    c.Request.RequestURI,
		})
	})

	return
}

func (server *HttpServer) Start() error {
	return server.router.Run(server.addr)
}

func (server *HttpServer) RegisterGlobalMiddleware(handlers ...gin.HandlerFunc) error {
	server.router.Use(handlers...)

	return nil
}

func (server *HttpServer) RegisterRoute(path, method string, handlers ...gin.HandlerFunc) error {
	server.router.Handle(method, path, handlers...)

	return nil
}

func (server *HttpServer) RegisterRouteGroup(group RoutingGroup) error {
	return group.Register(server.router)
}

func (server *HttpServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	server.router.ServeHTTP(w, req)
}

func (server *HttpServer) Name() string {
	return server.name
}

func (server *HttpServer) Address() string {
	return server.addr
}

func (server *HttpServer) LogRegisteredRoutes() {
	routes := server.router.Routes()
	for _, route := range routes {
		server.log.Trace().Str("method", route.Method).Str("path", route.Path).Msg("route registered")
	}
}
