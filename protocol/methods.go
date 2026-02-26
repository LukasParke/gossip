package protocol

// LSP method and notification names. Values match the Language Server Protocol
// 3.18 specification method strings.
const (
	// Lifecycle
	MethodInitialize  = "initialize"
	MethodInitialized = "initialized"
	MethodShutdown    = "shutdown"
	MethodExit        = "exit"
	MethodSetTrace    = "$/setTrace"

	// Text document sync
	MethodDidOpen   = "textDocument/didOpen"
	MethodDidChange = "textDocument/didChange"
	MethodDidClose  = "textDocument/didClose"
	MethodDidSave   = "textDocument/didSave"

	// Language features
	MethodHover             = "textDocument/hover"
	MethodCompletion        = "textDocument/completion"
	MethodDefinition        = "textDocument/definition"
	MethodDeclaration       = "textDocument/declaration"
	MethodTypeDefinition    = "textDocument/typeDefinition"
	MethodImplementation    = "textDocument/implementation"
	MethodReferences        = "textDocument/references"
	MethodDocumentSymbol    = "textDocument/documentSymbol"
	MethodCodeAction        = "textDocument/codeAction"
	MethodFormatting        = "textDocument/formatting"
	MethodRename            = "textDocument/rename"
	MethodSignatureHelp     = "textDocument/signatureHelp"
	MethodDocumentHighlight = "textDocument/documentHighlight"
	MethodFoldingRange      = "textDocument/foldingRange"
	MethodInlayHint         = "textDocument/inlayHint"
	MethodSemanticTokensFull = "textDocument/semanticTokens/full"
	MethodCodeLens          = "textDocument/codeLens"

	// Workspace
	MethodWorkspaceSymbol              = "workspace/symbol"
	MethodDidChangeConfiguration       = "workspace/didChangeConfiguration"
	MethodDidChangeWorkspaceFolders    = "workspace/didChangeWorkspaceFolders"
	MethodDidChangeWatchedFiles        = "workspace/didChangeWatchedFiles"
	MethodExecuteCommand               = "workspace/executeCommand"

	// Language features (additional)
	MethodPrepareRename           = "textDocument/prepareRename"
	MethodRangeFormatting         = "textDocument/rangeFormatting"
	MethodDocumentLink            = "textDocument/documentLink"
	MethodDocumentLinkResolve     = "documentLink/resolve"
	MethodSelectionRange          = "textDocument/selectionRange"
	MethodCompletionResolve       = "completionItem/resolve"
	MethodSemanticTokensRange     = "textDocument/semanticTokens/range"
	MethodDocumentDiagnostic      = "textDocument/diagnostic"
	MethodLinkedEditingRange      = "textDocument/linkedEditingRange"
	MethodPrepareCallHierarchy    = "textDocument/prepareCallHierarchy"
	MethodCallHierarchyIncoming   = "callHierarchy/incomingCalls"
	MethodCallHierarchyOutgoing   = "callHierarchy/outgoingCalls"
	MethodPrepareTypeHierarchy    = "textDocument/prepareTypeHierarchy"
	MethodTypeHierarchySupertypes = "typeHierarchy/supertypes"
	MethodTypeHierarchySubtypes   = "typeHierarchy/subtypes"

	// Special protocol notifications
	MethodCancelRequest = "$/cancelRequest"

	// Client notifications (server -> client)
	MethodPublishDiagnostics     = "textDocument/publishDiagnostics"
	MethodLogMessage             = "window/logMessage"
	MethodShowMessage            = "window/showMessage"
	MethodShowMessageRequest     = "window/showMessageRequest"
	MethodWorkspaceConfiguration = "workspace/configuration"

	// Client requests (server -> client)
	MethodApplyEdit            = "workspace/applyEdit"
	MethodRegisterCapability   = "client/registerCapability"
	MethodUnregisterCapability = "client/unregisterCapability"

	// Refresh requests (server -> client)
	MethodDiagnosticRefresh     = "workspace/diagnostic/refresh"
	MethodInlayHintRefresh      = "workspace/inlayHint/refresh"
	MethodSemanticTokensRefresh = "workspace/semanticTokens/refresh"

	// Work-done progress (server -> client)
	MethodWorkDoneProgressCreate = "window/workDoneProgress/create"
	MethodProgress               = "$/progress"
)
