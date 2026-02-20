package gossip

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/LukasParke/gossip/document"
	"github.com/LukasParke/gossip/jsonrpc"
	mw "github.com/LukasParke/gossip/middleware"
	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/treesitter"
)

// Server is the central type of the gossip framework. It registers handlers,
// manages lifecycle, and dispatches incoming LSP messages.
type Server struct {
	name    string
	version string
	logger  *slog.Logger

	// connection and client proxy (set during Serve)
	conn   *jsonrpc.Conn
	client *ClientProxy

	// built-in document store
	docStore *document.Store

	// tree-sitter manager (nil if not enabled)
	tsManager *treesitter.Manager

	// diagnostic engine (nil if tree-sitter not enabled)
	diagEngine *treesitter.DiagnosticEngine

	// config system (nil if not enabled)
	configHolder configHolder

	// middleware chain
	middlewares []mw.Middleware

	// handler registry
	mu                    sync.RWMutex
	handlers              map[string]interface{}
	rawHandlers           map[string]RawHandler
	rawNotifHandlers      map[string]RawNotificationHandler

	// workspace state (populated during initialize)
	rootURI          *protocol.DocumentURI
	workspaceFolders []protocol.WorkspaceFolder
	clientCaps       protocol.ClientCapabilities
	initOptions      json.RawMessage

	// options
	opts []Option

	// lifecycle state
	initialized bool
	shutdown    bool
}

// NewServer creates a new gossip LSP server with the given name and version.
func NewServer(name, version string, opts ...Option) *Server {
	s := &Server{
		name:             name,
		version:          version,
		logger:           slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		handlers:         make(map[string]interface{}),
		rawHandlers:      make(map[string]RawHandler),
		rawNotifHandlers: make(map[string]RawNotificationHandler),
		docStore:         document.NewStore(),
		opts:             opts,
	}
	return s
}

// --- Handler registration (functional pattern) ---

func (s *Server) OnHover(h HoverHandler)                           { s.register(protocol.MethodHover, h) }
func (s *Server) OnCompletion(h CompletionHandler)                 { s.register(protocol.MethodCompletion, h) }
func (s *Server) OnDefinition(h DefinitionHandler)                 { s.register(protocol.MethodDefinition, h) }
func (s *Server) OnReferences(h ReferencesHandler)                 { s.register(protocol.MethodReferences, h) }
func (s *Server) OnDocumentSymbol(h DocumentSymbolHandler)         { s.register(protocol.MethodDocumentSymbol, h) }
func (s *Server) OnCodeAction(h CodeActionHandler)                 { s.register(protocol.MethodCodeAction, h) }
func (s *Server) OnFormatting(h FormattingHandler)                 { s.register(protocol.MethodFormatting, h) }
func (s *Server) OnRename(h RenameHandler)                         { s.register(protocol.MethodRename, h) }
func (s *Server) OnSignatureHelp(h SignatureHelpHandler)           { s.register(protocol.MethodSignatureHelp, h) }
func (s *Server) OnDocumentHighlight(h DocumentHighlightHandler)   { s.register(protocol.MethodDocumentHighlight, h) }
func (s *Server) OnFoldingRange(h FoldingRangeHandler)             { s.register(protocol.MethodFoldingRange, h) }
func (s *Server) OnInlayHint(h InlayHintHandler)                  { s.register(protocol.MethodInlayHint, h) }
func (s *Server) OnSemanticTokens(h SemanticTokensHandler)         { s.register(protocol.MethodSemanticTokensFull, h) }
func (s *Server) OnCodeLens(h CodeLensHandler)                     { s.register(protocol.MethodCodeLens, h) }
func (s *Server) OnWorkspaceSymbol(h WorkspaceSymbolHandler)       { s.register(protocol.MethodWorkspaceSymbol, h) }
func (s *Server) OnDeclaration(h DeclarationHandler)               { s.register(protocol.MethodDeclaration, h) }
func (s *Server) OnTypeDefinition(h TypeDefinitionHandler)         { s.register(protocol.MethodTypeDefinition, h) }
func (s *Server) OnImplementation(h ImplementationHandler)         { s.register(protocol.MethodImplementation, h) }
func (s *Server) OnPrepareRename(h PrepareRenameHandler)           { s.register(protocol.MethodPrepareRename, h) }
func (s *Server) OnRangeFormatting(h RangeFormattingHandler)       { s.register(protocol.MethodRangeFormatting, h) }
func (s *Server) OnDocumentLink(h DocumentLinkHandler)             { s.register(protocol.MethodDocumentLink, h) }
func (s *Server) OnSelectionRange(h SelectionRangeHandler)         { s.register(protocol.MethodSelectionRange, h) }
func (s *Server) OnExecuteCommand(h ExecuteCommandHandler)         { s.register(protocol.MethodExecuteCommand, h) }

