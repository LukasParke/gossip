// Package protocol contains LSP 3.18 types used by the gossip framework.
package protocol

// DocumentURI represents the URI of a document.
type DocumentURI string

// URI is a generic URI string.
type URI string

// Position in a text document expressed as zero-based line and character offset.
type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

// Range in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location inside a resource.
type Location struct {
	URI   DocumentURI `json:"uri"`
	Range Range       `json:"range"`
}

// TextDocumentIdentifier identifies a text document.
type TextDocumentIdentifier struct {
	URI DocumentURI `json:"uri"`
}

// VersionedTextDocumentIdentifier identifies a versioned text document.
type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier
	Version int32 `json:"version"`
}

// TextDocumentItem describes a text document with content.
type TextDocumentItem struct {
	URI        DocumentURI `json:"uri"`
	LanguageID string      `json:"languageId"`
	Version    int32       `json:"version"`
	Text       string      `json:"text"`
}

// TextDocumentPositionParams combines a document identifier and a position.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// TextDocumentContentChangeEvent describes a content change in a text document.
type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength uint32 `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

// MarkupKind describes the content type of a Hover result.
type MarkupKind string

const (
	PlainText MarkupKind = "plaintext"
	Markdown  MarkupKind = "markdown"
)

// MarkupContent represents a string value with a specific content kind.
type MarkupContent struct {
	Kind  MarkupKind `json:"kind"`
	Value string     `json:"value"`
}

// --- Lifecycle types ---

// InitializeParams is sent as the first request from client to server.
type InitializeParams struct {
	ProcessID             *int32               `json:"processId"`
	RootURI               *DocumentURI         `json:"rootUri,omitempty"`
	Capabilities          ClientCapabilities   `json:"capabilities"`
	InitializationOptions interface{}          `json:"initializationOptions,omitempty"`
	WorkspaceFolders      []WorkspaceFolder    `json:"workspaceFolders,omitempty"`
	Trace                 string               `json:"trace,omitempty"`
}

// ClientCapabilities defines capabilities provided by the client.
type ClientCapabilities struct {
	Workspace    *WorkspaceClientCapabilities    `json:"workspace,omitempty"`
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Window       *WindowClientCapabilities       `json:"window,omitempty"`
	General      *GeneralClientCapabilities      `json:"general,omitempty"`
}

type WorkspaceClientCapabilities struct {
	Configuration    bool `json:"configuration,omitempty"`
	WorkspaceFolders bool `json:"workspaceFolders,omitempty"`
	DidChangeConfiguration *struct {
		DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	} `json:"didChangeConfiguration,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Synchronization *TextDocumentSyncClientCapabilities `json:"synchronization,omitempty"`
	Completion      *CompletionClientCapabilities       `json:"completion,omitempty"`
	Hover           *HoverClientCapabilities            `json:"hover,omitempty"`
}

type TextDocumentSyncClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

type CompletionClientCapabilities struct{}
type HoverClientCapabilities struct{}

type WindowClientCapabilities struct {
	WorkDoneProgress bool `json:"workDoneProgress,omitempty"`
}

type GeneralClientCapabilities struct {
	PositionEncodings []string `json:"positionEncodings,omitempty"`
}

// WorkspaceFolder represents a workspace folder.
type WorkspaceFolder struct {
	URI  DocumentURI `json:"uri"`
	Name string      `json:"name"`
}

// InitializeResult is the response to the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

