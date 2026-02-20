// Package middleware provides composable middleware for gossip LSP servers.
// Middleware wraps the JSON-RPC dispatch layer, allowing cross-cutting concerns
// like logging, panic recovery, and tracing to be applied to all handlers.
package middleware

import (
	"context"

	"github.com/gossip-lsp/gossip/jsonrpc"
)

// Handler processes a JSON-RPC method call and returns a result.
type Handler func(ctx context.Context, method string, params jsonrpc.RawMessage) (interface{}, error)

// Middleware wraps a Handler to add cross-cutting behavior.
type Middleware func(Handler) Handler

// Chain composes multiple middleware into a single middleware.
// Middleware is applied in the order given: the first middleware in the slice
// is the outermost wrapper (executes first).
func Chain(mws ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}
