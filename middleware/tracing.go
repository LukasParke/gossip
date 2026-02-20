package middleware

import (
	"context"

	"github.com/gossip-lsp/gossip/jsonrpc"
)

// Tracing returns middleware that creates a span for each request.
// This is a lightweight implementation that sets context values;
// for OpenTelemetry integration, use the otel build tag.
func Tracing() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, method string, params jsonrpc.RawMessage) (interface{}, error) {
			ctx = context.WithValue(ctx, traceMethodKey{}, method)
			return next(ctx, method, params)
		}
	}
}

type traceMethodKey struct{}

// TraceMethod returns the LSP method name from the context, if set by Tracing middleware.
func TraceMethod(ctx context.Context) string {
	if v, ok := ctx.Value(traceMethodKey{}).(string); ok {
		return v
	}
	return ""
}
