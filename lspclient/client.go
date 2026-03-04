package lspclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/LukasParke/gossip/jsonrpc"
	"github.com/LukasParke/gossip/protocol"
)

// DiagnosticHandler is called when the child LSP publishes diagnostics.
type DiagnosticHandler func(uri protocol.DocumentURI, diags []protocol.Diagnostic)

// ClientOptions configures a child LSP client.
type ClientOptions struct {
	// Command is the executable to run (e.g. "yaml-language-server").
	Command string
	// Args are the command-line arguments (e.g. ["--stdio"]).
	Args []string
	// RootURI is the workspace root URI sent in the initialize request.
	RootURI string
	// Settings is the configuration returned in workspace/configuration responses.
	// Each entry maps a config section name to its value.
	Settings map[string]any
	// OnDiagnostics is called when the child publishes diagnostics.
	OnDiagnostics DiagnosticHandler
	// Logger is optional; if nil, a no-op logger is used.
	Logger *slog.Logger
}

// Client manages a child LSP server subprocess. It handles the LSP
// initialization handshake, responds to workspace/configuration requests
// from the child, forwards document sync notifications, and collects
// publishDiagnostics notifications from the child.
type Client struct {
	opts      ClientOptions
	transport *ProcessTransport
	conn      *jsonrpc.Conn
	logger    *slog.Logger

	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	runDone  chan struct{}
	syncKind protocol.TextDocumentSyncKind
}

// NewClient creates a new child LSP client. Call Start to spawn the process.
func NewClient(opts ClientOptions) *Client {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	return &Client{
		opts:     opts,
		logger:   logger,
		syncKind: protocol.SyncFull,
	}
}

// Start spawns the child process, performs the LSP initialize/initialized
// handshake, and begins listening for notifications. It is safe to call
// document sync methods after Start returns.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("lspclient: already running")
	}

	t, err := NewProcessTransport(c.opts.Command, c.opts.Args...)
	if err != nil {
		return err
	}
	c.transport = t

	codec := jsonrpc.NewCodec(t, t)
	c.conn = jsonrpc.NewConn(codec, c.handleRequest, c.handleNotification)

	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.runDone = make(chan struct{})

	go func() {
		defer close(c.runDone)
		if err := c.conn.Run(runCtx); err != nil {
			c.logger.Debug("lspclient: conn.Run exited", "command", c.opts.Command, "error", err)
		}
	}()

	if err := c.initialize(ctx); err != nil {
		c.conn.Close()
		c.transport.Close()
		cancel()
		return fmt.Errorf("lspclient: initialize %s: %w", c.opts.Command, err)
	}

	c.running = true
	return nil
}

func (c *Client) initialize(ctx context.Context) error {
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pid := int32(os.Getpid())
	rootURI := protocol.DocumentURI(c.opts.RootURI)

	params := &protocol.InitializeParams{
		ProcessID: &pid,
		RootURI:   &rootURI,
		Capabilities: protocol.ClientCapabilities{
			Workspace: &protocol.WorkspaceClientCapabilities{
				Configuration: true,
			},
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Synchronization: &protocol.TextDocumentSyncClientCapabilities{
					DynamicRegistration: true,
				},
			},
		},
	}

	resp, err := c.conn.Call(initCtx, protocol.MethodInitialize, params)
	if err != nil {
		return fmt.Errorf("initialize call: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	var result protocol.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err == nil {
		if result.Capabilities.TextDocumentSync != nil {
			c.syncKind = result.Capabilities.TextDocumentSync.Change
		}
	}

	if err := c.conn.Notify(initCtx, protocol.MethodInitialized, &protocol.InitializedParams{}); err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}

	return nil
}

// handleRequest processes requests FROM the child LSP (e.g. workspace/configuration).
func (c *Client) handleRequest(_ context.Context, method string, params jsonrpc.RawMessage) (any, error) {
	switch method {
	case protocol.MethodWorkspaceConfiguration:
		return c.handleConfiguration(params)
	default:
		c.logger.Debug("lspclient: unhandled request from child", "method", method)
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "not supported"}
	}
}

func (c *Client) handleConfiguration(params jsonrpc.RawMessage) (any, error) {
	var req struct {
		Items []struct {
			Section string `json:"section"`
		} `json:"items"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("parse configuration request: %w", err)
	}

	results := make([]any, len(req.Items))
	for i, item := range req.Items {
		if val, ok := c.opts.Settings[item.Section]; ok {
			results[i] = val
		} else {
			results[i] = nil
		}
	}
	return results, nil
}

// handleNotification processes notifications FROM the child LSP.
func (c *Client) handleNotification(_ context.Context, method string, params jsonrpc.RawMessage) {
	switch method {
	case protocol.MethodPublishDiagnostics:
		var p protocol.PublishDiagnosticsParams
		if err := json.Unmarshal(params, &p); err != nil {
			c.logger.Warn("lspclient: invalid publishDiagnostics", "error", err)
			return
		}
		if c.opts.OnDiagnostics != nil {
			c.opts.OnDiagnostics(p.URI, p.Diagnostics)
		}
	default:
		c.logger.Debug("lspclient: unhandled notification from child", "method", method)
	}
}

// DidOpen forwards a textDocument/didOpen notification to the child.
func (c *Client) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	if !c.isRunning() {
		return nil
	}
	return c.conn.Notify(ctx, protocol.MethodDidOpen, params)
}

// DidChange forwards a textDocument/didChange notification to the child.
func (c *Client) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if !c.isRunning() {
		return nil
	}
	return c.conn.Notify(ctx, protocol.MethodDidChange, params)
}

// DidClose forwards a textDocument/didClose notification to the child.
func (c *Client) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	if !c.isRunning() {
		return nil
	}
	return c.conn.Notify(ctx, protocol.MethodDidClose, params)
}

// Stop performs a graceful shutdown of the child LSP, sending shutdown then
// exit. If the child doesn't respond within 5 seconds, it is killed.
func (c *Client) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}
	c.running = false

	shutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, _ = c.conn.Call(shutCtx, protocol.MethodShutdown, nil)
	_ = c.conn.Notify(shutCtx, protocol.MethodExit, nil)

	c.conn.Close()
	err := c.transport.Close()
	if c.cancel != nil {
		c.cancel()
	}

	<-c.runDone
	return err
}

// Running reports whether the child process is running.
func (c *Client) Running() bool { return c.isRunning() }

func (c *Client) isRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}
