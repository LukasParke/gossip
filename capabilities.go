package gossip

import "github.com/LukasParke/gossip/protocol"

// buildCapabilities inspects which handlers are registered and returns
// a ServerCapabilities struct that accurately reflects what the server supports.
func (s *Server) buildCapabilities() protocol.ServerCapabilities {
	s.mu.RLock()
	handlers := make(map[string]bool, len(s.handlers))
	for method := range s.handlers {
		handlers[method] = true
	}
	s.mu.RUnlock()

	has := func(method string) bool { return handlers[method] }

	caps := protocol.ServerCapabilities{}

	syncOpts := &protocol.TextDocumentSyncOptions{
		OpenClose: true,
		Change:    protocol.SyncIncremental,
	}
	if has(protocol.MethodDidSave) {
		syncOpts.Save = &protocol.SaveOptions{IncludeText: true}
	}
	caps.TextDocumentSync = syncOpts

	if has(protocol.MethodHover) {
		caps.HoverProvider = true
	}
	if has(protocol.MethodCompletion) {
		opts := &protocol.CompletionOptions{}
		if len(s.completionTriggerChars) > 0 {
			opts.TriggerCharacters = s.completionTriggerChars
		}
		caps.CompletionProvider = opts
	}
	if has(protocol.MethodDefinition) {
		caps.DefinitionProvider = true
	}
	if has(protocol.MethodReferences) {
		caps.ReferencesProvider = true
	}
	if has(protocol.MethodDocumentSymbol) {
		caps.DocumentSymbolProvider = true
	}
	if has(protocol.MethodCodeAction) {
		caps.CodeActionProvider = true
	}
	if has(protocol.MethodFormatting) {
		caps.DocumentFormattingProvider = true
	}
	if has(protocol.MethodRangeFormatting) {
		caps.DocumentRangeFormattingProvider = true
	}
	if has(protocol.MethodRename) || has(protocol.MethodPrepareRename) {
		if has(protocol.MethodPrepareRename) {
			caps.RenameProvider = protocol.RenameOptions{PrepareProvider: true}
		} else {
			caps.RenameProvider = true
		}
	}
	if has(protocol.MethodSignatureHelp) {
		opts := &protocol.SignatureHelpOptions{}
		if len(s.signatureHelpTriggerChars) > 0 {
			opts.TriggerCharacters = s.signatureHelpTriggerChars
		}
		caps.SignatureHelpProvider = opts
	}
	if has(protocol.MethodDocumentHighlight) {
		caps.DocumentHighlightProvider = true
	}
	if has(protocol.MethodFoldingRange) {
		caps.FoldingRangeProvider = true
	}
	if has(protocol.MethodInlayHint) {
		caps.InlayHintProvider = true
	}
	if has(protocol.MethodSemanticTokensFull) {
		legend := protocol.SemanticTokensLegend{
			TokenTypes:     []string{},
			TokenModifiers: []string{},
		}
		if s.semanticTokensLegend != nil {
			legend = *s.semanticTokensLegend
		}
		opts := &protocol.SemanticTokensOptions{
			Legend: legend,
			Full:  true,
		}
		if has(protocol.MethodSemanticTokensRange) {
			opts.Range = true
		}
		caps.SemanticTokensProvider = opts
	}
	if has(protocol.MethodCodeLens) {
		caps.CodeLensProvider = &protocol.CodeLensOptions{}
	}
	if has(protocol.MethodWorkspaceSymbol) {
		caps.WorkspaceSymbolProvider = true
	}
	if has(protocol.MethodDeclaration) {
		caps.DeclarationProvider = true
	}
	if has(protocol.MethodTypeDefinition) {
		caps.TypeDefinitionProvider = true
	}
	if has(protocol.MethodImplementation) {
		caps.ImplementationProvider = true
	}
	if has(protocol.MethodDocumentLink) {
		caps.DocumentLinkProvider = &protocol.DocumentLinkOptions{}
	}
	if has(protocol.MethodSelectionRange) {
		caps.SelectionRangeProvider = true
	}
	if has(protocol.MethodExecuteCommand) {
		caps.ExecuteCommandProvider = &protocol.ExecuteCommandOptions{
			Commands: s.executeCommands,
		}
	}
	if has(protocol.MethodLinkedEditingRange) {
		caps.LinkedEditingRangeProvider = true
	}
	if has(protocol.MethodPrepareCallHierarchy) {
		caps.CallHierarchyProvider = true
	}
	if has(protocol.MethodPrepareTypeHierarchy) {
		caps.TypeHierarchyProvider = true
	}
	if has(protocol.MethodDocumentDiagnostic) {
		caps.DiagnosticProvider = &protocol.DiagnosticOptions{
			InterFileDependencies: false,
			WorkspaceDiagnostics:  false,
		}
	}
	if has(protocol.MethodCompletionResolve) && caps.CompletionProvider != nil {
		caps.CompletionProvider.ResolveProvider = true
	}
	if has(protocol.MethodDocumentLinkResolve) && caps.DocumentLinkProvider != nil {
		caps.DocumentLinkProvider.ResolveProvider = true
	}

	caps.Workspace = &protocol.ServerWorkspaceCapabilities{
		WorkspaceFolders: &protocol.WorkspaceFoldersServerCapabilities{
			Supported:           true,
			ChangeNotifications: true,
		},
	}

	return caps
}
