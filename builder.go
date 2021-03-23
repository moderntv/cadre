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
	}

	if b.httpOptions == nil && b.grpcOptions == nil {
		err = fmt.Errorf("both grpc and http will be disabled. what do you want me to do?")
		return
	}

	// create and configure grpc server
	var (
		healthService *health.Server
	)
	if b.grpcOptions != nil {
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
			// TODO: add zerolog
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
			healthService = health.NewServer()

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

			m := stdhttp.NewServeMux()
			m.Handle("/", channelz.CreateHandler("/", grpcAddr))
			c.channelzHttpServer = &stdhttp.Server{
				Addr:    b.grpcOptions.channelzHttpAddr,
				Handler: m,
			}
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
	}

	// create and configure http server
	var httpServer *http.HttpServer
	if b.httpOptions != nil {
		c.httpAddr = b.httpOptions.listeningAddress
		httpServer, err = http.NewHttpServer(ctx, b.httpOptions.listeningAddress, "main_http", *b.logger, b.metrics, b.httpOptions.globalMiddleware...)
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
	}
	if b.prometheusHttpServerAddr != "" {
		c.prometheusAddr = b.prometheusHttpServerAddr

		c.prometheusHttpServer = &stdhttp.Server{
			Addr:    c.prometheusAddr,
			Handler: promhttp.HandlerFor(b.prometheusRegistry, promhttp.HandlerOpts{}),
		}
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

type Option func(*Builder) error

// WithContext supplies custom context for the cadre server. This is useful for graceful shutdown
func WithContext(ctx context.Context) Option {
	return func(options *Builder) error {
		options.ctx = ctx

		return nil
	}
}

// WithLogger allows configuring cadre with custom zerolog logger.
// If not used Cadre will be configured with zerolog.Nop()
func WithLogger(logger *zerolog.Logger) Option {
	return func(options *Builder) error {
		options.logger = logger

		return nil
	}
}

// WithStatus allows replacing the default status with a custom pre-configured one
func WithStatus(status *status.Status) Option {
	return func(options *Builder) error {
		options.status = status

		return nil
	}
}

// WithMetrics allows replacing the default metrics registry with a custom pre-configured one.
// If used, the prometheus registry from metrics Registry will replace the default prometheus registry.
// Do not use with WithPrometheus
func WithMetrics(metrics *metrics.Registry) Option {
	return func(options *Builder) error {
		options.metrics = metrics

		return nil
	}
}

// WithPrometheus configures cadre to use a specific prometheus registry.
// This prometheus registry will be used to create metrics registry.
func WithPrometheus(registry *prometheus.Registry) Option {
	return func(options *Builder) error {
		options.prometheusRegistry = registry

		return nil
	}
}

// WithPrometheusListeningAddress is meant to configure cadre to use
// a separate http server for prometheus - useful for putting it behind firewall
func WithPrometheusListeningAddress(serverListeningAddress string) Option {
	return func(options *Builder) error {
		options.prometheusHttpServerAddr = serverListeningAddress

		return nil
	}
}

// GRPC Options
type grpcOptions struct {
	listeningAddress string
	// whether the grpc server should be on the same http server as the main http server
	multiplexWithHTTP bool

	services map[string]ServiceRegistrator

	// whether enable recovery middleware
	enableRecoveryMiddleware  bool
	recoveryMiddlewareOptions []grpc_recovery.Option

	// logging middleware
	enableLoggingMiddleware  bool
	loggingMiddlewareOptions []grpc_zerolog.Option

	// whether to register the grpc health service
	enableHealthService bool

	// whether to enable reflection
	enableReflection bool

	// whether grpc channelz should be enabled
	enableChannelz   bool
	channelzHttpAddr string

	// allow registration of custom interceptors
	extraUnaryInterceptors  []grpc.UnaryServerInterceptor
	extraStreamInterceptors []grpc.StreamServerInterceptor
}

func defaultGRPCOptions() *grpcOptions {
	return &grpcOptions{
		services:                 map[string]ServiceRegistrator{},
		enableRecoveryMiddleware: true,
		enableLoggingMiddleware:  true,
		enableHealthService:      true,
		enableReflection:         true,
		enableChannelz:           true,
		channelzHttpAddr:         ":8192",
		extraUnaryInterceptors:   []grpc.UnaryServerInterceptor{},
		extraStreamInterceptors:  []grpc.StreamServerInterceptor{},
	}
}

func (g *grpcOptions) ensure() (err error) {
	if g.listeningAddress == "" && !g.multiplexWithHTTP {
		err = fmt.Errorf("grpc server has to either have listening address or be set up to be multiplexed with http server")
		return
	}
	if g.listeningAddress != "" && g.multiplexWithHTTP {
		err = fmt.Errorf("grpc can be either configured with a standalone listening address or to be multiplexed with other grpc")
		return
	}

	return
}

type GRPCOption func(*grpcOptions) error

// WithGRPC configures cadre with a GRPC server
func WithGRPC(grpcOptions ...GRPCOption) Option {
	return func(b *Builder) error {
		if b.grpcOptions == nil {
			b.grpcOptions = defaultGRPCOptions()
		}

		for _, option := range grpcOptions {
			err := option(b.grpcOptions)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// WithGRPCListeningAddress configures gRPC's standalone server's listening address
func WithGRPCListeningAddress(addr string) GRPCOption {
	return func(g *grpcOptions) error {
		g.listeningAddress = addr

		return nil
	}
}

// WithGRPCMultiplex configures Cadre to multiplex grpc and http on the same port
func WithGRPCMultiplex() GRPCOption {
	return func(g *grpcOptions) error {
		g.multiplexWithHTTP = true

		return nil
	}
}

// WithService registers a new gRPC service to the Cadre's gRPC server
func WithService(name string, registrator ServiceRegistrator) GRPCOption {
	return func(g *grpcOptions) error {
		_, ok := g.services[name]
		if ok {
			return fmt.Errorf("service already registered to grpc server")
		}

		g.services[name] = registrator

		return nil
	}
}

// WithoutLogging disables logging middleware - default on
func WithoutLogging() GRPCOption {
	return func(g *grpcOptions) error {
		g.enableLoggingMiddleware = false

		return nil
	}
}

func WithLoggingOptions(opts []grpc_zerolog.Option) GRPCOption {
	return func(g *grpcOptions) error {
		g.loggingMiddlewareOptions = opts

		return nil
	}
}

// WithoutReflection disables gRPC's reflection service - default on
func WithoutReflection() GRPCOption {
	return func(g *grpcOptions) error {
		g.enableReflection = false

		return nil
	}
}

// WithoutChannelz disables gRPC's channelz http server
func WithoutChannelz() GRPCOption {
	return func(g *grpcOptions) error {
		g.enableChannelz = false

		return nil
	}
}

// WithoutRecovery disables gRPC's recovery middleware
func WithoutRecovery() GRPCOption {
	return func(g *grpcOptions) error {
		g.enableRecoveryMiddleware = false

		return nil
	}
}

// WithRecoveryOptions configures gRPC's recovery middleware with custom options
func WithRecoveryOptions(opts []grpc_recovery.Option) GRPCOption {
	return func(g *grpcOptions) error {
		g.recoveryMiddlewareOptions = opts

		return nil
	}
}

// WithChannelzListeningAddress configures the channelz's listening address
func WithChannelzListeningAddress(listenAddr string) GRPCOption {
	return func(g *grpcOptions) error {
		g.channelzHttpAddr = listenAddr

		return nil
	}
}

// WithUnaryInterceptors adds custom grpc unary interceptor(s) to the end of the interceptor chain
// default order (if not disabled) - ctxtags, logging, recovery, metrics
func WithUnaryInterceptors(unaryInterceptors ...grpc.UnaryServerInterceptor) GRPCOption {
	return func(g *grpcOptions) error {
		g.extraUnaryInterceptors = append(g.extraUnaryInterceptors, unaryInterceptors...)

		return nil
	}
}

// WithStreamInterceptors adds custom grpc stream interceptor(s) to the end of the interceptor chain
// default order (if not disabled) - ctxtags, logging, recovery, metrics
func WithStreamInterceptors(streamInterceptors ...grpc.StreamServerInterceptor) GRPCOption {
	return func(g *grpcOptions) error {
		g.extraStreamInterceptors = append(g.extraStreamInterceptors, streamInterceptors...)

		return nil
	}
}

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
