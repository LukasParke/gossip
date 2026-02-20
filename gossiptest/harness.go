// Package gossiptest provides testing utilities for gossip LSP servers.
// It includes an in-memory client that communicates with a server without
// network I/O, plus assertion helpers for common LSP patterns.
package gossiptest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gossip-lsp/gossip"
	"github.com/gossip-lsp/gossip/jsonrpc"
	"github.com/gossip-lsp/gossip/protocol"
	"github.com/gossip-lsp/gossip/transport"
)

// Client is a test LSP client that communicates with a server over an
// in-memory transport. It provides typed helper methods for common LSP requests.
type Client struct {
	t    testing.TB
	conn *jsonrpc.Conn
	stop func()

	mu            sync.Mutex
	notifications []notification
}

type notification struct {
	Method string
	Params json.RawMessage
}

// NewClient creates a test client connected to the given server.
// The server runs in a background goroutine and is automatically stopped
// when the test completes.
func NewClient(t testing.TB, s *gossip.Server) *Client {
	clientTransport, serverTransport := transport.MemoryPipe()

	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		t:    t,
		stop: cancel,
	}

	// Start server in background
	go func() {
		err := gossip.Serve(s, gossip.WithTransport(serverTransport))
		if err != nil && ctx.Err() == nil {
			t.Logf("server error: %v", err)
		}
	}()

	codec := jsonrpc.NewCodec(clientTransport, clientTransport)
	c.conn = jsonrpc.NewConn(codec, func(ctx context.Context, method string, params jsonrpc.RawMessage) (interface{}, error) {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "client does not handle requests"}
	}, func(ctx context.Context, method string, params jsonrpc.RawMessage) {
		c.mu.Lock()
		c.notifications = append(c.notifications, notification{Method: method, Params: params})
		c.mu.Unlock()
	})

	go func() {
		c.conn.Run(ctx)
	}()

	t.Cleanup(func() {
		cancel()
		c.conn.Close()
		clientTransport.Close()
	})

	// Auto-initialize
	c.Initialize()

	return c
}

// Initialize sends the initialize request and initialized notification.
func (c *Client) Initialize() *protocol.InitializeResult {
	c.t.Helper()
	params := &protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{},
	}
	var result protocol.InitializeResult
	c.call(protocol.MethodInitialize, params, &result)
	c.notify(protocol.MethodInitialized, &protocol.InitializedParams{})
	return &result
}

// Open sends a textDocument/didOpen notification.
func (c *Client) Open(uri string, text string) {
	c.t.Helper()
	c.notify(protocol.MethodDidOpen, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentURI(uri),
			LanguageID: "plaintext",
			Version:    1,
			Text:       text,
		},
	})
	// Give the server a moment to process
	time.Sleep(10 * time.Millisecond)
}

// OpenWithLanguage sends a textDocument/didOpen with a specific language ID.
func (c *Client) OpenWithLanguage(uri, languageID, text string) {
	c.t.Helper()
	c.notify(protocol.MethodDidOpen, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentURI(uri),
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	})
	time.Sleep(10 * time.Millisecond)
}

// Change sends a textDocument/didChange notification with full content replacement.
func (c *Client) Change(uri string, version int32, text string) {
	c.t.Helper()
	c.notify(protocol.MethodDidChange, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
			Version:                version,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: text}},
	})
	time.Sleep(10 * time.Millisecond)
}

// Close sends a textDocument/didClose notification.
func (c *Client) Close(uri string) {
	c.t.Helper()
	c.notify(protocol.MethodDidClose, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
	})
}

// Hover sends a textDocument/hover request.
func (c *Client) Hover(uri string, pos protocol.Position) (*protocol.Hover, error) {
	c.t.Helper()
	var result protocol.Hover
	err := c.callErr(protocol.MethodHover, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
			Position:     pos,
		},
	}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Completion sends a textDocument/completion request.
func (c *Client) Completion(uri string, pos protocol.Position) (*protocol.CompletionList, error) {
	c.t.Helper()
	var result protocol.CompletionList
	err := c.callErr(protocol.MethodCompletion, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
			Position:     pos,
		},
	}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Definition sends a textDocument/definition request.
func (c *Client) Definition(uri string, pos protocol.Position) ([]protocol.Location, error) {
	c.t.Helper()
	var result []protocol.Location
	err := c.callErr(protocol.MethodDefinition, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
			Position:     pos,
		},
	}, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ChangeIncremental sends a textDocument/didChange notification with an
// incremental edit (range-based replacement) rather than full content.
func (c *Client) ChangeIncremental(uri string, version int32, rng protocol.Range, text string) {
	c.t.Helper()
	c.notify(protocol.MethodDidChange, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
			Version:                version,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Range: &rng, Text: text}},
	})
	time.Sleep(10 * time.Millisecond)
}

// Diagnostics returns all published diagnostics notifications received so far.
func (c *Client) Diagnostics() []protocol.PublishDiagnosticsParams {
	c.t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	var result []protocol.PublishDiagnosticsParams
	for _, n := range c.notifications {
		if n.Method == protocol.MethodPublishDiagnostics {
			var p protocol.PublishDiagnosticsParams
			if json.Unmarshal(n.Params, &p) == nil {
				result = append(result, p)
			}
		}
	}
	return result
}

// WaitForDiagnostics polls until at least one PublishDiagnostics notification
// has been received for the given URI, or until the timeout expires. It returns
// the latest diagnostics for that URI.
func (c *Client) WaitForDiagnostics(uri string, timeout time.Duration) []protocol.Diagnostic {
	c.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		for i := len(c.notifications) - 1; i >= 0; i-- {
			n := c.notifications[i]
			if n.Method == protocol.MethodPublishDiagnostics {
				var p protocol.PublishDiagnosticsParams
				if json.Unmarshal(n.Params, &p) == nil && string(p.URI) == uri {
					c.mu.Unlock()
					return p.Diagnostics
				}
			}
		}
		c.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	c.t.Fatalf("timed out waiting for diagnostics on %s", uri)
	return nil
}

// LatestDiagnostics returns the most recent PublishDiagnostics for the given
// URI, or nil if none have been received.
func (c *Client) LatestDiagnostics(uri string) []protocol.Diagnostic {
	c.t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.notifications) - 1; i >= 0; i-- {
		n := c.notifications[i]
		if n.Method == protocol.MethodPublishDiagnostics {
			var p protocol.PublishDiagnosticsParams
			if json.Unmarshal(n.Params, &p) == nil && string(p.URI) == uri {
				return p.Diagnostics
			}
		}
	}
	return nil
}

// Shutdown sends the shutdown request.
func (c *Client) Shutdown() {
	c.t.Helper()
	c.call(protocol.MethodShutdown, nil, nil)
}

func (c *Client) call(method string, params, result interface{}) {
	c.t.Helper()
	if err := c.callErr(method, params, result); err != nil {
		c.t.Fatalf("call %s failed: %v", method, err)
	}
}

func (c *Client) callErr(method string, params, result interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.conn.Call(ctx, method, params)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return resp.Error
	}
	if result != nil && resp.Result != nil {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("unmarshalling result: %w", err)
		}
	}
	return nil
}

func (c *Client) notify(method string, params interface{}) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.conn.Notify(ctx, method, params); err != nil {
		c.t.Fatalf("notify %s failed: %v", method, err)
	}
}
