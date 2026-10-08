package cadre

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Per-method gRPC counters have to exist at 0 from the start, otherwise a method that is
// called once per process (a nightly stream, a rare RPC) is born at its final value and
// increase()/rate() in Prometheus never see the call.
func TestGRPCMetricsInitializedBeforeFirstCall(t *testing.T) {
	t.Parallel()

	addr := freeAddr(t)

	b, err := NewBuilder(
		"grpc_metrics",
		WithHTTP("main_http", WithHTTPListeningAddress(addr)),
		WithGRPC(WithGRPCListeningAddress(freeAddr(t))),
	)
	require.NoError(t, err)

	c, err := b.Build()
	require.NoError(t, err)

	w := serve(t, c, addr, "/metrics")
	require.Equal(t, 200, w.Code)

	assert.Contains(
		t,
		w.Body.String(),
		`grpc_server_started_total{grpc_method="Check",grpc_service="grpc.health.v1.Health",grpc_type="unary"} 0`,
	)
}