// Notification handlers
func (s *Server) OnDidOpen(h DidOpenHandler)     { s.register(protocol.MethodDidOpen, h) }
func (s *Server) OnDidChange(h DidChangeHandler) { s.register(protocol.MethodDidChange, h) }
func (s *Server) OnDidClose(h DidCloseHandler)   { s.register(protocol.MethodDidClose, h) }
func (s *Server) OnDidSave(h DidSaveHandler)     { s.register(protocol.MethodDidSave, h) }
func (s *Server) OnDidChangeConfiguration(h DidChangeConfigurationHandler) {
	s.register(protocol.MethodDidChangeConfiguration, h)
}
func (s *Server) OnDidChangeWatchedFiles(h DidChangeWatchedFilesHandler) {
	s.register(protocol.MethodDidChangeWatchedFiles, h)
}
func (s *Server) OnDidChangeWorkspaceFolders(h DidChangeWorkspaceFoldersHandler) {
	s.register(protocol.MethodDidChangeWorkspaceFolders, h)
}

// Check registers a declarative, pattern-based diagnostic rule. When
// tree-sitter is enabled, the pattern is run incrementally on each edit (scoped
// to only the changed ranges) and matching diagnostics are automatically cached,
// merged, and published. If tree-sitter is not enabled, the check is silently
// ignored.
func (s *Server) Check(name string, c treesitter.Check) {
	if s.diagEngine != nil {
		s.diagEngine.RegisterCheck(name, c)
	}
}

// Analyze registers an imperative diagnostic analyzer. See treesitter.Analyzer
// for the full API. Like Check, this is a no-op when tree-sitter is not enabled.
func (s *Server) Analyze(name string, a treesitter.Analyzer) {
	if s.diagEngine != nil {
		s.diagEngine.RegisterAnalyzer(name, a)
	}
}

// --- Accessor methods ("break glass" escape hatches) ---

// TreeSitter returns the tree-sitter Manager, or nil if tree-sitter is not enabled.
func (s *Server) TreeSitter() *treesitter.Manager { return s.tsManager }

// DiagnosticEngine returns the diagnostic engine, or nil if tree-sitter is not enabled.
func (s *Server) DiagnosticEngine() *treesitter.DiagnosticEngine { return s.diagEngine }

// Documents returns the document store.
func (s *Server) Documents() *document.Store { return s.docStore }

// Logger returns the server's logger.
func (s *Server) Logger() *slog.Logger { return s.logger }

// Conn returns the JSON-RPC connection, or nil before Serve() is called.
func (s *Server) Conn() *jsonrpc.Conn { return s.conn }

func (s *Server) register(method string, handler interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
}

func (s *Server) getHandler(method string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.handlers[method]
	return h, ok
}

// dispatch is the main JSON-RPC handler callback. It routes incoming messages
// to the appropriate registered handler.
func (s *Server) dispatch(ctx context.Context, method string, params jsonrpc.RawMessage) (interface{}, error) {
	gctx := newContext(ctx, s)

	switch method {
	case protocol.MethodInitialize:
		return s.handleInitialize(gctx, params)
	case protocol.MethodShutdown:
		return s.handleShutdown(gctx)
	}

	if !s.initialized {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeServerNotInitialized, Message: "server not initialized"}
	}

	return s.dispatchToHandler(gctx, method, params)
}

// dispatchNotification handles JSON-RPC notifications.
func (s *Server) dispatchNotification(ctx context.Context, method string, params jsonrpc.RawMessage) {
	gctx := newContext(ctx, s)

	switch method {
	case protocol.MethodInitialized:
		s.logger.Info("client initialized")
		return
	case protocol.MethodExit:
		s.logger.Info("received exit notification")
		if s.conn != nil {
			s.conn.Close()
		}
		if s.shutdown {
			os.Exit(0)
		}
		os.Exit(1)
	case protocol.MethodSetTrace:
		return
	}

	if !s.initialized {
		return
	}

	s.dispatchNotificationToHandler(gctx, method, params)
}

func (s *Server) handleInitialize(ctx *Context, params jsonrpc.RawMessage) (interface{}, error) {
	var p protocol.InitializeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
	}

	s.mu.Lock()
	s.rootURI = p.RootURI
	s.workspaceFolders = p.WorkspaceFolders
	s.clientCaps = p.Capabilities
	if p.InitializationOptions != nil {
		if raw, err := json.Marshal(p.InitializationOptions); err == nil {
			s.initOptions = raw
		}
	}

	if len(s.workspaceFolders) == 0 && s.rootURI != nil {
		s.workspaceFolders = []protocol.WorkspaceFolder{
			{URI: *s.rootURI, Name: uriBasename(string(*s.rootURI))},
		}
	}
	s.mu.Unlock()

	caps := s.buildCapabilities()
	s.initialized = true

	if s.configHolder != nil {
		s.startConfigWatchers()
	}

	s.logger.Info("server initialized",
		"name", s.name,
		"version", s.version,
		"workspaceFolders", len(s.workspaceFolders),
	)

	return &protocol.InitializeResult{
		Capabilities: caps,
		ServerInfo: &protocol.ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
	}, nil
}

