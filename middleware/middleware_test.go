package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"

	"github.com/LukasParke/gossip/jsonrpc"
)

func TestRecoveryMiddleware(t *testing.T) {
	// Handler that panics
	panicHandler := func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
		panic("test panic")
	}

	// Handler that runs normally
	okHandler := func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
		return "ok", nil
	}

	recovery := Recovery()
	ctx := context.Background()

	t.Run("catches panic and returns internal error", func(t *testing.T) {
		handler := recovery(Handler(panicHandler))
		result, err := handler(ctx, "test/method", nil)
		if result != nil {
			t.Errorf("result = %v, want nil on panic", result)
		}
		if err == nil {
			t.Fatal("err = nil, want jsonrpc.Error on panic")
		}
		rpcErr, ok := err.(*jsonrpc.Error)
		if !ok {
			t.Errorf("err = %T, want *jsonrpc.Error", err)
			return
		}
		if rpcErr.Code != jsonrpc.CodeInternalError {
			t.Errorf("Code = %d, want CodeInternalError (%d)", rpcErr.Code, jsonrpc.CodeInternalError)
		}
		if rpcErr.Message == "" {
			t.Error("Message is empty")
		}
	})

	t.Run("non-panicking handler passes through", func(t *testing.T) {
		handler := recovery(Handler(okHandler))
		result, err := handler(ctx, "test/method", nil)
		if err != nil {
			t.Errorf("err = %v, want nil", err)
		}
		if result != "ok" {
			t.Errorf("result = %v, want \"ok\"", result)
		}
	})
}

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	okHandler := func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
		return "result", nil
	}

	logging := Logging(logger)
	handler := logging(Handler(okHandler))
	ctx := context.Background()

	buf.Reset()
	_, err := handler(ctx, "initialize", nil)
	if err != nil {
		t.Fatalf("handler err = %v", err)
	}

	logOut := buf.String()
	if !bytes.Contains([]byte(logOut), []byte("method=initialize")) {
		t.Errorf("log output %q does not contain method name: %s", logOut, "method=initialize")
	}
}

func TestTelemetryMiddleware(t *testing.T) {
	metrics := NewMetrics()

	okHandler := func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
		return nil, nil
	}

	telemetry := Telemetry(metrics)
	handler := telemetry(Handler(okHandler))
	ctx := context.Background()

	_, _ = handler(ctx, "textDocument/didOpen", nil)
	_, _ = handler(ctx, "textDocument/didOpen", nil)
	_, _ = handler(ctx, "workspace/didChangeConfig", nil)

	snap := metrics.Snapshot()
	if snap["textDocument/didOpen"].Count != 2 {
		t.Errorf("textDocument/didOpen count = %d, want 2", snap["textDocument/didOpen"].Count)
	}
	if snap["workspace/didChangeConfig"].Count != 1 {
		t.Errorf("workspace/didChangeConfig count = %d, want 1", snap["workspace/didChangeConfig"].Count)
	}
}

func TestChain(t *testing.T) {
	var order []string
	var mu sync.Mutex

	record := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
				mu.Lock()
				order = append(order, name)
				mu.Unlock()
				defer func() {
					mu.Lock()
					order = append(order, name)
					mu.Unlock()
				}()
				return next(ctx, method, params)
			}
		}
	}

	inner := record("inner")
	middle := record("middle")
	outer := record("outer")

	baseHandler := func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
		mu.Lock()
		order = append(order, "handler")
		mu.Unlock()
		return nil, nil
	}

	chained := Chain(outer, middle, inner)(Handler(baseHandler))
	ctx := context.Background()

	order = nil
	_, err := chained(ctx, "test", nil)
	if err != nil {
		t.Fatalf("chained handler err = %v", err)
	}

	want := []string{"outer", "middle", "inner", "handler", "inner", "middle", "outer"}
	if len(order) != len(want) {
		t.Errorf("order length = %d, want %d: %v", len(order), len(want), order)
		return
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q (got %v)", i, order[i], want[i], order)
			return
		}
	}
}