// ServerInfo is returned as part of the initialize result.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// ServerCapabilities defines what the server can do.
type ServerCapabilities struct {
	TextDocumentSync           *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	HoverProvider              interface{}              `json:"hoverProvider,omitempty"`
	CompletionProvider         *CompletionOptions       `json:"completionProvider,omitempty"`
	DefinitionProvider         interface{}              `json:"definitionProvider,omitempty"`
	ReferencesProvider         interface{}              `json:"referencesProvider,omitempty"`
	DocumentSymbolProvider     interface{}              `json:"documentSymbolProvider,omitempty"`
	WorkspaceSymbolProvider    interface{}              `json:"workspaceSymbolProvider,omitempty"`
	CodeActionProvider         interface{}              `json:"codeActionProvider,omitempty"`
	DocumentFormattingProvider interface{}              `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider interface{}          `json:"documentRangeFormattingProvider,omitempty"`
	RenameProvider             interface{}              `json:"renameProvider,omitempty"`
	SignatureHelpProvider      *SignatureHelpOptions    `json:"signatureHelpProvider,omitempty"`
	DiagnosticProvider         interface{}              `json:"diagnosticProvider,omitempty"`
	InlayHintProvider          interface{}              `json:"inlayHintProvider,omitempty"`
	SemanticTokensProvider     interface{}              `json:"semanticTokensProvider,omitempty"`
	DocumentHighlightProvider  interface{}              `json:"documentHighlightProvider,omitempty"`
	FoldingRangeProvider       interface{}              `json:"foldingRangeProvider,omitempty"`
	DeclarationProvider        interface{}              `json:"declarationProvider,omitempty"`
	TypeDefinitionProvider     interface{}              `json:"typeDefinitionProvider,omitempty"`
	ImplementationProvider     interface{}              `json:"implementationProvider,omitempty"`
	CodeLensProvider           *CodeLensOptions         `json:"codeLensProvider,omitempty"`
	DocumentLinkProvider       *DocumentLinkOptions     `json:"documentLinkProvider,omitempty"`
	SelectionRangeProvider     interface{}              `json:"selectionRangeProvider,omitempty"`
	ExecuteCommandProvider     *ExecuteCommandOptions   `json:"executeCommandProvider,omitempty"`
	ColorProvider              interface{}              `json:"colorProvider,omitempty"`
	Workspace                  *ServerWorkspaceCapabilities `json:"workspace,omitempty"`
}

// TextDocumentSyncKind defines how text documents are synced.
type TextDocumentSyncKind int

const (
	SyncNone        TextDocumentSyncKind = 0
	SyncFull        TextDocumentSyncKind = 1
	SyncIncremental TextDocumentSyncKind = 2
)

type TextDocumentSyncOptions struct {
	OpenClose bool                 `json:"openClose,omitempty"`
	Change    TextDocumentSyncKind `json:"change,omitempty"`
	Save      *SaveOptions         `json:"save,omitempty"`
}

type SaveOptions struct {
	IncludeText bool `json:"includeText,omitempty"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
}

type SignatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type CodeLensOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

type DocumentLinkOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

// InitializedParams is sent as a notification after successful initialize.
type InitializedParams struct{}

// --- Text document sync notifications ---

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         *string                `json:"text,omitempty"`
}

// --- Request params ---

type HoverParams struct {
	TextDocumentPositionParams
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type CompletionParams struct {
	TextDocumentPositionParams
	Context *CompletionContext `json:"context,omitempty"`
}

type CompletionContext struct {
	TriggerKind      CompletionTriggerKind `json:"triggerKind"`
	TriggerCharacter string                `json:"triggerCharacter,omitempty"`
}

type CompletionTriggerKind int

const (
	Invoked                         CompletionTriggerKind = 1
	TriggerCharacter                CompletionTriggerKind = 2
	TriggerForIncompleteCompletions CompletionTriggerKind = 3
)

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type CompletionItem struct {
	Label         string              `json:"label"`
	Kind          CompletionItemKind  `json:"kind,omitempty"`
	Detail        string              `json:"detail,omitempty"`
	Documentation interface{}         `json:"documentation,omitempty"`
	InsertText    string              `json:"insertText,omitempty"`
	TextEdit      *TextEdit           `json:"textEdit,omitempty"`
}

type CompletionItemKind int

const (
	CompletionKindText        CompletionItemKind = 1
	CompletionKindMethod      CompletionItemKind = 2
	CompletionKindFunction    CompletionItemKind = 3
	CompletionKindConstructor CompletionItemKind = 4
	CompletionKindField       CompletionItemKind = 5
	CompletionKindVariable    CompletionItemKind = 6
	CompletionKindClass       CompletionItemKind = 7
	CompletionKindInterface   CompletionItemKind = 8
	CompletionKindModule      CompletionItemKind = 9
	CompletionKindProperty    CompletionItemKind = 10
	CompletionKindKeyword     CompletionItemKind = 14
	CompletionKindSnippet     CompletionItemKind = 15
)

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type DefinitionParams struct {
	TextDocumentPositionParams
}

type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type CodeAction struct {
	Title       string      `json:"title"`
	Kind        string      `json:"kind,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
	Command     *Command    `json:"command,omitempty"`
}

type WorkspaceEdit struct {
	Changes map[DocumentURI][]TextEdit `json:"changes,omitempty"`
}

type Command struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// --- Diagnostics ---

type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity,omitempty"`
	Code     interface{}        `json:"code,omitempty"`
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
}

type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Version     *int32       `json:"version,omitempty"`
}