func (s *Server) startConfigWatchers() {
	s.mu.RLock()
	folders := s.workspaceFolders
	s.mu.RUnlock()

	for _, folder := range folders {
		rootDir := uriToPath(string(folder.URI))
		if rootDir == "" {
			rootDir = "."
		}
		if err := s.configHolder.startWatcher(s.logger, rootDir); err != nil {
			s.logger.Warn("config watcher failed to start", "folder", folder.URI, "error", err)
		}
	}

	if len(folders) == 0 {
		if err := s.configHolder.startWatcher(s.logger, "."); err != nil {
			s.logger.Warn("config watcher failed to start", "error", err)
		}
	}
}

func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

func uriBasename(uri string) string {
	s := strings.TrimRight(uri, "/")
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

func (s *Server) handleShutdown(_ *Context) (interface{}, error) {
	s.shutdown = true
	s.logger.Info("server shutting down")
	return nil, nil
}

func (s *Server) dispatchToHandler(ctx *Context, method string, params jsonrpc.RawMessage) (interface{}, error) {
	h, ok := s.getHandler(method)
	if ok {
		return callHandler(ctx, h, method, params)
	}

	s.mu.RLock()
	rh, rok := s.rawHandlers[method]
	s.mu.RUnlock()
	if rok {
		return rh(ctx, params)
	}

	return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: fmt.Sprintf("method not found: %s", method)}
}

func (s *Server) dispatchNotificationToHandler(ctx *Context, method string, params jsonrpc.RawMessage) {
	switch method {
	case protocol.MethodDidOpen:
		var p protocol.DidOpenTextDocumentParams
		if err := json.Unmarshal(params, &p); err == nil {
			s.docStore.Open(&p)
		}
	case protocol.MethodDidChange:
		var p protocol.DidChangeTextDocumentParams
		if err := json.Unmarshal(params, &p); err == nil {
			s.docStore.Change(&p)
		}
	case protocol.MethodDidClose:
		var p protocol.DidCloseTextDocumentParams
		if err := json.Unmarshal(params, &p); err == nil {
			s.docStore.Close(&p)
			if s.diagEngine != nil {
				s.diagEngine.ClearCache(p.TextDocument.URI)
			}
		}
	case protocol.MethodDidChangeWorkspaceFolders:
		var p protocol.DidChangeWorkspaceFoldersParams
		if err := json.Unmarshal(params, &p); err == nil {
			s.handleWorkspaceFolderChange(p.Event)
		}
	}

	h, ok := s.getHandler(method)
	if ok {
		_, _ = callHandler(ctx, h, method, params)
		return
	}

	s.mu.RLock()
	rh, rok := s.rawNotifHandlers[method]
	s.mu.RUnlock()
	if rok {
		rh(ctx, params)
	}
}

func (s *Server) handleWorkspaceFolderChange(event protocol.WorkspaceFoldersChangeEvent) {
	s.mu.Lock()
	for _, removed := range event.Removed {
		for i, f := range s.workspaceFolders {
			if f.URI == removed.URI {
				s.workspaceFolders = append(s.workspaceFolders[:i], s.workspaceFolders[i+1:]...)
				break
			}
		}
	}
	for _, added := range event.Added {
		s.workspaceFolders = append(s.workspaceFolders, added)
	}
	s.mu.Unlock()

	s.logger.Info("workspace folders changed",
		"added", len(event.Added),
		"removed", len(event.Removed),
	)
}

// HandleRequest registers a raw handler for a custom or unhandled LSP method.
func (s *Server) HandleRequest(method string, h RawHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawHandlers[method] = h
}

// HandleNotification registers a raw handler for a custom or unhandled LSP notification.
func (s *Server) HandleNotification(method string, h RawNotificationHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawNotifHandlers[method] = h
}

// FolderFor returns the workspace folder that contains the given document URI,
// using longest-prefix matching. Returns nil if no folder matches.
func (s *Server) FolderFor(uri protocol.DocumentURI) *protocol.WorkspaceFolder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	uriStr := string(uri)
	var best *protocol.WorkspaceFolder
	bestLen := 0
	for i := range s.workspaceFolders {
		prefix := string(s.workspaceFolders[i].URI)
		if strings.HasPrefix(uriStr, prefix) && len(prefix) > bestLen {
			best = &s.workspaceFolders[i]
			bestLen = len(prefix)
		}
	}
	return best
}

