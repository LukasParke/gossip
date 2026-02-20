package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/gossip-lsp/gossip/jsonrpc"
)

// Logging returns middleware that logs each request's method, duration, and errors.
func Logging(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, method string, params jsonrpc.RawMessage) (interface{}, error) {
			start := time.Now()
			result, err := next(ctx, method, params)
			duration := time.Since(start)

			attrs := []slog.Attr{
				slog.String("method", method),
				slog.Duration("duration", duration),
			}
			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
				logger.LogAttrs(ctx, slog.LevelError, "request failed", attrs...)
			} else {
				logger.LogAttrs(ctx, slog.LevelDebug, "request handled", attrs...)
			}

			return result, err
		}
	}
}
