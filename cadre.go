package cadre

import (
	"context"
	"net"
	stdhttp "net/http"
	"sync"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"

	"github.com/moderntv/cadre/metrics"
	"github.com/moderntv/cadre/status"
)

type Cadre interface {
	Start() error
	Shutdown() error
}

type cadre struct {
	ctx       context.Context
	ctxCancel func()

	// config stuff that is good to know at runtime
	httpAddr       string
	grpcAddr       string
	channelzAddr   string
	prometheusAddr string

	logger  zerolog.Logger
	status  *status.Status
	metrics *metrics.Registry

	grpcHealthService *health.Server

	swg          sync.WaitGroup // services wait group
	grpcServer   *grpc.Server
	grpcListener net.Listener
	// httpServer         *http.HttpServer
	httpServer           *stdhttp.Server
	channelzHttpServer   *stdhttp.Server
	prometheusHttpServer *stdhttp.Server
}

func (c *cadre) Start() error {
	// start http server
	c.swg.Add(1)
	go c.startHttp()

	// start grpc server
	c.swg.Add(1)
	go c.startGRPC()

	// start channelz http server
	c.swg.Add(1)
	go c.startChannelzHttp()
	// start prometheus http server
	c.swg.Add(1)
	go c.startPrometheusHttp()

	c.swg.Wait()
	return nil
}

func (c *cadre) Shutdown() error {
	c.ctxCancel()
	c.swg.Wait()
	return nil
}

func (c *cadre) startGRPC() {
	defer c.swg.Done()

	if c.grpcListener == nil || c.grpcServer == nil {
		c.logger.Trace().Msg("standalone grpc server disabled")

		return
	}

	go func() {
		c.logger.Debug().
			Str("addr", c.grpcAddr).
			Msg("starting grpc server")

		err := c.grpcServer.Serve(c.grpcListener)
		if err != nil {
			c.logger.Error().
				Err(err).
				Msg("grpc server failed")
		}

	}()
	<-c.ctx.Done()
}

func (c *cadre) startHttp() {
	defer c.swg.Done()

	if c.httpServer == nil {
		c.logger.Trace().Msg("standalone http server disabled")
		return
	}

	go func() {
		c.logger.Debug().
			Str("addr", c.httpAddr).
			Msg("starting http server")

		err := c.httpServer.ListenAndServe()
		if err != nil {
			c.logger.Error().
				Err(err).
				Msg("http server failed")
		}

	}()
	<-c.ctx.Done()

	// TODO: cleanup
}

func (c *cadre) startPrometheusHttp() {
	defer c.swg.Done()

	if c.prometheusHttpServer == nil {
		c.logger.Trace().Msg("standalone prometheus http server disabled")
		return
	}

	go func() {
		c.logger.Debug().
			Str("addr", c.prometheusAddr).
			Msg("starting prometheus http server")

		err := c.prometheusHttpServer.ListenAndServe()
		if err != nil {
			c.logger.Error().
				Err(err).
				Msg("prometheus http server failed")
		}

	}()
	<-c.ctx.Done()
}

func (c *cadre) startChannelzHttp() {
	defer c.swg.Done()

	if c.channelzHttpServer == nil {
		c.logger.Trace().Msg("standalone channelz http server disabled")
		return
	}

	go func() {
		c.logger.Debug().
			Str("addr", c.channelzAddr).
			Msg("starting channelz http server")

		err := c.channelzHttpServer.ListenAndServe()
		if err != nil {
			c.logger.Error().
				Err(err).
				Msg("channelz http server failed")
		}

	}()
	<-c.ctx.Done()
}
