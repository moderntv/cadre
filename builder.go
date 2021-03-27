package cadre

import (
	"fmt"
	"log"
	"net"
	stdhttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	channelz "github.com/rantav/go-grpc-channelz"
	grpc_middleware "github.com/rkollar/go-grpc-middleware"
	grpc_zerolog "github.com/rkollar/go-grpc-middleware/logging/zerolog"
	grpc_recovery "github.com/rkollar/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/rkollar/go-grpc-middleware/tags"
	"github.com/rs/zerolog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	channelz_service "google.golang.org/grpc/channelz/service"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/moderntv/cadre/http"
	"github.com/moderntv/cadre/http/middleware"
	"github.com/moderntv/cadre/http/responses"
	"github.com/moderntv/cadre/metrics"
	"github.com/moderntv/cadre/status"
)

type ServiceRegistrator func(*grpc.Server)

type Builder struct {
	name string // app name

	ctx context.Context

	// logging
	logger *zerolog.Logger

	// status
	status               *status.Status
	statusHttpServerAddr string
	statusPath           string

	// prometheus
	metrics                  *metrics.Registry
	prometheusRegistry       *prometheus.Registry
	prometheusHttpServerAddr string
	prometheusPath           string

	grpcOptions *grpcOptions
	httpOptions *httpOptions
}

// NewBuilder creates a new Builder instance and allows the user to configure the Cadre server by various options
func NewBuilder(name string, options ...Option) (b *Builder, err error) {
	b = &Builder{
		name: name,
		ctx:  context.Background(),

		statusPath:     "/status",
		prometheusPath: "/metrics",
	}

	for _, option := range options {
		err = option(b)
		if err != nil {
			err = fmt.Errorf("cannot apply option: %w", err)
			return
		}
	}

	return
}

// Build validates the Builder's configuration and creates a new Cadre server
func (b *Builder) Build() (c *cadre, err error) {
	err = b.ensure()
	if err != nil {
		err = fmt.Errorf("cadre builder validation is invalid: %w", err)
		return
	}
	// logger, status, metrics already exist here no matter what

	ctx, ctxCancel := context.WithCancel(b.ctx)
	c = &cadre{
		ctx:       ctx,
		ctxCancel: ctxCancel,

		logger:  *b.logger,
		status:  b.status,
		metrics: b.metrics,

		httpServers: make(map[string]*httpServer),
	}

	if b.httpOptions == nil && b.grpcOptions == nil {
		err = fmt.Errorf("both grpc and http will be disabled. what do you want me to do?")
		return
	}

	// create and configure grpc server
	if b.grpcOptions != nil {
		err = b.buildGrpc(c)
		if err != nil {
			err = fmt.Errorf("grpc server building failed: ", err)
			return
		}
	}

	// create and configure http server
	var httpServer *http.HttpServer
	if b.httpOptions != nil {
		httpServer, err = b.buildHttp(c, ctx)
	}

	if b.prometheusHttpServerAddr != "" {
		c.prometheusAddr = b.prometheusHttpServerAddr
		m := b.getHttpMux(c, c.prometheusAddr, "prometheus")
		m.Handle(b.prometheusPath, promhttp.HandlerFor(b.prometheusRegistry, promhttp.HandlerOpts{}))
	}

	if httpServer != nil || (b.grpcOptions != nil && b.grpcOptions.multiplexWithHTTP) {
		var h stdhttp.Handler
		if b.grpcOptions != nil && b.grpcOptions.multiplexWithHTTP {
			h = stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				log.Printf("handling http request. protomajor = %v; content-type = %v; headers = %v", r.ProtoMajor, r.Header.Get("content-type"), r.Header)
				if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
					c.grpcServer.ServeHTTP(w, r)
				} else {
					httpServer.ServeHTTP(w, r)
				}
			})
		} else {
			// no multiplexing => use gin "directly"
			h = httpServer
		}

		c.httpServer = &stdhttp.Server{
			Addr:    b.httpOptions.listeningAddress,
			Handler: h,
		}
	}

	return
}

func (b *Builder) ensure() (err error) {
	// check prometheus & metrics. ensure they use the same prometheus registry
	if b.metrics != nil && b.prometheusRegistry != nil {
		err = fmt.Errorf("pass either existing metrics.Registry or prometheus.Registry")
		return
	}
	// initialize metrics
	if b.metrics == nil {
		// if b.prometheusRegistry is nil, metrics will just create a new one
		b.metrics, err = metrics.NewRegistry(b.name, b.prometheusRegistry)
	}
	if b.prometheusRegistry == nil {
		b.prometheusRegistry = b.metrics.GetPrometheusRegistry()
	}

	// basic checks
	if b.logger == nil {
		l := zerolog.Nop()
		b.logger = &l
	}
	if b.status == nil {
		b.status = status.NewStatus("TODO")
	}

	// grpc checks
	if b.grpcOptions != nil {
		err = b.grpcOptions.ensure()
		if err != nil {
			return
		}
	}
	// http checks
	if b.httpOptions != nil {
		err = b.httpOptions.ensure()
		if err != nil {
			return
		}
	}

	// http + grpc checks
	if b.grpcOptions != nil && b.grpcOptions.multiplexWithHTTP && b.httpOptions == nil {
		err = fmt.Errorf("grpc set to be multiplexed with http, but no http server will be created")
		return
	}

	return
}

