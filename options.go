package gossip

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/middleware"
	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/transport"
	"github.com/LukasParke/gossip/treesitter"
)

// Option configures a Server during construction.
type Option func(*Server)

// ServeOption configures how the server is served.
type ServeOption func(*serveConfig)

type serveConfig struct {
	transport        transport.Transport
	transportFactory func() (transport.Transport, error)
}

// WithStdio configures the server to communicate over stdin/stdout. Use this
// for editor integrations (e.g., VS Code extensions) that spawn the server and
// attach via standard streams. This is the typical transport for LSP clients.
func WithStdio() ServeOption {
	return func(cfg *serveConfig) {
		cfg.transport = transport.Stdio()
	}
}

// WithTransport configures the server to use the given transport implementation.
// Use this when you need full control over the transport (e.g., custom IPC or
// wrapping an existing connection). The transport must not be nil.
func WithTransport(t transport.Transport) ServeOption {
	return func(cfg *serveConfig) {
		cfg.transport = t
	}
}

// WithLogger sets a custom slog logger on the server. Use this to control
// log output format, level, or destination. If not set, the server uses a
// default text handler writing to stderr at Info level.
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

// WithTreeSitterLanguage adds a single tree-sitter language mapping by file
// extension. It creates the Manager and DiagnosticEngine if not already
// present, otherwise registers the language on the existing Manager. Use this
// as a convenience when you only need one language; for multiple languages or
// custom config, use WithTreeSitter instead.
func WithTreeSitterLanguage(ext string, lang *tree_sitter.Language) Option {
	return func(s *Server) {
		if s.tsManager == nil {
			cfg := treesitter.Config{
				Languages: map[string]*tree_sitter.Language{ext: lang},
			}
			s.tsManager = treesitter.NewManager(cfg, s.docStore)
			s.diagEngine = treesitter.NewDiagnosticEngine(s.tsManager, s.docStore, s.logger)
		} else {
			s.tsManager.Registry().Register(ext, lang)
		}
	}
}

// WithCompletionTriggerCharacters configures characters that trigger
// completion when typed. These are advertised in InitializeResult and typically
// include "." and "(" for method/function invocation. Must match what the
// client supports (see ClientCapabilities).
func WithCompletionTriggerCharacters(chars ...string) Option {
	return func(s *Server) {
		s.completionTriggerChars = chars
	}
}

// WithSignatureHelpTriggerCharacters configures characters that trigger
// signature help when typed (e.g., "(" for function calls). Advertised in
// InitializeResult; use when the client supports dynamic registration.
func WithSignatureHelpTriggerCharacters(chars ...string) Option {
	return func(s *Server) {
		s.signatureHelpTriggerChars = chars
	}
}

// WithSemanticTokensLegend configures the legend for semantic token ranges.
// The legend maps token types and modifiers to numeric indices used in the
// encoded token stream. Required if you use OnSemanticTokens or OnSemanticTokensRange.
func WithSemanticTokensLegend(legend protocol.SemanticTokensLegend) Option {
	return func(s *Server) {
		s.semanticTokensLegend = &legend
	}
}

// WithExecuteCommands configures the list of workspace/executeCommand
// identifiers the server supports. These are advertised in InitializeResult;
// clients invoke them via workspace/executeCommand. Must register a handler
// via OnExecuteCommand (or HandleRequest) for each command.
func WithExecuteCommands(commands ...string) Option {
	return func(s *Server) {
		s.executeCommands = commands
	}
}

// WithTCP configures the server to listen on a TCP address. Use for remote or
// multi-process setups (e.g., ":9257"). The transport is created when Serve
// starts; addr must be a valid network address.
func WithTCP(addr string) ServeOption {
	return func(cfg *serveConfig) {
		cfg.transportFactory = func() (transport.Transport, error) {
			return transport.ListenTCP(addr)
		}
	}
}

// WithSocket configures the server to listen on a Unix domain socket at the
// given path. Use for local IPC; the socket file is created on disk.
func WithSocket(path string) ServeOption {
	return func(cfg *serveConfig) {
		cfg.transportFactory = func() (transport.Transport, error) {
			return transport.ListenSocket(path)
		}
	}
}

// WithPipe configures the server to listen on a named pipe. On Windows, uses
// a true named pipe; on Unix, uses a Unix domain socket. Use for editor/IDE
// integrations that connect via named pipes.
func WithPipe(name string) ServeOption {
	return func(cfg *serveConfig) {
		cfg.transportFactory = func() (transport.Transport, error) {
			return transport.ListenPipe(name)
		}
	}
}

// WithWebSocket configures the server to listen for WebSocket connections at
// the given address. Use for browser-based LSP clients or remote access over HTTP.
func WithWebSocket(addr string) ServeOption {
	return func(cfg *serveConfig) {
		cfg.transportFactory = func() (transport.Transport, error) {
			return transport.ListenWebSocket(addr)
		}
	}
}

// WithNodeIPC configures the server for Node.js IPC transport. Use when running
// as a VS Code extension where the extension host communicates via Node IPC
// rather than stdio.
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