// --- Symbols ---

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type SymbolKind int

const (
	SymbolFile        SymbolKind = 1
	SymbolModule      SymbolKind = 2
	SymbolNamespace   SymbolKind = 3
	SymbolPackage     SymbolKind = 4
	SymbolClass       SymbolKind = 5
	SymbolMethod      SymbolKind = 6
	SymbolProperty    SymbolKind = 7
	SymbolField       SymbolKind = 8
	SymbolConstructor SymbolKind = 9
	SymbolFunction    SymbolKind = 12
	SymbolVariable    SymbolKind = 13
	SymbolConstant    SymbolKind = 14
	SymbolString      SymbolKind = 15
	SymbolStruct      SymbolKind = 23
)

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

type SymbolInformation struct {
	Name     string   `json:"name"`
	Kind     SymbolKind `json:"kind"`
	Location Location `json:"location"`
}

// --- Formatting ---

type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

type FormattingOptions struct {
	TabSize      uint32 `json:"tabSize"`
	InsertSpaces bool   `json:"insertSpaces"`
}

// --- Rename ---

type RenameParams struct {
	TextDocumentPositionParams
	NewName string `json:"newName"`
}

// --- Signature Help ---

type SignatureHelpParams struct {
	TextDocumentPositionParams
}

type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature *uint32               `json:"activeSignature,omitempty"`
	ActiveParameter *uint32               `json:"activeParameter,omitempty"`
}

type SignatureInformation struct {
	Label         string                 `json:"label"`
	Documentation interface{}            `json:"documentation,omitempty"`
	Parameters    []ParameterInformation `json:"parameters,omitempty"`
}

type ParameterInformation struct {
	Label         interface{} `json:"label"`
	Documentation interface{} `json:"documentation,omitempty"`
}

// --- Window messages ---

type MessageType int

const (
	Error   MessageType = 1
	Warning MessageType = 2
	Info    MessageType = 3
	Log     MessageType = 4
)

type LogMessageParams struct {
	Type    MessageType `json:"type"`
	Message string      `json:"message"`
}

type ShowMessageParams struct {
	Type    MessageType `json:"type"`
	Message string      `json:"message"`
}

// --- Configuration ---

type DidChangeConfigurationParams struct {
	Settings interface{} `json:"settings"`
}

type ConfigurationParams struct {
	Items []ConfigurationItem `json:"items"`
}

type ConfigurationItem struct {
	ScopeURI *DocumentURI `json:"scopeUri,omitempty"`
	Section  string       `json:"section,omitempty"`
}

// --- Folding Range ---

type FoldingRangeParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type FoldingRange struct {
	StartLine      uint32 `json:"startLine"`
	StartCharacter uint32 `json:"startCharacter,omitempty"`
	EndLine        uint32 `json:"endLine"`
	EndCharacter   uint32 `json:"endCharacter,omitempty"`
	Kind           string `json:"kind,omitempty"`
}

// --- Document Highlight ---

type DocumentHighlightParams struct {
	TextDocumentPositionParams
}

type DocumentHighlight struct {
	Range Range `json:"range"`
	Kind  int   `json:"kind,omitempty"`
}

// --- Inlay Hints ---

type InlayHintParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
}

