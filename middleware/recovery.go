package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/gossip-lsp/gossip/jsonrpc"
)

// Recovery returns middleware that recovers from panics in handlers,
// logs the stack trace, and returns an internal error to the client.
func Recovery(logger ...*slog.Logger) Middleware {
	var log *slog.Logger
	if len(logger) > 0 && logger[0] != nil {
		log = logger[0]
	} else {
		log = slog.Default()
	}

	return func(next Handler) Handler {
		return func(ctx context.Context, method string, params jsonrpc.RawMessage) (result interface{}, err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					log.Error("panic recovered in handler",
						"method", method,
						"panic", fmt.Sprint(r),
						"stack", string(stack),
					)
					err = &jsonrpc.Error{
						Code:    jsonrpc.CodeInternalError,
						Message: fmt.Sprintf("internal error: %v", r),
					}
				}
			}()
			return next(ctx, method, params)
		}
	}
}