// callHandler unmarshals params and calls the appropriate handler based on its type.
func callHandler(ctx *Context, handler interface{}, method string, params jsonrpc.RawMessage) (interface{}, error) {
	switch method {
	case protocol.MethodHover:
		var p protocol.HoverParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(HoverHandler)(ctx, &p)

	case protocol.MethodCompletion:
		var p protocol.CompletionParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(CompletionHandler)(ctx, &p)

	case protocol.MethodDefinition:
		var p protocol.DefinitionParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(DefinitionHandler)(ctx, &p)

	case protocol.MethodReferences:
		var p protocol.ReferenceParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(ReferencesHandler)(ctx, &p)

	case protocol.MethodDocumentSymbol:
		var p protocol.DocumentSymbolParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(DocumentSymbolHandler)(ctx, &p)

	case protocol.MethodCodeAction:
		var p protocol.CodeActionParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(CodeActionHandler)(ctx, &p)

	case protocol.MethodFormatting:
		var p protocol.DocumentFormattingParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(FormattingHandler)(ctx, &p)

	case protocol.MethodRename:
		var p protocol.RenameParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(RenameHandler)(ctx, &p)

	case protocol.MethodSignatureHelp:
		var p protocol.SignatureHelpParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(SignatureHelpHandler)(ctx, &p)

	case protocol.MethodDocumentHighlight:
		var p protocol.DocumentHighlightParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(DocumentHighlightHandler)(ctx, &p)

	case protocol.MethodFoldingRange:
		var p protocol.FoldingRangeParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(FoldingRangeHandler)(ctx, &p)

	case protocol.MethodInlayHint:
		var p protocol.InlayHintParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(InlayHintHandler)(ctx, &p)

	case protocol.MethodSemanticTokensFull:
		var p protocol.SemanticTokensParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(SemanticTokensHandler)(ctx, &p)

	case protocol.MethodCodeLens:
		var p protocol.CodeLensParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(CodeLensHandler)(ctx, &p)

	case protocol.MethodWorkspaceSymbol:
		var p protocol.WorkspaceSymbolParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(WorkspaceSymbolHandler)(ctx, &p)

	case protocol.MethodDeclaration:
		var p protocol.DeclarationParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(DeclarationHandler)(ctx, &p)

	case protocol.MethodTypeDefinition:
		var p protocol.TypeDefinitionParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(TypeDefinitionHandler)(ctx, &p)

	case protocol.MethodImplementation:
		var p protocol.ImplementationParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(ImplementationHandler)(ctx, &p)

	case protocol.MethodPrepareRename:
		var p protocol.PrepareRenameParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(PrepareRenameHandler)(ctx, &p)

	case protocol.MethodRangeFormatting:
		var p protocol.DocumentRangeFormattingParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(RangeFormattingHandler)(ctx, &p)

	case protocol.MethodDocumentLink:
		var p protocol.DocumentLinkParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(DocumentLinkHandler)(ctx, &p)

	case protocol.MethodSelectionRange:
		var p protocol.SelectionRangeParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(SelectionRangeHandler)(ctx, &p)

	case protocol.MethodExecuteCommand:
		var p protocol.ExecuteCommandParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return handler.(ExecuteCommandHandler)(ctx, &p)

	// Notification handlers
	case protocol.MethodDidOpen:
		var p protocol.DidOpenTextDocumentParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return nil, handler.(DidOpenHandler)(ctx, &p)

	case protocol.MethodDidChange:
		var p protocol.DidChangeTextDocumentParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return nil, handler.(DidChangeHandler)(ctx, &p)

	case protocol.MethodDidClose:
		var p protocol.DidCloseTextDocumentParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return nil, handler.(DidCloseHandler)(ctx, &p)

	case protocol.MethodDidSave:
		var p protocol.DidSaveTextDocumentParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return nil, handler.(DidSaveHandler)(ctx, &p)

	case protocol.MethodDidChangeConfiguration:
		var p protocol.DidChangeConfigurationParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return nil, handler.(DidChangeConfigurationHandler)(ctx, &p)

	case protocol.MethodDidChangeWatchedFiles:
		var p protocol.DidChangeWatchedFilesParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return nil, handler.(DidChangeWatchedFilesHandler)(ctx, &p)

	case protocol.MethodDidChangeWorkspaceFolders:
		var p protocol.DidChangeWorkspaceFoldersParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		return nil, handler.(DidChangeWorkspaceFoldersHandler)(ctx, &p)

	default:
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: fmt.Sprintf("no handler for method: %s", method)}
	}
}
