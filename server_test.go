package gossip

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/LukasParke/gossip/jsonrpc"
	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/treesitter"
)

func TestNewServer(t *testing.T) {
	s := NewServer("test", "1.0")
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	store := s.Documents()
	if store == nil {
		t.Fatal("Documents() returned nil")
	}
}

func TestServerHandlerRegistration(t *testing.T) {
	s := NewServer("test", "1.0")
	s.OnHover(func(ctx *Context, p *protocol.HoverParams) (*protocol.Hover, error) { return nil, nil })
	s.OnCompletion(func(ctx *Context, p *protocol.CompletionParams) (*protocol.CompletionList, error) { return nil, nil })
	s.OnDefinition(func(ctx *Context, p *protocol.DefinitionParams) ([]protocol.Location, error) { return nil, nil })

	caps := s.buildCapabilities()

	if caps.HoverProvider != true {
		t.Error("expected HoverProvider true, got false")
	}
	if caps.CompletionProvider == nil {
		t.Error("expected CompletionProvider set, got nil")
	}
	if caps.DefinitionProvider != true {
		t.Error("expected DefinitionProvider true, got false")
	}
}

func TestBuildCapabilities_Empty(t *testing.T) {
	s := NewServer("test", "1.0")
	caps := s.buildCapabilities()

	if caps.TextDocumentSync == nil {
		t.Error("expected TextDocumentSync set, got nil")
	}
	if caps.Workspace == nil {
		t.Error("expected Workspace set, got nil")
	}
	if caps.HoverProvider != nil {
		t.Error("expected HoverProvider nil for empty server")
	}
	if caps.CompletionProvider != nil {
		t.Error("expected CompletionProvider nil for empty server")
	}
	if caps.DefinitionProvider != nil {
		t.Error("expected DefinitionProvider nil for empty server")
	}
}

func TestBuildCapabilities_AllHandlers(t *testing.T) {
	s := NewServer("test", "1.0")
	s.OnHover(func(ctx *Context, p *protocol.HoverParams) (*protocol.Hover, error) { return nil, nil })
	s.OnCompletion(func(ctx *Context, p *protocol.CompletionParams) (*protocol.CompletionList, error) { return nil, nil })
	s.OnDefinition(func(ctx *Context, p *protocol.DefinitionParams) ([]protocol.Location, error) { return nil, nil })
	s.OnReferences(func(ctx *Context, p *protocol.ReferenceParams) ([]protocol.Location, error) { return nil, nil })
	s.OnDocumentSymbol(func(ctx *Context, p *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) { return nil, nil })
	s.OnCodeAction(func(ctx *Context, p *protocol.CodeActionParams) ([]protocol.CodeAction, error) { return nil, nil })
	s.OnFormatting(func(ctx *Context, p *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) { return nil, nil })
	s.OnRename(func(ctx *Context, p *protocol.RenameParams) (*protocol.WorkspaceEdit, error) { return nil, nil })
	s.OnSignatureHelp(func(ctx *Context, p *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) { return nil, nil })
	s.OnDocumentHighlight(func(ctx *Context, p *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) { return nil, nil })
	s.OnFoldingRange(func(ctx *Context, p *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) { return nil, nil })
	s.OnInlayHint(func(ctx *Context, p *protocol.InlayHintParams) ([]protocol.InlayHint, error) { return nil, nil })
	s.OnCodeLens(func(ctx *Context, p *protocol.CodeLensParams) ([]protocol.CodeLens, error) { return nil, nil })
	s.OnWorkspaceSymbol(func(ctx *Context, p *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) { return nil, nil })
	s.OnDocumentLink(func(ctx *Context, p *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) { return nil, nil })
	s.OnSelectionRange(func(ctx *Context, p *protocol.SelectionRangeParams) ([]protocol.SelectionRange, error) { return nil, nil })

	caps := s.buildCapabilities()

	checks := []struct {
		name  string
		check func() bool
	}{
		{"HoverProvider", func() bool { return caps.HoverProvider == true }},
		{"CompletionProvider", func() bool { return caps.CompletionProvider != nil }},
		{"DefinitionProvider", func() bool { return caps.DefinitionProvider == true }},
		{"ReferencesProvider", func() bool { return caps.ReferencesProvider == true }},
		{"DocumentSymbolProvider", func() bool { return caps.DocumentSymbolProvider == true }},
		{"CodeActionProvider", func() bool { return caps.CodeActionProvider == true }},
		{"DocumentFormattingProvider", func() bool { return caps.DocumentFormattingProvider == true }},
		{"RenameProvider", func() bool { return caps.RenameProvider == true }},
		{"SignatureHelpProvider", func() bool { return caps.SignatureHelpProvider != nil }},
		{"DocumentHighlightProvider", func() bool { return caps.DocumentHighlightProvider == true }},
		{"FoldingRangeProvider", func() bool { return caps.FoldingRangeProvider == true }},
		{"InlayHintProvider", func() bool { return caps.InlayHintProvider == true }},
		{"CodeLensProvider", func() bool { return caps.CodeLensProvider != nil }},
		{"WorkspaceSymbolProvider", func() bool { return caps.WorkspaceSymbolProvider == true }},
		{"DocumentLinkProvider", func() bool { return caps.DocumentLinkProvider != nil }},
		{"SelectionRangeProvider", func() bool { return caps.SelectionRangeProvider == true }},
	}

	for _, c := range checks {
		if !c.check() {
			t.Errorf("%s: capability not set as expected", c.name)
		}
	}
}

