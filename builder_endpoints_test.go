package cadre

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/moderntv/cadre/metrics"
	"github.com/moderntv/cadre/status"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusEndpoint covers the behaviour that makes /status usable as a readiness probe - the endpoint has
// to answer 503 as soon as any component reports ERROR.
func TestStatusEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		componentType status.StatusType
		wantHTTP      int
		wantOverall   status.StatusType
	}{
		{"healthy component", status.OK, stdhttp.StatusOK, status.OK},
		{"degraded component still serves", status.WARN, stdhttp.StatusOK, status.WARN},
		{"failed component is not ready", status.ERROR, stdhttp.StatusServiceUnavailable, status.ERROR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			addr := freeAddr(t)

			appStatus := status.NewStatus("1.2.3")

			component, err := appStatus.Register("database")
			require.NoError(t, err)

			component.SetStatus(tt.componentType, "message")

			b, err := NewBuilder(
				"status",
				WithStatus(appStatus),
				WithHTTP("main_http", WithHTTPListeningAddress(addr)),
			)
			require.NoError(t, err)

			c, err := b.Build()
			require.NoError(t, err)

			w := serve(t, c, addr, "/status")
			assert.Equal(t, tt.wantHTTP, w.Code)

			var body struct {
				Data status.Report `json:"data"`
			}

			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tt.wantOverall, body.Data.Status)
			assert.Equal(t, "1.2.3", body.Data.Version)
			assert.Contains(t, body.Data.Components, "database")
		})
	}
}

// TestInternalEndpointPlacement covers where /metrics and /status end up, which is decided by ensure() and
// the address-keyed merging of http servers.
func TestInternalEndpointPlacement(t *testing.T) {
	t.Parallel()

	t.Run("default onto the first http server", func(t *testing.T) {
		t.Parallel()

		addr := freeAddr(t)

		b, err := NewBuilder("placement", WithHTTP("main_http", WithHTTPListeningAddress(addr)))
		require.NoError(t, err)

		c, err := b.Build()
		require.NoError(t, err)

		require.Len(t, c.httpServers, 1, "everything should be merged onto the single http server")
		assert.Equal(t, stdhttp.StatusOK, serve(t, c, addr, "/metrics").Code)
		assert.Equal(t, stdhttp.StatusOK, serve(t, c, addr, "/status").Code)
	})

	t.Run("share one internal server when given the same address", func(t *testing.T) {
		t.Parallel()

		addr := freeAddr(t)
		internalAddr := freeAddr(t)

		b, err := NewBuilder(
			"placement",
			WithMetricsListeningAddress(internalAddr),
			WithStatusListeningAddress(internalAddr),
			WithHTTP("main_http", WithHTTPListeningAddress(addr)),
		)
		require.NoError(t, err)

		c, err := b.Build()
		require.NoError(t, err)

		require.Len(t, c.httpServers, 2)
		assert.Equal(t, stdhttp.StatusOK, serve(t, c, internalAddr, "/metrics").Code)
		assert.Equal(t, stdhttp.StatusOK, serve(t, c, internalAddr, "/status").Code)
		// and they are no longer reachable on the main server
		assert.Equal(t, stdhttp.StatusNotFound, serve(t, c, addr, "/metrics").Code)
		assert.Equal(t, stdhttp.StatusNotFound, serve(t, c, addr, "/status").Code)
	})

	t.Run("split onto separate servers when given different addresses", func(t *testing.T) {
		t.Parallel()

		addr := freeAddr(t)
		metricsAddr := freeAddr(t)
		statusAddr := freeAddr(t)

		b, err := NewBuilder(
			"placement",
			WithMetricsListeningAddress(metricsAddr),
			WithStatusListeningAddress(statusAddr),
			WithHTTP("main_http", WithHTTPListeningAddress(addr)),
		)
		require.NoError(t, err)

		c, err := b.Build()
		require.NoError(t, err)

		require.Len(t, c.httpServers, 3)
		assert.Equal(t, stdhttp.StatusOK, serve(t, c, metricsAddr, "/metrics").Code)
		assert.Equal(t, stdhttp.StatusOK, serve(t, c, statusAddr, "/status").Code)
		assert.Equal(t, stdhttp.StatusNotFound, serve(t, c, metricsAddr, "/status").Code)
	})
}

func TestBuildRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("neither http nor grpc", func(t *testing.T) {
		t.Parallel()

		b, err := NewBuilder("empty")
		require.NoError(t, err)

		_, err = b.Build()
		require.Error(t, err)
	})

	t.Run("both metrics and prometheus registry", func(t *testing.T) {
		t.Parallel()

		metricsRegistry, err := metrics.NewRegistry("both", nil)
		require.NoError(t, err)

		b, err := NewBuilder(
			"both",
			WithMetricsRegistry(metricsRegistry),
			WithPrometheusRegistry(prometheus.NewRegistry()),
			WithHTTP("main_http", WithHTTPListeningAddress(":8000")),
		)
		require.NoError(t, err)

		_, err = b.Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not both")
	})

	t.Run("grpc without an address and without multiplexing", func(t *testing.T) {
		t.Parallel()

		b, err := NewBuilder("grpc", WithGRPC())
		require.NoError(t, err)

		_, err = b.Build()
		require.Error(t, err)
	})
}

func serve(t *testing.T, c *cadre, addr, path string) *httptest.ResponseRecorder {
	t.Helper()

	server, ok := c.httpServers[addr]
	require.True(t, ok, "no http server on %s", addr)

	req, err := stdhttp.NewRequestWithContext(t.Context(), stdhttp.MethodGet, path, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	return w
}
