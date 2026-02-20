// Full-featured gossip LSP server demonstrating all major capabilities:
// hover, completion, definition, code actions, diagnostics (via Check/Analyze),
// formatting, middleware, config, and cross-editor transport support.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"strings"

	"github.com/LukasParke/gossip"
	"github.com/LukasParke/gossip/middleware"
	"github.com/LukasParke/gossip/protocol"
	"github.com/LukasParke/gossip/treesitter"
)

type Config struct {
	MaxCompletions int  `toml:"max_completions"`
	EnableLinting  bool `toml:"enable_linting"`
}

func main() {
	s := gossip.NewServer("complete-lsp", "0.1.0",
		gossip.WithMiddleware(
			middleware.Logging(slog.Default()),
			middleware.Recovery(),
		),
		gossip.WithConfig[Config](".complete-lsp.toml", Config{
			MaxCompletions: 50,
			EnableLinting:  true,
		}),
		// Enable tree-sitter for incremental diagnostics.
		// In a real server, you'd pass language grammars here:
		//   gossip.WithTreeSitter(treesitter.Config{
		//       Languages: map[string]*tree_sitter.Language{".go": goLang},
		//   }),
	)

	// --- Declarative checks (pattern-based, auto-scoped to changed ranges) ---

	// Syntax errors: automatically detected on each keystroke, scoped to only
	// the changed region. No OnDidChange handler needed.
	s.Check("syntax-errors", treesitter.Check{
		Pattern:  "(ERROR) @error",
		Severity: protocol.SeverityError,
		Message:  func(c treesitter.Capture) string { return fmt.Sprintf("Syntax error near '%s'", c.Text) },
	})

	// TODO comments: the Filter func restricts matches to only comments
	// containing "TODO". Diagnostics are cached, merged, and published
	// automatically.
	s.Check("todo-comments", treesitter.Check{
		Pattern:  "(comment) @comment",
		Severity: protocol.SeverityInformation,
		Filter:   func(c treesitter.Capture) bool { return strings.Contains(c.Text, "TODO") },
		Message:  func(c treesitter.Capture) string { return "TODO comment found" },
	})

	// --- Imperative analyzers (full control, cross-reference capable) ---

	// Duplicate function names: a ScopeFile analyzer that re-runs only when
	// function_declaration nodes are affected. Uses MergePrevious for partial
	// re-check.
	s.Analyze("duplicate-functions", treesitter.Analyzer{
		Scope:         treesitter.ScopeFile,
		InterestKinds: []string{"function_declaration"},
		Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
			if !ctx.Diff.IsFullReparse && !ctx.Diff.AffectsKind("function_declaration") {
				return ctx.Previous
			}

			captures, _ := ctx.Tree.QueryCaptures(ctx.Language,
				"(function_declaration name: (identifier) @name)")

			seen := map[string]treesitter.Capture{}
			var diags []protocol.Diagnostic
			for _, c := range captures {
				if prev, exists := seen[c.Text]; exists {
					diags = append(diags, protocol.Diagnostic{
						Range:    treesitter.NodeRange(c.Node),
						Severity: protocol.SeverityError,
						Source:   "duplicate-functions",
						Message:  fmt.Sprintf("Duplicate function '%s' (first declared at line %d)", c.Text, treesitter.NodeRange(prev.Node).Start.Line+1),
					})
				}
				seen[c.Text] = c
			}
			return diags
		},
	})

	// --- Traditional handlers still work alongside Check/Analyze ---

	s.OnHover(hoverHandler)
	s.OnCompletion(completionHandler)
	s.OnDefinition(definitionHandler)
	s.OnCodeAction(codeActionHandler)
	s.OnFormatting(formattingHandler)
	s.OnDocumentSymbol(symbolHandler)

	gossip.OnConfigChange(s, func(ctx *gossip.Context, old, new_ *Config) {
		ctx.Client.LogMessage(ctx, protocol.Info, "Config reloaded")
	})

	if err := gossip.Serve(s, gossip.FromArgs()); err != nil {
		log.Fatal(err)
	}
}

func hoverHandler(ctx *gossip.Context, p *protocol.HoverParams) (*protocol.Hover, error) {
	doc := ctx.Documents.Get(p.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}
	word := doc.WordAt(p.Position)
	if word == "" {
		return nil, nil
	}
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: fmt.Sprintf("### %s\n\nIdentifier at line %d, character %d", word, p.Position.Line+1, p.Position.Character+1),
		},
	}, nil
}

func completionHandler(ctx *gossip.Context, p *protocol.CompletionParams) (*protocol.CompletionList, error) {
	keywords := []string{"func", "var", "const", "type", "import", "package", "return", "if", "for", "switch"}
	items := make([]protocol.CompletionItem, len(keywords))
	for i, kw := range keywords {
		items[i] = protocol.CompletionItem{
			Label:  kw,
			Kind:   protocol.CompletionKindKeyword,
			Detail: "keyword",
		}
	}
	return &protocol.CompletionList{Items: items}, nil
}

func definitionHandler(ctx *gossip.Context, p *protocol.DefinitionParams) ([]protocol.Location, error) {
	return []protocol.Location{
		{
			URI: p.TextDocument.URI,
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 0},
			},
		},
	}, nil
}

func codeActionHandler(ctx *gossip.Context, p *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	var actions []protocol.CodeAction
	for _, diag := range p.Context.Diagnostics {
		if strings.Contains(diag.Message, "TODO") {
			actions = append(actions, protocol.CodeAction{
				Title:       "Remove TODO comment",
				Kind:        "quickfix",
				Diagnostics: []protocol.Diagnostic{diag},
			})
		}
	}
	return actions, nil
}

func formattingHandler(ctx *gossip.Context, p *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	doc := ctx.Documents.Get(p.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}
	text := doc.Text()
	trimmed := strings.TrimRight(text, " \t\n") + "\n"
	if trimmed == text {
		return nil, nil
	}
	lines := strings.Count(text, "\n")
	return []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: uint32(lines + 1), Character: 0},
			},
			NewText: trimmed,
		},
	}, nil
}

func symbolHandler(ctx *gossip.Context, p *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) {
	doc := ctx.Documents.Get(p.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}
	text := doc.Text()
	var symbols []protocol.DocumentSymbol
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "func ") {
			name := strings.TrimPrefix(strings.TrimSpace(line), "func ")
			if idx := strings.IndexByte(name, '('); idx > 0 {
				name = name[:idx]
			}
			symbols = append(symbols, protocol.DocumentSymbol{
				Name: name,
				Kind: protocol.SymbolFunction,
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(i)},
					End:   protocol.Position{Line: uint32(i), Character: uint32(len(line))},
				},
				SelectionRange: protocol.Range{
					Start: protocol.Position{Line: uint32(i), Character: 5},
					End:   protocol.Position{Line: uint32(i), Character: uint32(5 + len(name))},
				},
			})
		}
	}
	return symbols, nil
}
