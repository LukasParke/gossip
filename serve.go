package gossip

import (
	"context"
	"fmt"

	"github.com/gossip-lsp/gossip/jsonrpc"
	mw "github.com/gossip-lsp/gossip/middleware"
	"github.com/gossip-lsp/gossip/transport"
)


// Serve starts the LSP server using the given transport options.
// If no ServeOption is provided, stdio is used by default.
func Serve(s *Server, opts ...ServeOption) error {
	cfg := &serveConfig{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.transport == nil && cfg.transportFactory != nil {
		var err error
		cfg.transport, err = cfg.transportFactory()
		if err != nil {
			return fmt.Errorf("creating transport: %w", err)
		}
	}
	if cfg.transport == nil {
		cfg.transport = transport.Stdio()
	}

	// Apply server-level options
	for _, o := range s.opts {
		o(s)
	}

	codec := jsonrpc.NewCodec(cfg.transport, cfg.transport)

	// Wrap dispatch with middleware chain
	handler := jsonrpc.Handler(s.dispatch)
	notifHandler := s.dispatchNotification
	if len(s.middlewares) > 0 {
		chain := mw.Chain(s.middlewares...)
		wrappedHandler := chain(mw.Handler(handler))
		handler = jsonrpc.Handler(wrappedHandler)

		notifInner := mw.Handler(func(ctx context.Context, method string, params jsonrpc.RawMessage) (interface{}, error) {
			s.dispatchNotification(ctx, method, params)
			return nil, nil
		})
		wrappedNotif := chain(notifInner)
		notifHandler = func(ctx context.Context, method string, params jsonrpc.RawMessage) {
			wrappedNotif(ctx, method, params)
		}
	}

	conn := jsonrpc.NewConn(codec, handler, notifHandler)
	s.conn = conn
	s.client = newClientProxy(conn)

	// Wire the diagnostic engine publish function now that the client exists.
	if s.diagEngine != nil {
		s.diagEngine.SetPublish(s.client.PublishDiagnostics)
	}

	if s.configHolder != nil {
		defer s.configHolder.close()
	}

	s.logger.Info("gossip server starting",
		"name", s.name,
		"version", s.version,
	)

	ctx := context.Background()
	err := conn.Run(ctx)
	if err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}
