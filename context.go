package gossip

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/LukasParke/gossip/document"
	"github.com/LukasParke/gossip/protocol"
)

// Context wraps context.Context with convenient accessors for LSP services.
type Context struct {
	context.Context

	Client    *ClientProxy
	Documents *document.Store
	server    *Server
}

func newContext(ctx context.Context, s *Server) *Context {
	return &Context{
		Context:   ctx,
		Client:    s.client,
		Documents: s.docStore,
		server:    s,
	}
}

// ServerInfo returns the server's name and version.
func (c *Context) ServerInfo() protocol.ServerInfo {
	return protocol.ServerInfo{
		Name:    c.server.name,
		Version: c.server.version,
	}
}

// Server returns the underlying Server, providing full access to internals.
func (c *Context) Server() *Server {
	return c.server
}

// Logger returns the server's logger.
func (c *Context) Logger() *slog.Logger {
	return c.server.logger
}

// WorkspaceRoot returns the primary workspace root URI. This is the first
// workspace folder, or the rootURI from InitializeParams if no folders were sent.
func (c *Context) WorkspaceRoot() protocol.DocumentURI {
	c.server.mu.RLock()
	defer c.server.mu.RUnlock()
	if len(c.server.workspaceFolders) > 0 {
		return c.server.workspaceFolders[0].URI
	}
	if c.server.rootURI != nil {
		return *c.server.rootURI
	}
	return ""
}

// WorkspaceFolders returns all current workspace folders. The returned slice
// reflects dynamic adds/removes via workspace/didChangeWorkspaceFolders.
func (c *Context) WorkspaceFolders() []protocol.WorkspaceFolder {
	c.server.mu.RLock()
	defer c.server.mu.RUnlock()
	out := make([]protocol.WorkspaceFolder, len(c.server.workspaceFolders))
	copy(out, c.server.workspaceFolders)
	return out
}

// FolderFor returns the workspace folder that contains the given document URI,
// using longest-prefix matching. Returns nil if no folder matches.
func (c *Context) FolderFor(uri protocol.DocumentURI) *protocol.WorkspaceFolder {
	return c.server.FolderFor(uri)
}

// ClientCapabilities returns the capabilities sent by the client during initialization.
func (c *Context) ClientCapabilities() protocol.ClientCapabilities {
	c.server.mu.RLock()
	defer c.server.mu.RUnlock()
	return c.server.clientCaps
}

// InitOptions returns the raw initializationOptions sent by the client.
func (c *Context) InitOptions() json.RawMessage {
	c.server.mu.RLock()
	defer c.server.mu.RUnlock()
	return c.server.initOptions
}
