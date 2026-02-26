package gossip

import (
	"context"
	"fmt"

	"github.com/LukasParke/gossip/jsonrpc"
	mw "github.com/LukasParke/gossip/middleware"
	"github.com/LukasParke/gossip/transport"
)


// Serve starts the LSP server using the given transport options and blocks
// until the connection closes. If no ServeOption is provided, stdio is used.
//
// Lifecycle: Serve applies server-level Options (from NewServer), establishes
// the transport, and sets Conn and Client on the Server. Conn() and
// Context.Client are only valid after Serve has begun—run Serve in a goroutine
// if you need to access Conn from elsewhere before the server exits. Serve
// returns when the client disconnects or sends exit.
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
	defer cfg.transport.Close()

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
