// Package gossip provides an LSP server framework with handler-based
// registration and optional tree-sitter integration.
package gossip

import (
	"encoding/json"

	"github.com/LukasParke/gossip/protocol"
)

// Handler type conventions:
//   - Request handlers (e.g., HoverHandler, CompletionHandler) take *Context
//     and typed params, return (result, error). Register via Server.On* methods.
//   - Notification handlers (e.g., DidOpenHandler) have the same shape but
//     return only error; the framework discards the result.
//   - RawHandler and RawNotificationHandler accept json.RawMessage for custom
//     or untyped methods; register via HandleRequest and HandleNotification.

// RawHandler processes a JSON-RPC request with raw params. Use HandleRequest
// to register these for custom or unsupported LSP methods.
type RawHandler func(ctx *Context, params json.RawMessage) (interface{}, error)

// RawNotificationHandler processes a JSON-RPC notification with raw params.
// Use HandleNotification to register these for custom or unsupported LSP notifications.
type RawNotificationHandler func(ctx *Context, params json.RawMessage)

// InitializeHandler handles the initialize request; the framework processes it
// internally, so registration is optional and typically unused.
type InitializeHandler func(ctx *Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error)

// ShutdownHandler handles the shutdown request; the framework marks the server
// as shutting down. Register to perform cleanup before the client sends exit.
type ShutdownHandler func(ctx *Context) error

// SetTraceHandler handles trace notification updates; optional for debugging.
type SetTraceHandler func(ctx *Context, params *protocol.SetTraceParams) error

// Text document sync handlers. These are invoked when documents are opened,
// changed, closed, or saved. The framework also updates the document store.
type DidOpenHandler func(ctx *Context, params *protocol.DidOpenTextDocumentParams) error
type DidChangeHandler func(ctx *Context, params *protocol.DidChangeTextDocumentParams) error
type DidCloseHandler func(ctx *Context, params *protocol.DidCloseTextDocumentParams) error
type DidSaveHandler func(ctx *Context, params *protocol.DidSaveTextDocumentParams) error

// Language feature request handlers. Each maps to a textDocument or workspace
// LSP method. Return nil for optional features the server does not support.
type HoverHandler func(ctx *Context, params *protocol.HoverParams) (*protocol.Hover, error)
type CompletionHandler func(ctx *Context, params *protocol.CompletionParams) (*protocol.CompletionList, error)
type DefinitionHandler func(ctx *Context, params *protocol.DefinitionParams) ([]protocol.Location, error)
type ReferencesHandler func(ctx *Context, params *protocol.ReferenceParams) ([]protocol.Location, error)
type DocumentSymbolHandler func(ctx *Context, params *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error)
type CodeActionHandler func(ctx *Context, params *protocol.CodeActionParams) ([]protocol.CodeAction, error)
type FormattingHandler func(ctx *Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error)
type RenameHandler func(ctx *Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error)
type SignatureHelpHandler func(ctx *Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error)
type DocumentHighlightHandler func(ctx *Context, params *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error)
type FoldingRangeHandler func(ctx *Context, params *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error)
type InlayHintHandler func(ctx *Context, params *protocol.InlayHintParams) ([]protocol.InlayHint, error)
type SemanticTokensHandler func(ctx *Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error)
type CodeLensHandler func(ctx *Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error)
type WorkspaceSymbolHandler func(ctx *Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error)

// Additional language feature handlers.
type DeclarationHandler func(ctx *Context, params *protocol.DeclarationParams) ([]protocol.Location, error)
type TypeDefinitionHandler func(ctx *Context, params *protocol.TypeDefinitionParams) ([]protocol.Location, error)
type ImplementationHandler func(ctx *Context, params *protocol.ImplementationParams) ([]protocol.Location, error)
type PrepareRenameHandler func(ctx *Context, params *protocol.PrepareRenameParams) (*protocol.PrepareRenameResult, error)
type RangeFormattingHandler func(ctx *Context, params *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error)
type DocumentLinkHandler func(ctx *Context, params *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error)
type SelectionRangeHandler func(ctx *Context, params *protocol.SelectionRangeParams) ([]protocol.SelectionRange, error)
type ExecuteCommandHandler func(ctx *Context, params *protocol.ExecuteCommandParams) (interface{}, error)

// SemanticTokensRangeHandler handles semantic tokens for a specific range (LSP 3.18).
type SemanticTokensRangeHandler func(ctx *Context, params *protocol.SemanticTokensRangeParams) (*protocol.SemanticTokens, error)

// CompletionResolveHandler resolves additional properties for a completion item.
type CompletionResolveHandler func(ctx *Context, params *protocol.CompletionItem) (*protocol.CompletionItem, error)

// DocumentLinkResolveHandler resolves the target URL for a document link.
type DocumentLinkResolveHandler func(ctx *Context, params *protocol.DocumentLink) (*protocol.DocumentLink, error)

// DocumentDiagnosticHandler provides pull-based diagnostics for a document.
type DocumentDiagnosticHandler func(ctx *Context, params *protocol.DocumentDiagnosticParams) (*protocol.DocumentDiagnosticReport, error)

// LinkedEditingRangeHandler returns ranges that should be edited together.
type LinkedEditingRangeHandler func(ctx *Context, params *protocol.LinkedEditingRangeParams) (*protocol.LinkedEditingRanges, error)

// Call hierarchy handlers.
type PrepareCallHierarchyHandler func(ctx *Context, params *protocol.CallHierarchyPrepareParams) ([]protocol.CallHierarchyItem, error)
type CallHierarchyIncomingHandler func(ctx *Context, params *protocol.CallHierarchyIncomingCallsParams) ([]protocol.CallHierarchyIncomingCall, error)
type CallHierarchyOutgoingHandler func(ctx *Context, params *protocol.CallHierarchyOutgoingCallsParams) ([]protocol.CallHierarchyOutgoingCall, error)

// Type hierarchy handlers.
type PrepareTypeHierarchyHandler func(ctx *Context, params *protocol.TypeHierarchyPrepareParams) ([]protocol.TypeHierarchyItem, error)
type TypeHierarchySupertypesHandler func(ctx *Context, params *protocol.TypeHierarchySupertypesParams) ([]protocol.TypeHierarchyItem, error)
type TypeHierarchySubtypesHandler func(ctx *Context, params *protocol.TypeHierarchySubtypesParams) ([]protocol.TypeHierarchyItem, error)

// Workspace notification handlers.
type DidChangeConfigurationHandler func(ctx *Context, params *protocol.DidChangeConfigurationParams) error
type DidChangeWatchedFilesHandler func(ctx *Context, params *protocol.DidChangeWatchedFilesParams) error
type DidChangeWorkspaceFoldersHandler func(ctx *Context, params *protocol.DidChangeWorkspaceFoldersParams) error
