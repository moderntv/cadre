package consul

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moderntv/cadre/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testService = "greeter"
	node1       = "node1"
	node2       = "node2"
	node3       = "node3"
)

// fakeConsul serves the health endpoint with the configured nodes, in the configured order.
type fakeConsul struct {
	mu        sync.Mutex
	nodes     []string
	lastQuery url.Values
}

func (fc *fakeConsul) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.lastQuery = r.URL.Query()

	entries := make([]map[string]any, 0, len(fc.nodes))
	for _, node := range fc.nodes {
		entries = append(entries, map[string]any{
			"Node":    map[string]any{"Node": node},
			"Service": map[string]any{"Service": testService, "Port": 9000},
		})
	}

	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(entries)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (fc *fakeConsul) setNodes(nodes ...string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.nodes = nodes
}

func (fc *fakeConsul) query() url.Values {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	return fc.lastQuery
}

func newTestRegistry(t *testing.T, fc *fakeConsul, opts ...Option) *consulRegistry {
	t.Helper()

	srv := httptest.NewServer(fc)
	t.Cleanup(srv.Close)

	r, err := NewRegistry(strings.TrimPrefix(srv.URL, "http://"), "dc1", nil, time.Hour, opts...)
	require.NoError(t, err)

	return r.(*consulRegistry)
}

// nodeNames returns the node names of instances, in order.
func nodeNames(is []registry.Instance) (nodes []string) {
	for _, i := range is {
		nodes = append(nodes, strings.TrimSuffix(i.Address(), ":9000"))
	}

	return
}

func TestNewRegistry_Options(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{name: "no options"},
		{name: "near", opts: []Option{WithNear("_agent")}},
		{name: "near and limit", opts: []Option{WithNear("_agent"), WithLimit(2)}},
		{name: "empty near", opts: []Option{WithNear("")}, wantErr: true},
		{name: "zero limit", opts: []Option{WithNear("_agent"), WithLimit(0)}, wantErr: true},
		{name: "limit without near", opts: []Option{WithLimit(2)}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRegistry("127.0.0.1:8500", "dc1", nil, time.Second, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistry_Near(t *testing.T) {
	t.Parallel()

	t.Run("sends near and keeps the order", func(t *testing.T) {
		t.Parallel()

		fc := &fakeConsul{}
		fc.setNodes(node3, node1, node2)
		cr := newTestRegistry(t, fc, WithNear("_agent"))

		_, stop := cr.Watch(testService)
		defer stop()

		assert.Equal(t, "_agent", fc.query().Get("near"))
		assert.Equal(t, []string{node3, node1, node2}, nodeNames(cr.Instances(testService)))
	})

	t.Run("no near by default", func(t *testing.T) {
		t.Parallel()

		fc := &fakeConsul{}
		fc.setNodes(node1)
		cr := newTestRegistry(t, fc)

		_, stop := cr.Watch(testService)
		defer stop()

		assert.False(t, fc.query().Has("near"))
	})

	t.Run("limit keeps the nearest instances", func(t *testing.T) {
		t.Parallel()

		fc := &fakeConsul{}
		fc.setNodes(node3, node1, node2)
		cr := newTestRegistry(t, fc, WithNear("_agent"), WithLimit(2))

		_, stop := cr.Watch(testService)
		defer stop()

		assert.Equal(t, []string{node3, node1}, nodeNames(cr.Instances(testService)))
	})
}

func TestRegistry_WriteChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		limit       int
		before      []string
		after       []string
		wantChanges []registry.RegistryChangeType
		wantNodes   []string
	}{
		{
			name:      "unchanged",
			before:    []string{node1, node2},
			after:     []string{node1, node2},
			wantNodes: []string{node1, node2},
		},
		{
			name:        "reordered",
			before:      []string{node1, node2},
			after:       []string{node2, node1},
			wantChanges: []registry.RegistryChangeType{registry.RCTReordered},
			wantNodes:   []string{node2, node1},
		},
		{
			name:        "nearest instance replaced within limit",
			limit:       2,
			before:      []string{node1, node2, node3},
			after:       []string{node3, node1, node2},
			wantChanges: []registry.RegistryChangeType{registry.RCTDeregistered, registry.RCTRegistered},
			wantNodes:   []string{node3, node1},
		},
		{
			name:        "instance added",
			before:      []string{node1},
			after:       []string{node1, node2},
			wantChanges: []registry.RegistryChangeType{registry.RCTRegistered},
			wantNodes:   []string{node1, node2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := []Option{WithNear("_agent")}
			if tt.limit > 0 {
				opts = append(opts, WithLimit(tt.limit))
			}

			fc := &fakeConsul{}
			cr := newTestRegistry(t, fc, opts...)
			ch := make(chan registry.RegistryChange, 16)

			fc.setNodes(tt.before...)
			cr.resolveService(testService, testService, ch)

			for len(ch) > 0 {
				<-ch
			}

			fc.setNodes(tt.after...)
			cr.resolveService(testService, testService, ch)
			close(ch)

			var changes []registry.RegistryChangeType
			for c := range ch {
				changes = append(changes, c.Type)
			}

			assert.Equal(t, tt.wantChanges, changes)
			assert.Equal(t, tt.wantNodes, nodeNames(cr.Instances(testService)))
		})
	}
}
