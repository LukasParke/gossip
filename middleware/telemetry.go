package middleware

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gossip-lsp/gossip/jsonrpc"
)

// Metrics holds request counts and duration statistics per method.
type Metrics struct {
	mu      sync.RWMutex
	methods map[string]*MethodMetrics
}

// MethodMetrics holds metrics for a single LSP method.
type MethodMetrics struct {
	Count    atomic.Int64
	Errors   atomic.Int64
	TotalNs  atomic.Int64
}

// NewMetrics creates a new metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{methods: make(map[string]*MethodMetrics)}
}

func (m *Metrics) getOrCreate(method string) *MethodMetrics {
	m.mu.RLock()
	mm, ok := m.methods[method]
	m.mu.RUnlock()
	if ok {
		return mm
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if mm, ok := m.methods[method]; ok {
		return mm
	}
	mm = &MethodMetrics{}
	m.methods[method] = mm
	return mm
}

// Snapshot returns a point-in-time copy of all method metrics.
func (m *Metrics) Snapshot() map[string]MethodSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snap := make(map[string]MethodSnapshot, len(m.methods))
	for name, mm := range m.methods {
		snap[name] = MethodSnapshot{
			Count:     mm.Count.Load(),
			Errors:    mm.Errors.Load(),
			TotalTime: time.Duration(mm.TotalNs.Load()),
		}
	}
	return snap
}

// MethodSnapshot is a point-in-time copy of metrics for one method.
type MethodSnapshot struct {
	Count     int64
	Errors    int64
	TotalTime time.Duration
}

// Telemetry returns middleware that collects request count and latency metrics.
func Telemetry(metrics *Metrics) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, method string, params jsonrpc.RawMessage) (interface{}, error) {
			mm := metrics.getOrCreate(method)
			start := time.Now()
			result, err := next(ctx, method, params)
			elapsed := time.Since(start)

			mm.Count.Add(1)
			mm.TotalNs.Add(int64(elapsed))
			if err != nil {
				mm.Errors.Add(1)
			}

			return result, err
		}
	}
}