func TestServerCheckAnalyze(t *testing.T) {
	s := NewServer("test", "1.0")
	opt := WithTreeSitter(treesitter.Config{})
	opt(s)

	s.Check("test-check", treesitter.Check{
		Pattern:  "(ERROR) @err",
		Severity: protocol.SeverityError,
		Message:  func(treesitter.Capture) string { return "error" },
	})
	s.Analyze("test-analyzer", treesitter.Analyzer{
		Run: func(*treesitter.AnalysisContext) []protocol.Diagnostic { return nil },
	})

	if s.diagEngine == nil {
		t.Fatal("diagEngine is nil after WithTreeSitter and Check/Analyze")
	}
}

func TestFolderFor(t *testing.T) {
	s := NewServer("test", "1.0")
	s.mu.Lock()
	s.workspaceFolders = []protocol.WorkspaceFolder{
		{URI: "file:///workspace/project-a", Name: "project-a"},
		{URI: "file:///workspace/project-b", Name: "project-b"},
	}
	s.mu.Unlock()

	tests := []struct {
		uri      protocol.DocumentURI
		wantName string
	}{
		{"file:///workspace/project-a/src/main.go", "project-a"},
		{"file:///workspace/project-b/pkg/foo.go", "project-b"},
		{"file:///workspace/project-a/deep/nested/file.go", "project-a"},
	}

	for _, tt := range tests {
		folder := s.FolderFor(tt.uri)
		if folder == nil {
			t.Errorf("FolderFor(%q) = nil, want folder with Name %q", tt.uri, tt.wantName)
			continue
		}
		if folder.Name != tt.wantName {
			t.Errorf("FolderFor(%q).Name = %q, want %q", tt.uri, folder.Name, tt.wantName)
		}
	}

	// Non-matching URI should return nil
	noMatch := s.FolderFor("file:///other/workspace/file.go")
	if noMatch != nil {
		t.Errorf("FolderFor(file:///other/workspace/file.go) = %+v, want nil", noMatch)
	}
}

func TestDispatch_Initialize(t *testing.T) {
	s := NewServer("test-server", "1.0.0")
	for _, o := range s.opts {
		o(s)
	}

	rootURI := protocol.DocumentURI("file:///workspace")
	params := &protocol.InitializeParams{
		RootURI:      &rootURI,
		Capabilities: protocol.ClientCapabilities{},
	}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	result, err := s.dispatch(context.Background(), "initialize", json.RawMessage(raw))
	if err != nil {
		t.Fatalf("dispatch initialize: %v", err)
	}

	initResult, ok := result.(*protocol.InitializeResult)
	if !ok {
		t.Fatalf("expected *InitializeResult, got %T", result)
	}
	if initResult.ServerInfo.Name != "test-server" {
		t.Errorf("ServerInfo.Name = %q, want test-server", initResult.ServerInfo.Name)
	}
	if initResult.ServerInfo.Version != "1.0.0" {
		t.Errorf("ServerInfo.Version = %q, want 1.0.0", initResult.ServerInfo.Version)
	}
}

