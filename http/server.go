package http

import (
	"context"
	"net/http"

	"github.com/moderntv/cadre/http/middleware"
	"github.com/moderntv/cadre/http/responses"
	"github.com/moderntv/cadre/metrics"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type RoutingGroup struct {
	Base       string
	Middleware []gin.HandlerFunc
	Routes     map[string]map[string][]gin.HandlerFunc // path:method:handlers
}

type HttpServer struct {
	addr string

	router *gin.Engine
}

func init() {
	gin.SetMode(gin.ReleaseMode)
}

func NewHttpServer(ctx context.Context, addr, serverName string, logger zerolog.Logger, metricsRegistry *metrics.Registry, middlewares ...gin.HandlerFunc) (server *HttpServer, err error) {
	server = &HttpServer{
		addr: addr,

		router: gin.New(),
	}

	metricsMiddleware, err := middleware.NewMetrics(metricsRegistry, serverName)
	if err != nil {
		return
	}

	middlewares = append([]gin.HandlerFunc{
		metricsMiddleware,
		middleware.NewLogger(logger),
		gin.Recovery(),
	}, middlewares...)

	server.router.Use(middlewares...)

	// CORS example
	// // g.Use(cors.Default())
	// corsConfig := cors.DefaultConfig()
	// // corsConfig.AllowOrigins = config.Config.Cors.AllowOrigins
	// corsConfig.AllowAllOrigins = true
	// corsConfig.AllowMethods = config.Config.Cors.AllowMethods
	// corsConfig.AllowHeaders = config.Config.Cors.AllowHeaders
	// corsConfig.AllowCredentials = config.Config.Cors.AllowCredentials
	// g.Use(cors.New(corsConfig))
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
	g := server.router.Group(group.Base, group.Middleware...)
	for path, methodHandlers := range group.Routes {
		for method, handlers := range methodHandlers {
			g.Handle(method, path, handlers...)
		}
	}

	return nil
}

func (server *HttpServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	server.router.ServeHTTP(w, req)
}
