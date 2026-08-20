package cadre

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moderntv/cadre/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoutingGroupMergingWithinServer covers routing groups added to a single HTTP server. Groups sharing a
// Base are merged recursively rather than replacing each other.
func TestRoutingGroupMergingWithinServer(t *testing.T) {
	t.Parallel()

	addr := freeAddr(t)

	b, err := NewBuilder(
		"routing",
		WithMetricsListeningAddress(freeAddr(t)),
		WithStatusListeningAddress(freeAddr(t)),
		WithHTTP(
			"main_http",
			WithHTTPListeningAddress(addr),
			WithRoutingGroup(group("/api", nil, group("/v1", map[string]map[string][]gin.HandlerFunc{
				"/orders": {stdhttp.MethodGet: {handler("orders")}},
			}))),
			// same /api base, same /v1 sub-group - both have to survive the merge
			WithRoutingGroup(group("/api", nil, group("/v1", map[string]map[string][]gin.HandlerFunc{
				"/users": {stdhttp.MethodGet: {handler("users")}},
			}))),
			// a different sub-group of the same base
			WithRoutingGroup(group("/api", nil, group("/v2", map[string]map[string][]gin.HandlerFunc{
				"/orders": {stdhttp.MethodGet: {handler("orders v2")}},
			}))),
		),
	)
	require.NoError(t, err)

	c, err := b.Build()
	require.NoError(t, err)

	server := c.httpServers[addr]
	require.NotNil(t, server)

	for path, want := range map[string]string{
		"/api/v1/orders": "orders",
		"/api/v1/users":  "users",
		"/api/v2/orders": "orders v2",
	} {
		req, err := stdhttp.NewRequestWithContext(t.Context(), stdhttp.MethodGet, path, nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, req)

		assert.Equal(t, stdhttp.StatusOK, w.Code, "route %s", path)
		assert.Equal(t, want, w.Body.String(), "route %s", path)
	}
}

// TestRoutingGroupMergingAcrossServers covers the merge that happens when two WithHTTP calls share a
// listening address. This is the mechanism the internal metrics and status endpoints rely on.
func TestRoutingGroupMergingAcrossServers(t *testing.T) {
	t.Parallel()

	addr := freeAddr(t)

	b, err := NewBuilder(
		"routing",
		WithMetricsListeningAddress(freeAddr(t)),
		WithStatusListeningAddress(freeAddr(t)),
		WithHTTP("first", WithHTTPListeningAddress(addr), WithRoute("GET", "/first", handler("first"))),
		WithHTTP("second", WithHTTPListeningAddress(addr), WithRoute("GET", "/second", handler("second"))),
	)
	require.NoError(t, err)

	c, err := b.Build()
	require.NoError(t, err)

	// the two configurations collapsed into a single server
	assert.Len(t, c.httpServers, 3, "expected the merged server plus the metrics and status servers")

	server := c.httpServers[addr]
	require.NotNil(t, server)

	for path, want := range map[string]string{"/first": "first", "/second": "second"} {
		req, err := stdhttp.NewRequestWithContext(t.Context(), stdhttp.MethodGet, path, nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, req)

		assert.Equal(t, stdhttp.StatusOK, w.Code, "route %s", path)
		assert.Equal(t, want, w.Body.String(), "route %s", path)
	}
}

func TestRoutingGroupConflictIsReported(t *testing.T) {
	t.Parallel()

	t.Run("within a single server", func(t *testing.T) {
		t.Parallel()

		_, err := NewBuilder(
			"conflict",
			WithHTTP(
				"main_http",
				WithHTTPListeningAddress(":8000"),
				WithRoute("GET", "/orders", handler("a")),
				WithRoute("GET", "/orders", handler("b")),
			),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting path already registered")
	})

	t.Run("across servers sharing an address", func(t *testing.T) {
		t.Parallel()

		b, err := NewBuilder(
			"conflict",
			WithHTTP("first", WithHTTPListeningAddress(":8000"), WithRoute("GET", "/orders", handler("a"))),
			WithHTTP("second", WithHTTPListeningAddress(":8000"), WithRoute("GET", "/orders", handler("b"))),
		)
		require.NoError(t, err, "the conflict is only detectable once the servers are merged")

		_, err = b.Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting path already registered")
	})

	t.Run("the same path with different methods is not a conflict", func(t *testing.T) {
		t.Parallel()

		b, err := NewBuilder(
			"no-conflict",
			WithMetricsListeningAddress(freeAddr(t)),
			WithStatusListeningAddress(freeAddr(t)),
			WithHTTP(
				"main_http",
				WithHTTPListeningAddress(freeAddr(t)),
				WithRoute("GET", "/orders", handler("get")),
				WithRoute("POST", "/orders", handler("post")),
			),
		)
		require.NoError(t, err)

		_, err = b.Build()
		require.NoError(t, err)
	})
}

// TestAutomaticHEAD covers the convenience behaviour of registering HEAD alongside every GET.
func TestAutomaticHEAD(t *testing.T) {
	t.Parallel()

	addr := freeAddr(t)

	b, err := NewBuilder(
		"head",
		WithMetricsListeningAddress(freeAddr(t)),
		WithStatusListeningAddress(freeAddr(t)),
		WithHTTP("main_http", WithHTTPListeningAddress(addr), WithRoute("GET", "/ping", handler("pong"))),
	)
	require.NoError(t, err)

	c, err := b.Build()
	require.NoError(t, err)

	req, err := stdhttp.NewRequestWithContext(t.Context(), stdhttp.MethodHead, "/ping", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c.httpServers[addr].Handler.ServeHTTP(w, req)

	assert.Equal(t, stdhttp.StatusOK, w.Code)
}

func handler(body string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.String(stdhttp.StatusOK, body)
	}
}

func group(
	base string,
	routes map[string]map[string][]gin.HandlerFunc,
	subGroups ...http.RoutingGroup,
) http.RoutingGroup {
	return http.RoutingGroup{
		Base:   base,
		Routes: routes,
		Groups: subGroups,
	}
}