func (b *Builder) buildGrpc(c *cadre) (err error) {
	// metrics
	grpcMetrics := grpc_prometheus.NewServerMetrics()
	err = b.metrics.Register("grpc", grpcMetrics)
	if err != nil {
		err = fmt.Errorf("cannot register grpc metrics to metrics registry: %w", err)
		return
	}

	// interceptors - always in order: tags, logging, tracing, metrics, custom, recovery => recovery should be always the last one
	unaryInterceptors := []grpc.UnaryServerInterceptor{}
	streamInterceptors := []grpc.StreamServerInterceptor{}
	// logging
	if b.grpcOptions.enableLoggingMiddleware {
		unaryInterceptors = append(
			unaryInterceptors,
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zerolog.UnaryServerInterceptor(*b.logger, b.grpcOptions.loggingMiddlewareOptions...),
		)
		streamInterceptors = append(
			streamInterceptors,
			grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zerolog.StreamServerInterceptor(*b.logger, b.grpcOptions.loggingMiddlewareOptions...),
		)
	}

	// TODO: add tracing middleware
	//metrics middleware
	unaryInterceptors = append(unaryInterceptors, grpcMetrics.UnaryServerInterceptor())
	streamInterceptors = append(streamInterceptors, grpcMetrics.StreamServerInterceptor())

	// add extra interceptors
	unaryInterceptors = append(unaryInterceptors, b.grpcOptions.extraUnaryInterceptors...)
	streamInterceptors = append(streamInterceptors, b.grpcOptions.extraStreamInterceptors...)

	//recovery middleware
	if b.grpcOptions.enableRecoveryMiddleware {
		unaryInterceptors = append(unaryInterceptors, grpc_recovery.UnaryServerInterceptor(b.grpcOptions.recoveryMiddlewareOptions...))
		streamInterceptors = append(streamInterceptors, grpc_recovery.StreamServerInterceptor(b.grpcOptions.recoveryMiddlewareOptions...))
	}

	//
	//create grpc server
	//
	c.grpcAddr = b.grpcOptions.listeningAddress
	c.grpcServer = grpc.NewServer(
		grpc_middleware.WithUnaryServerChain(unaryInterceptors...),
		grpc_middleware.WithStreamServerChain(streamInterceptors...),
	)

	// replace gRPC logger
	grpc_zerolog.ReplaceGrpcLoggerV2(*b.logger)

	// register services
	// health service
	if b.grpcOptions.enableHealthService {
		healthService := health.NewServer()

		healthpb.RegisterHealthServer(c.grpcServer, healthService)
	}

	// reflection
	if b.grpcOptions.enableReflection {
		reflection.Register(c.grpcServer)
	}

	// channelz
	if b.grpcOptions.enableChannelz {
		channelz_service.RegisterChannelzServiceToServer(c.grpcServer)

		grpcAddr := b.grpcOptions.listeningAddress
		if b.grpcOptions.multiplexWithHTTP {
			grpcAddr = b.httpOptions.listeningAddress
		}

		m := b.getHttpMux(c, b.grpcOptions.channelzHttpAddr, "channelz")
		m.Handle("/", channelz.CreateHandler("/", grpcAddr))
		c.channelzAddr = b.grpcOptions.channelzHttpAddr
	}

	// user-specified grpc services
	for _, registrator := range b.grpcOptions.services {
		registrator(c.grpcServer)
	}

	// grpc listener
	if b.grpcOptions.listeningAddress != "" {
		c.grpcListener, err = net.Listen("tcp", b.grpcOptions.listeningAddress)
		if err != nil {
			return
		}
	}

	return
}

func (b *Builder) buildHttp(c *cadre, cadreContext context.Context) (httpServer *http.HttpServer, err error) {
	c.httpAddr = b.httpOptions.listeningAddress

	metricsMiddleware, err := middleware.NewMetrics(b.metrics, "main_http")
	if err != nil {
		return
	}

	middlewares := append([]gin.HandlerFunc{
		metricsMiddleware,
		middleware.NewLogger(*b.logger),
		gin.Recovery(),
	}, b.httpOptions.globalMiddleware...)

	httpServer, err = http.NewHttpServer(cadreContext, b.httpOptions.listeningAddress, middlewares...)
	if err != nil {
		return
	}

	for _, group := range b.httpOptions.routingGroups {
		httpServer.RegisterRouteGroup(group)
	}

	// prometheus
	if b.prometheusHttpServerAddr == "" {
		// empty prometheus listening address => listen in main http server
		httpServer.RegisterRoute(b.prometheusPath, "GET", gin.WrapH(promhttp.HandlerFor(b.prometheusRegistry, promhttp.HandlerOpts{})))
	}
	if b.statusHttpServerAddr == "" {
		httpServer.RegisterRoute(b.statusPath, "GET", func(c *gin.Context) {
			report := b.status.Report()
			if report.Status == status.OK {
				responses.Ok(c, report)
				return
			}

			c.AbortWithStatusJSON(503, gin.H{
				"data": report,
			})
		})
	}

	return
}

func (b *Builder) getHttpMux(c *cadre, listen string, serviceName string) *stdhttp.ServeMux {
	hServer, ok := c.httpServers[listen]
	if ok {
		hServer.Services = append(hServer.Services, serviceName)
		return hServer.Mux
	}

	mux := stdhttp.NewServeMux()
	server := &stdhttp.Server{
		Addr:    listen,
		Handler: mux,
	}

	hServer = &httpServer{
		Services: []string{serviceName},
		Server:   server,
		Mux:      mux,
	}

	c.httpServers[listen] = hServer
	return mux
}
