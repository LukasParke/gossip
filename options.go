package gossip

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/gossip-lsp/gossip/middleware"
	"github.com/gossip-lsp/gossip/transport"
	"github.com/gossip-lsp/gossip/treesitter"
)

// Option configures a Server during construction.
type Option func(*Server)

// ServeOption configures how the server is served.
type ServeOption func(*serveConfig)

type serveConfig struct {
	transport        transport.Transport
	transportFactory func() (transport.Transport, error)
}

// WithStdio configures the server to communicate over stdin/stdout.
func WithStdio() ServeOption {
	return func(cfg *serveConfig) {
		cfg.transport = transport.Stdio()
	}
}

// WithTransport configures the server to use a specific transport.
func WithTransport(t transport.Transport) ServeOption {
	return func(cfg *serveConfig) {
		cfg.transport = t
	}
}

// WithLogger sets a custom slog logger on the server.
func WithLogger(l *slog.Logger) Option {
	return func(s *Server) {
		s.logger = l
	}
}

// WithTreeSitter enables native tree-sitter integration. Parsers are created
// per-document and automatically perform incremental re-parsing on edits.
// A DiagnosticEngine is created that runs registered Checks and Analyzers
// incrementally and auto-publishes diagnostics.
func WithTreeSitter(cfg treesitter.Config) Option {
	return func(s *Server) {
		s.tsManager = treesitter.NewManager(cfg, s.docStore)
		s.diagEngine = treesitter.NewDiagnosticEngine(s.tsManager, s.docStore, s.logger)
	}
}

// WithMiddleware adds middleware to the server's dispatch chain.
// Middleware is applied in order: the first middleware is outermost.
func WithMiddleware(mws ...middleware.Middleware) Option {
	return func(s *Server) {
		s.middlewares = append(s.middlewares, mws...)
	}
}

// WithTCP configures the server to listen on a TCP address (e.g., ":9257").
func WithTCP(addr string) ServeOption {
	return func(cfg *serveConfig) {
		cfg.transportFactory = func() (transport.Transport, error) {
			return transport.ListenTCP(addr)
		}
	}
}

// WithSocket configures the server to listen on a Unix domain socket.
func WithSocket(path string) ServeOption {
	return func(cfg *serveConfig) {
		cfg.transportFactory = func() (transport.Transport, error) {
			return transport.ListenSocket(path)
		}
	}
}

// WithPipe configures the server to listen on a named pipe (or Unix socket on non-Windows).
func WithPipe(name string) ServeOption {
	return func(cfg *serveConfig) {
		cfg.transportFactory = func() (transport.Transport, error) {
			return transport.ListenPipe(name)
		}
	}
}

// WithWebSocket configures the server to listen for WebSocket connections.
func WithWebSocket(addr string) ServeOption {
	return func(cfg *serveConfig) {
		cfg.transportFactory = func() (transport.Transport, error) {
			return transport.ListenWebSocket(addr)
		}
	}
}

// WithNodeIPC configures the server for Node.js IPC (VS Code extension host).
func WithNodeIPC() ServeOption {
	return func(cfg *serveConfig) {
		cfg.transport = transport.NodeIPC()
	}
}

// FromArgs parses os.Args to determine the transport. Supported flags:
//
//	--stdio               (default)
//	--tcp :PORT
//	--socket PATH
//	--pipe NAME
//	--ws :PORT
//	--node-ipc
func FromArgs() ServeOption {
	return func(cfg *serveConfig) {
		args := os.Args[1:]
		for i := 0; i < len(args); i++ {
			arg := args[i]
			nextArg := func() string {
				if i+1 < len(args) {
					i++
					return args[i]
				}
				return ""
			}
			switch {
			case arg == "--stdio":
				cfg.transport = transport.Stdio()
				return
			case arg == "--tcp":
				addr := nextArg()
				if addr == "" {
					fmt.Fprintln(os.Stderr, "gossip: --tcp requires an address (e.g., :9257)")
					os.Exit(1)
				}
				cfg.transportFactory = func() (transport.Transport, error) {
					return transport.ListenTCP(addr)
				}
				return
			case strings.HasPrefix(arg, "--tcp="):
				addr := strings.TrimPrefix(arg, "--tcp=")
				cfg.transportFactory = func() (transport.Transport, error) {
					return transport.ListenTCP(addr)
				}
				return
			case arg == "--socket":
				path := nextArg()
				if path == "" {
					fmt.Fprintln(os.Stderr, "gossip: --socket requires a path")
					os.Exit(1)
				}
				cfg.transportFactory = func() (transport.Transport, error) {
					return transport.ListenSocket(path)
				}
				return
			case strings.HasPrefix(arg, "--socket="):
				path := strings.TrimPrefix(arg, "--socket=")
				cfg.transportFactory = func() (transport.Transport, error) {
					return transport.ListenSocket(path)
				}
				return
			case arg == "--pipe":
				name := nextArg()
				if name == "" {
					fmt.Fprintln(os.Stderr, "gossip: --pipe requires a name")
					os.Exit(1)
				}
				cfg.transportFactory = func() (transport.Transport, error) {
					return transport.ListenPipe(name)
				}
				return
			case strings.HasPrefix(arg, "--pipe="):
				name := strings.TrimPrefix(arg, "--pipe=")
				cfg.transportFactory = func() (transport.Transport, error) {
					return transport.ListenPipe(name)
				}
				return
			case arg == "--ws":
				addr := nextArg()
				if addr == "" {
					fmt.Fprintln(os.Stderr, "gossip: --ws requires an address (e.g., :9258)")
					os.Exit(1)
				}
				cfg.transportFactory = func() (transport.Transport, error) {
					return transport.ListenWebSocket(addr)
				}
				return
			case strings.HasPrefix(arg, "--ws="):
				addr := strings.TrimPrefix(arg, "--ws=")
				cfg.transportFactory = func() (transport.Transport, error) {
					return transport.ListenWebSocket(addr)
				}
				return
			case arg == "--node-ipc":
				cfg.transport = transport.NodeIPC()
				return
			}
		}
		// Default: stdio
		cfg.transport = transport.Stdio()
	}
}
