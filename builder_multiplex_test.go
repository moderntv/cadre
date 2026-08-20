package cadre

import (
	"context"
	"io"
	"net"
	stdhttp "net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// TestGRPCMultiplexing is the regression test for serving HTTP and gRPC on a single port. gRPC clients speak
// cleartext HTTP/2, which net/http does not negotiate unless it is enabled explicitly - without that the
// requests are parsed as HTTP/1.1 and never reach the gRPC server.
func TestGRPCMultiplexing(t *testing.T) {
	t.Parallel()

	addr := freeAddr(t)

	b, err := NewBuilder(
		"multiplex",
		WithGRPC(WithGRPCMultiplex()),
		WithHTTP("main_http", WithHTTPListeningAddress(addr), pingRoute()),
	)
	require.NoError(t, err)

	c, err := b.Build()
	require.NoError(t, err)

	startCadre(t, c, addr)

	t.Run("serves http on the multiplexed port", func(t *testing.T) {
		t.Parallel()

		status, body := httpGet(t, t.Context(), "http://"+addr+"/ping")

		assert.Equal(t, stdhttp.StatusOK, status)
		assert.Equal(t, "pong", body)
	})

	t.Run("serves grpc on the multiplexed port", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		res, err := grpcHealthCheck(t, ctx, addr)
		require.NoError(t, err)
		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, res.GetStatus())
	})

	t.Run("serves the internal endpoints on the multiplexed port", func(t *testing.T) {
		t.Parallel()

		// metrics and status default to the first http server's address, so they end up multiplexed too.
		metricsStatus, _ := httpGet(t, t.Context(), "http://"+addr+"/metrics")
		assert.Equal(t, stdhttp.StatusOK, metricsStatus)

		statusStatus, _ := httpGet(t, t.Context(), "http://"+addr+"/status")
		assert.Equal(t, stdhttp.StatusOK, statusStatus)
	})
}

// TestGRPCSeparatePort is the control for TestGRPCMultiplexing - the same assertions against the topology
// that keeps the two protocols on their own ports.
func TestGRPCSeparatePort(t *testing.T) {
	t.Parallel()

	httpAddr := freeAddr(t)
	grpcAddr := freeAddr(t)

	b, err := NewBuilder(
		"separate",
		WithGRPC(WithGRPCListeningAddress(grpcAddr)),
		WithHTTP("main_http", WithHTTPListeningAddress(httpAddr), pingRoute()),
	)
	require.NoError(t, err)

	c, err := b.Build()
	require.NoError(t, err)

	startCadre(t, c, httpAddr, grpcAddr)

	t.Run("serves http", func(t *testing.T) {
		t.Parallel()

		status, body := httpGet(t, t.Context(), "http://"+httpAddr+"/ping")

		assert.Equal(t, stdhttp.StatusOK, status)
		assert.Equal(t, "pong", body)
	})

	t.Run("serves grpc", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		res, err := grpcHealthCheck(t, ctx, grpcAddr)
		require.NoError(t, err)
		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, res.GetStatus())
	})

	t.Run("does not serve grpc on the http port", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		_, err := grpcHealthCheck(t, ctx, httpAddr)
		require.Error(t, err)
	})
}

func TestGRPCMultiplexingRejectsListeningAddress(t *testing.T) {
	t.Parallel()

	b, err := NewBuilder(
		"invalid",
		WithGRPC(WithGRPCMultiplex(), WithGRPCListeningAddress(":9000")),
		WithHTTP("main_http", WithHTTPListeningAddress(":8000")),
	)
	require.NoError(t, err)

	_, err = b.Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "standalone listening address")
}

// freeAddr reserves a loopback port and releases it again so that the caller can bind it.
func freeAddr(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig

	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := l.Addr().String()
	require.NoError(t, l.Close())

	return addr
}

// startCadre runs the server in the background, waits for it to accept connections and shuts it down again
// when the test ends.
func startCadre(t *testing.T, c *cadre, addrs ...string) {
	t.Helper()

	stopped := make(chan error, 1)

	go func() {
		stopped <- c.Start()
	}()

	t.Cleanup(func() {
		require.NoError(t, c.Shutdown())

		select {
		case err := <-stopped:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Error("cadre did not shut down in time")
		}
	})

	for _, addr := range addrs {
		waitForTCP(t, addr)
	}
}

func waitForTCP(t *testing.T, addr string) {
	t.Helper()

	dialer := net.Dialer{Timeout: 200 * time.Millisecond}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := dialer.DialContext(t.Context(), "tcp", addr)
		if err == nil {
			require.NoError(t, conn.Close())

			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("nothing is listening on %s", addr)
}

func httpGet(t *testing.T, ctx context.Context, url string) (int, string) {
	t.Helper()

	req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, url, nil)
	require.NoError(t, err)

	res, err := stdhttp.DefaultClient.Do(req)
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, res.Body.Close())
	}()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	return res.StatusCode, string(body)
}

// grpcHealthCheck calls the health service, which Cadre registers on every gRPC server by default. It is a
// real unary RPC, so a successful response proves the whole gRPC stack is reachable at addr.
func grpcHealthCheck(t *testing.T, ctx context.Context, addr string) (*healthpb.HealthCheckResponse, error) {
	t.Helper()

	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, cc.Close())
	})

	return healthpb.NewHealthClient(cc).Check(ctx, &healthpb.HealthCheckRequest{})
}

func pingRoute() HTTPOption {
	return WithRoute("GET", "/ping", func(c *gin.Context) {
		c.String(stdhttp.StatusOK, "pong")
	})
}

// TestGRPCMultiplexingOnSharedAddress covers multiplexing that is triggered implicitly, by pointing the gRPC
// server at an address an HTTP server already uses instead of by calling WithGRPCMultiplex. buildGrpc binds
// that address before the multiplexing branch discards the listener, so the port has to be released again -
// otherwise the HTTP server cannot bind it.
func TestGRPCMultiplexingOnSharedAddress(t *testing.T) {
	t.Parallel()

	addr := freeAddr(t)

	b, err := NewBuilder(
		"shared",
		WithGRPC(WithGRPCListeningAddress(addr)),
		WithHTTP("main_http", WithHTTPListeningAddress(addr), pingRoute()),
	)
	require.NoError(t, err)

	c, err := b.Build()
	require.NoError(t, err)

	startCadre(t, c, addr)

	t.Run("serves http on the shared port", func(t *testing.T) {
		t.Parallel()

		// a leaked listener holds the port without accepting, so this times out rather than being refused.
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		status, body := httpGet(t, ctx, "http://"+addr+"/ping")

		assert.Equal(t, stdhttp.StatusOK, status)
		assert.Equal(t, "pong", body)
	})

	t.Run("serves grpc on the shared port", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		res, err := grpcHealthCheck(t, ctx, addr)
		require.NoError(t, err)
		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, res.GetStatus())
	})
}
