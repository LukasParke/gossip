package gossip

import "github.com/gossip-lsp/gossip/protocol"

// buildCapabilities inspects which handlers are registered and returns
// a ServerCapabilities struct that accurately reflects what the server supports.
func (s *Server) buildCapabilities() protocol.ServerCapabilities {
	caps := protocol.ServerCapabilities{}

	syncOpts := &protocol.TextDocumentSyncOptions{
		OpenClose: true,
		Change:    protocol.SyncIncremental,
	}
	if _, ok := s.getHandler(protocol.MethodDidSave); ok {
		syncOpts.Save = &protocol.SaveOptions{IncludeText: true}
	}
	caps.TextDocumentSync = syncOpts

	if _, ok := s.getHandler(protocol.MethodHover); ok {
		caps.HoverProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodCompletion); ok {
		caps.CompletionProvider = &protocol.CompletionOptions{}
	}
	if _, ok := s.getHandler(protocol.MethodDefinition); ok {
		caps.DefinitionProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodReferences); ok {
		caps.ReferencesProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodDocumentSymbol); ok {
		caps.DocumentSymbolProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodCodeAction); ok {
		caps.CodeActionProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodFormatting); ok {
		caps.DocumentFormattingProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodRangeFormatting); ok {
		caps.DocumentRangeFormattingProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodRename); ok {
		caps.RenameProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodPrepareRename); ok {
		caps.RenameProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodSignatureHelp); ok {
		caps.SignatureHelpProvider = &protocol.SignatureHelpOptions{}
	}
	if _, ok := s.getHandler(protocol.MethodDocumentHighlight); ok {
		caps.DocumentHighlightProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodFoldingRange); ok {
		caps.FoldingRangeProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodInlayHint); ok {
		caps.InlayHintProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodSemanticTokensFull); ok {
		caps.SemanticTokensProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodCodeLens); ok {
		caps.CodeLensProvider = &protocol.CodeLensOptions{}
	}
	if _, ok := s.getHandler(protocol.MethodWorkspaceSymbol); ok {
		caps.WorkspaceSymbolProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodDeclaration); ok {
		caps.DeclarationProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodTypeDefinition); ok {
		caps.TypeDefinitionProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodImplementation); ok {
		caps.ImplementationProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodDocumentLink); ok {
		caps.DocumentLinkProvider = &protocol.DocumentLinkOptions{}
	}
	if _, ok := s.getHandler(protocol.MethodSelectionRange); ok {
		caps.SelectionRangeProvider = true
	}
	if _, ok := s.getHandler(protocol.MethodExecuteCommand); ok {
		caps.ExecuteCommandProvider = &protocol.ExecuteCommandOptions{}
	}

	caps.Workspace = &protocol.ServerWorkspaceCapabilities{
		WorkspaceFolders: &protocol.WorkspaceFoldersServerCapabilities{
			Supported:           true,
			ChangeNotifications: true,
		},
	}

	return caps
}