func TestDispatch_Shutdown(t *testing.T) {
	s := NewServer("test", "1.0")
	for _, o := range s.opts {
		o(s)
	}

	// Initialize first
	initParams, _ := json.Marshal(&protocol.InitializeParams{})
	s.dispatch(context.Background(), "initialize", json.RawMessage(initParams))

	result, err := s.dispatch(context.Background(), "shutdown", nil)
	if err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
	if result != nil {
		t.Errorf("shutdown result should be nil, got %v", result)
	}
}

func TestDispatch_UnknownMethod(t *testing.T) {
	s := NewServer("test", "1.0")
	for _, o := range s.opts {
		o(s)
	}
	// Initialize first
	initParams, _ := json.Marshal(&protocol.InitializeParams{})
	s.dispatch(context.Background(), "initialize", json.RawMessage(initParams))

	_, err := s.dispatch(context.Background(), "textDocument/nonExistentMethod", nil)
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
	rpcErr, ok := err.(*jsonrpc.Error)
	if !ok {
		t.Fatalf("expected *jsonrpc.Error, got %T: %v", err, err)
	}
	if rpcErr.Code != -32601 {
		t.Errorf("error code = %d, want -32601 (MethodNotFound)", rpcErr.Code)
	}
}

func TestDispatch_HoverHandler(t *testing.T) {
	s := NewServer("test", "1.0")
	s.OnHover(func(ctx *Context, p *protocol.HoverParams) (*protocol.Hover, error) {
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: "plaintext", Value: "test hover"},
		}, nil
	})
	for _, o := range s.opts {
		o(s)
	}
	s.Documents().Open(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: "file:///test.go", LanguageID: "go", Version: 1, Text: "package main",
		},
	})

	initParams, _ := json.Marshal(&protocol.InitializeParams{})
	s.dispatch(context.Background(), "initialize", json.RawMessage(initParams))

	params, _ := json.Marshal(&protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.go"},
			Position:     protocol.Position{Line: 0, Character: 5},
		},
	})

	result, err := s.dispatch(context.Background(), "textDocument/hover", json.RawMessage(params))
	if err != nil {
		t.Fatalf("hover error: %v", err)
	}
	if result == nil {
		t.Fatal("hover result is nil")
	}
	hover, ok := result.(*protocol.Hover)
	if !ok {
		t.Fatalf("expected *Hover, got %T", result)
	}
	if hover.Contents.Value != "test hover" {
		t.Errorf("hover value = %q, want 'test hover'", hover.Contents.Value)
	}
}

func TestDispatch_CustomHandler(t *testing.T) {
	s := NewServer("test", "1.0")
	s.HandleRequest("custom/method", func(ctx *Context, params json.RawMessage) (interface{}, error) {
		return map[string]string{"result": "ok"}, nil
	})
	for _, o := range s.opts {
		o(s)
	}

	initParams, _ := json.Marshal(&protocol.InitializeParams{})
	s.dispatch(context.Background(), "initialize", json.RawMessage(initParams))

	result, err := s.dispatch(context.Background(), "custom/method", nil)
	if err != nil {
		t.Fatalf("custom method error: %v", err)
	}
	m, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", result)
	}
	if m["result"] != "ok" {
		t.Errorf("result = %q, want 'ok'", m["result"])
	}
}

func TestDispatch_NotificationDidOpen(t *testing.T) {
	s := NewServer("test", "1.0")
	for _, o := range s.opts {
		o(s)
	}
	initParams, _ := json.Marshal(&protocol.InitializeParams{})
	s.dispatch(context.Background(), "initialize", json.RawMessage(initParams))

	params, _ := json.Marshal(&protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.go",
			LanguageID: "go",
			Version:    1,
			Text:       "package main",
		},
	})

	s.dispatchNotification(context.Background(), "textDocument/didOpen", json.RawMessage(params))

	doc := s.Documents().Get("file:///test.go")
	if doc == nil {
		t.Fatal("document not in store after didOpen")
	}
	if doc.Text() != "package main" {
		t.Errorf("text = %q, want 'package main'", doc.Text())
	}
}
