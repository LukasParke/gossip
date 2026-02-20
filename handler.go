package gossip

import (
	"encoding/json"

	"github.com/LukasParke/gossip/protocol"
)

// RawHandler processes a JSON-RPC request with raw params. Use HandleRequest
// to register these for custom or unsupported LSP methods.
type RawHandler func(ctx *Context, params json.RawMessage) (interface{}, error)

// RawNotificationHandler processes a JSON-RPC notification with raw params.
// Use HandleNotification to register these for custom or unsupported LSP notifications.
type RawNotificationHandler func(ctx *Context, params json.RawMessage)

// Handler function types for each LSP method.
// Request handlers return a result and an error.
// Notification handlers return only an error.

type InitializeHandler func(ctx *Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error)
type ShutdownHandler func(ctx *Context) error
type SetTraceHandler func(ctx *Context, params *protocol.SetTraceParams) error

// Text document sync
type DidOpenHandler func(ctx *Context, params *protocol.DidOpenTextDocumentParams) error
type DidChangeHandler func(ctx *Context, params *protocol.DidChangeTextDocumentParams) error
type DidCloseHandler func(ctx *Context, params *protocol.DidCloseTextDocumentParams) error
type DidSaveHandler func(ctx *Context, params *protocol.DidSaveTextDocumentParams) error

// Language features
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

// Additional language features
type DeclarationHandler func(ctx *Context, params *protocol.DeclarationParams) ([]protocol.Location, error)
type TypeDefinitionHandler func(ctx *Context, params *protocol.TypeDefinitionParams) ([]protocol.Location, error)
type ImplementationHandler func(ctx *Context, params *protocol.ImplementationParams) ([]protocol.Location, error)
type PrepareRenameHandler func(ctx *Context, params *protocol.PrepareRenameParams) (interface{}, error)
type RangeFormattingHandler func(ctx *Context, params *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error)
type DocumentLinkHandler func(ctx *Context, params *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error)
type SelectionRangeHandler func(ctx *Context, params *protocol.SelectionRangeParams) ([]protocol.SelectionRange, error)
type ExecuteCommandHandler func(ctx *Context, params *protocol.ExecuteCommandParams) (interface{}, error)

// Workspace notifications
type DidChangeConfigurationHandler func(ctx *Context, params *protocol.DidChangeConfigurationParams) error
type DidChangeWatchedFilesHandler func(ctx *Context, params *protocol.DidChangeWatchedFilesParams) error
type DidChangeWorkspaceFoldersHandler func(ctx *Context, params *protocol.DidChangeWorkspaceFoldersParams) error