type InlayHint struct {
	Position Position    `json:"position"`
	Label    interface{} `json:"label"`
	Kind     *int        `json:"kind,omitempty"`
}

// --- Semantic Tokens ---

type SemanticTokensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type SemanticTokens struct {
	ResultID string   `json:"resultId,omitempty"`
	Data     []uint32 `json:"data"`
}

// --- Code Lens ---

type CodeLensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type CodeLens struct {
	Range   Range    `json:"range"`
	Command *Command `json:"command,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// --- Workspace Symbols ---

type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// --- Set Trace ---

type SetTraceParams struct {
	Value string `json:"value"`
}

// --- Workspace Folders ---

type ServerWorkspaceCapabilities struct {
	WorkspaceFolders *WorkspaceFoldersServerCapabilities `json:"workspaceFolders,omitempty"`
	FileOperations   interface{}                         `json:"fileOperations,omitempty"`
}

type WorkspaceFoldersServerCapabilities struct {
	Supported           bool        `json:"supported,omitempty"`
	ChangeNotifications interface{} `json:"changeNotifications,omitempty"`
}

type DidChangeWorkspaceFoldersParams struct {
	Event WorkspaceFoldersChangeEvent `json:"event"`
}

type WorkspaceFoldersChangeEvent struct {
	Added   []WorkspaceFolder `json:"added"`
	Removed []WorkspaceFolder `json:"removed"`
}

// --- Execute Command ---

type ExecuteCommandOptions struct {
	Commands []string `json:"commands,omitempty"`
}

type ExecuteCommandParams struct {
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// --- Additional Language Feature Params ---

type DeclarationParams struct {
	TextDocumentPositionParams
}

type TypeDefinitionParams struct {
	TextDocumentPositionParams
}

type ImplementationParams struct {
	TextDocumentPositionParams
}

type PrepareRenameParams struct {
	TextDocumentPositionParams
}

type DocumentRangeFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Options      FormattingOptions      `json:"options"`
}

type DocumentLinkParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DocumentLink struct {
	Range   Range        `json:"range"`
	Target  *DocumentURI `json:"target,omitempty"`
	Tooltip string       `json:"tooltip,omitempty"`
	Data    interface{}  `json:"data,omitempty"`
}

type SelectionRangeParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Positions    []Position             `json:"positions"`
}

type SelectionRange struct {
	Range  Range          `json:"range"`
	Parent *SelectionRange `json:"parent,omitempty"`
}

// --- File Events ---

type FileChangeType int

const (
	FileCreated FileChangeType = 1
	FileChanged FileChangeType = 2
	FileDeleted FileChangeType = 3
)

type FileEvent struct {
	URI  DocumentURI    `json:"uri"`
	Type FileChangeType `json:"type"`
}

type DidChangeWatchedFilesParams struct {
	Changes []FileEvent `json:"changes"`
}

// --- Workspace Edit (server -> client requests) ---

type ApplyWorkspaceEditParams struct {
	Label string        `json:"label,omitempty"`
	Edit  WorkspaceEdit `json:"edit"`
}

type ApplyWorkspaceEditResponse struct {
	Applied       bool   `json:"applied"`
	FailureReason string `json:"failureReason,omitempty"`
}

// --- Show Message Request ---

type ShowMessageRequestParams struct {
	Type    MessageType         `json:"type"`
	Message string              `json:"message"`
	Actions []MessageActionItem `json:"actions,omitempty"`
}

type MessageActionItem struct {
	Title string `json:"title"`
}

// --- Dynamic Registration ---

type RegistrationParams struct {
	Registrations []Registration `json:"registrations"`
}

type Registration struct {
	ID              string      `json:"id"`
	Method          string      `json:"method"`
	RegisterOptions interface{} `json:"registerOptions,omitempty"`
}

type UnregistrationParams struct {
	// JSON key is intentionally misspelled to match the LSP specification.
	Unregistrations []Unregistration `json:"unregisterations"`
}

type Unregistration struct {
	ID     string `json:"id"`
	Method string `json:"method"`
}
