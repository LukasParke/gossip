package treesitter

import (
	"context"
	"log/slog"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/gossip-lsp/gossip/document"
	"github.com/gossip-lsp/gossip/protocol"
)

// PublishFunc sends diagnostics to the LSP client. The server layer provides
// the concrete implementation backed by ClientProxy.
type PublishFunc func(ctx context.Context, params *protocol.PublishDiagnosticsParams) error

// DiagnosticEngine orchestrates declarative Checks and imperative Analyzers.
// It maintains per-file, per-check/analyzer diagnostic caches and
// automatically publishes merged results after every tree update.
type DiagnosticEngine struct {
	mu        sync.Mutex
	checks    []namedCheck
	analyzers []namedAnalyzer

	// Per-file, per-check/analyzer name cache.
	cache map[protocol.DocumentURI]map[string][]protocol.Diagnostic

	store    *document.Store
	manager  *Manager
	publish  PublishFunc
	logger   *slog.Logger
}

type namedCheck struct {
	name  string
	check Check
}

type namedAnalyzer struct {
	name     string
	analyzer Analyzer
}

// NewDiagnosticEngine creates an engine tied to a tree-sitter Manager. The
// engine registers itself as the onTreeUpdate callback on the manager.
func NewDiagnosticEngine(manager *Manager, store *document.Store, logger *slog.Logger) *DiagnosticEngine {
	e := &DiagnosticEngine{
		cache:   make(map[protocol.DocumentURI]map[string][]protocol.Diagnostic),
		store:   store,
		manager: manager,
		logger:  logger,
	}
	manager.OnTreeUpdate(e.onTreeUpdate)
	return e
}

// SetPublish sets the function used to send diagnostics to the client.
// This must be called before the engine starts processing tree updates (i.e.,
// before any documents are opened).
func (e *DiagnosticEngine) SetPublish(fn PublishFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.publish = fn
}

// RegisterCheck adds a declarative check to the engine.
func (e *DiagnosticEngine) RegisterCheck(name string, c Check) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.checks = append(e.checks, namedCheck{name: name, check: c})
}

// RegisterAnalyzer adds an imperative analyzer to the engine.
func (e *DiagnosticEngine) RegisterAnalyzer(name string, a Analyzer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.analyzers = append(e.analyzers, namedAnalyzer{name: name, analyzer: a})
}

// ClearCache removes cached diagnostics for a document (on close).
func (e *DiagnosticEngine) ClearCache(uri protocol.DocumentURI) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.cache, uri)
}

// onTreeUpdate is called by the Manager after every parse/reparse.
func (e *DiagnosticEngine) onTreeUpdate(uri protocol.DocumentURI, tree *Tree) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.publish == nil {
		return
	}
	if len(e.checks) == 0 && len(e.analyzers) == 0 {
		return
	}

	doc := e.store.Get(uri)
	if doc == nil {
		return
	}

	lang, err := e.manager.Registry().LanguageFor(string(uri))
	if err != nil {
		return
	}

	diff := tree.Diff
	if diff == nil {
		diff = &TreeDiff{IsFullReparse: true, AffectedKinds: make(map[string]bool)}
	}

	fileCache := e.cache[uri]
	if fileCache == nil {
		fileCache = make(map[string][]protocol.Diagnostic)
		e.cache[uri] = fileCache
	}

	e.runChecks(uri, tree, diff, lang, fileCache)
	e.runAnalyzers(uri, tree, diff, doc, lang, fileCache)

	var all []protocol.Diagnostic
	for _, diags := range fileCache {
		all = append(all, diags...)
	}

	_ = e.publish(context.Background(), &protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: all,
	})
}

func (e *DiagnosticEngine) runChecks(
	uri protocol.DocumentURI,
	tree *Tree,
	diff *TreeDiff,
	lang *tree_sitter.Language,
	fileCache map[string][]protocol.Diagnostic,
) {
	for _, nc := range e.checks {
		if diff.IsFullReparse {
			diags := e.executeCheck(tree, lang, nc)
			fileCache[nc.name] = diags
			continue
		}

		if len(diff.ChangedRanges) == 0 {
			continue
		}

		freshCaptures := e.executeCheckInRanges(tree, lang, nc, diff.ChangedRanges)

		prev := fileCache[nc.name]
		var kept []protocol.Diagnostic
		for _, d := range prev {
			if !rangesOverlapAny(d.Range, diff.ChangedRanges) {
				kept = append(kept, d)
			}
		}
		fileCache[nc.name] = append(kept, freshCaptures...)
	}
}

func (e *DiagnosticEngine) executeCheck(tree *Tree, lang *tree_sitter.Language, nc namedCheck) []protocol.Diagnostic {
	captures, err := tree.QueryCaptures(lang, nc.check.Pattern)
	if err != nil {
		e.logger.Warn("check query failed", "check", nc.name, "error", err)
		return nil
	}
	return capturesToDiagnostics(captures, nc)
}

func (e *DiagnosticEngine) executeCheckInRanges(tree *Tree, lang *tree_sitter.Language, nc namedCheck, ranges []protocol.Range) []protocol.Diagnostic {
	captures, err := tree.QueryCapturesInRanges(lang, nc.check.Pattern, ranges)
	if err != nil {
		e.logger.Warn("check scoped query failed", "check", nc.name, "error", err)
		return nil
	}
	return capturesToDiagnostics(captures, nc)
}

func capturesToDiagnostics(captures []Capture, nc namedCheck) []protocol.Diagnostic {
	var diags []protocol.Diagnostic
	for _, c := range captures {
		if nc.check.Filter != nil && !nc.check.Filter(c) {
			continue
		}
		msg := c.Text
		if nc.check.Message != nil {
			msg = nc.check.Message(c)
		}
		source := nc.check.Source
		if source == "" {
			source = nc.name
		}
		diags = append(diags, protocol.Diagnostic{
			Range:    NodeRange(c.Node),
			Severity: nc.check.Severity,
			Source:   source,
			Message:  msg,
		})
	}
	return diags
}

func (e *DiagnosticEngine) runAnalyzers(
	uri protocol.DocumentURI,
	tree *Tree,
	diff *TreeDiff,
	doc *document.Document,
	lang *tree_sitter.Language,
	fileCache map[string][]protocol.Diagnostic,
) {
	for _, na := range e.analyzers {
		previous := fileCache[na.name]

		if !diff.IsFullReparse && !e.analyzerShouldRun(na.analyzer, diff) {
			continue
		}

		actx := &AnalysisContext{
			Context:  context.Background(),
			Tree:     tree,
			Diff:     diff,
			Document: doc,
			Language: lang,
			Previous: previous,
		}

		result := na.analyzer.Run(actx)
		fileCache[na.name] = result
	}
}

func (e *DiagnosticEngine) analyzerShouldRun(a Analyzer, diff *TreeDiff) bool {
	if len(a.InterestKinds) == 0 {
		return true
	}
	for _, kind := range a.InterestKinds {
		if diff.AffectsKind(kind) {
			return true
		}
	}
	return false
}
