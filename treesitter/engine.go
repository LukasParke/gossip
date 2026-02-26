package treesitter

import (
	"context"
	"log/slog"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/document"
	"github.com/LukasParke/gossip/protocol"
)

// PublishFunc sends diagnostics to the LSP client. The server layer provides
// the concrete implementation backed by ClientProxy.
type PublishFunc func(ctx context.Context, params *protocol.PublishDiagnosticsParams) error

// DiagnosticEngine orchestrates declarative Checks and imperative Analyzers.
// It maintains per-file, per-check/analyzer diagnostic caches and
// automatically publishes merged results after every tree update.
//
// NewDiagnosticEngine registers the engine as an OnTreeUpdate callback on the
// Manager, so it runs after every parse or reparse. SetPublish must be called
// before any documents are opened, or diagnostics will not be sent. The cache
// stores the last result from each check/analyzer per file; on incremental edits
// only changed ranges are re-analyzed and results are merged with cached
// diagnostics from unchanged regions.
type DiagnosticEngine struct {
	mu        sync.Mutex
	checks    []namedCheck
	analyzers []namedAnalyzer

	// Per-file, per-check/analyzer name cache.
	cache map[protocol.DocumentURI]map[string][]protocol.Diagnostic

	store            *document.Store
	manager          *Manager
	publish          PublishFunc
	userDataProvider UserDataProvider
	logger           *slog.Logger
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
// Must be called before any documents are opened; if unset, onTreeUpdate
// returns early and diagnostics are never published.
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

// SetUserDataProvider registers a function that provides user-defined data for
// each document. The return value is set as AnalysisContext.UserData before
// each analyzer runs. This allows consumers to pass custom state (e.g., a
// semantic model) to analyzers.
func (e *DiagnosticEngine) SetUserDataProvider(fn UserDataProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.userDataProvider = fn
}

// ClearCache removes cached diagnostics for a document (on close).
func (e *DiagnosticEngine) ClearCache(uri protocol.DocumentURI) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.cache, uri)
}

// Invalidate clears cached diagnostics for a document and triggers a full
// re-evaluation of all checks and analyzers. This is useful for cross-file
// dependency invalidation: when file B changes, call Invalidate on file A
// if A depends on B.
func (e *DiagnosticEngine) Invalidate(uri protocol.DocumentURI) {
	tree := e.manager.GetTree(uri)
	if tree == nil {
		return
	}
	e.mu.Lock()
	delete(e.cache, uri)
	e.mu.Unlock()

	fullDiff := &TreeDiff{IsFullReparse: true, AffectedKinds: make(map[string]bool)}
	collectAllKinds(tree.RootNode(), fullDiff.AffectedKinds)

	e.onTreeUpdate(uri, &Tree{raw: tree.raw, src: tree.src, Diff: fullDiff})
}

// onTreeUpdate is called by the Manager after every parse/reparse.
func (e *DiagnosticEngine) onTreeUpdate(uri protocol.DocumentURI, tree *Tree) {
	e.mu.Lock()
	publishFn := e.publish
	if publishFn == nil {
		e.mu.Unlock()
		return
	}
	if len(e.checks) == 0 && len(e.analyzers) == 0 {
		e.mu.Unlock()
		return
	}

	checks := make([]namedCheck, len(e.checks))
	copy(checks, e.checks)
	analyzers := make([]namedAnalyzer, len(e.analyzers))
	copy(analyzers, e.analyzers)
	udp := e.userDataProvider

	fileCache := e.cache[uri]
	if fileCache == nil {
		fileCache = make(map[string][]protocol.Diagnostic)
		e.cache[uri] = fileCache
	}
	cacheCopy := make(map[string][]protocol.Diagnostic, len(fileCache))
	for k, v := range fileCache {
		cacheCopy[k] = v
	}
	e.mu.Unlock()

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

	e.runChecksUnlocked(tree, diff, lang, checks, cacheCopy)
	e.runAnalyzersUnlocked(uri, tree, diff, doc, lang, analyzers, udp, cacheCopy)

	var all []protocol.Diagnostic
	for _, diags := range cacheCopy {
		all = append(all, diags...)
	}

	e.mu.Lock()
	e.cache[uri] = cacheCopy
	e.mu.Unlock()

	version := doc.Version()
	if err := publishFn(context.Background(), &protocol.PublishDiagnosticsParams{
		URI:         uri,
		Version:     &version,
		Diagnostics: all,
	}); err != nil {
		e.logger.Warn("failed to publish diagnostics", "uri", uri, "error", err)
	}
}

func (e *DiagnosticEngine) runChecksUnlocked(
	tree *Tree,
	diff *TreeDiff,
	lang *tree_sitter.Language,
	checks []namedCheck,
	fileCache map[string][]protocol.Diagnostic,
) {
	enc := tree.Encoder()
	for _, nc := range checks {
		if diff.IsFullReparse {
			diags := e.executeCheck(tree, lang, nc, enc)
			fileCache[nc.name] = diags
			continue
		}

		if len(diff.ChangedRanges) == 0 {
			continue
		}

		freshCaptures := e.executeCheckInRanges(tree, lang, nc, diff.ChangedRanges, enc)

		prev := fileCache[nc.name]
		var kept []protocol.Diagnostic
		for _, d := range prev {
			if !d.Range.OverlapsAny(diff.ChangedRanges) {
				kept = append(kept, d)
			}
		}
		fileCache[nc.name] = append(kept, freshCaptures...)
	}
}

func (e *DiagnosticEngine) executeCheck(tree *Tree, lang *tree_sitter.Language, nc namedCheck, enc *Encoder) []protocol.Diagnostic {
	captures, err := tree.QueryCaptures(lang, nc.check.Pattern)
	if err != nil {
		e.logger.Warn("check query failed", "check", nc.name, "error", err)
		return nil
	}
	return capturesToDiagnostics(captures, nc, enc)
}

func (e *DiagnosticEngine) executeCheckInRanges(tree *Tree, lang *tree_sitter.Language, nc namedCheck, ranges []protocol.Range, enc *Encoder) []protocol.Diagnostic {
	captures, err := tree.QueryCapturesInRanges(lang, nc.check.Pattern, ranges)
	if err != nil {
		e.logger.Warn("check scoped query failed", "check", nc.name, "error", err)
		return nil
	}
	return capturesToDiagnostics(captures, nc, enc)
}

func capturesToDiagnostics(captures []Capture, nc namedCheck, enc *Encoder) []protocol.Diagnostic {
	var diags []protocol.Diagnostic
	for _, c := range captures {
		if nc.check.DeduplicateNested && hasChildOfSameKind(c.Node) {
			continue
		}
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
		d := protocol.Diagnostic{
			Range:    enc.NodeRange(c.Node),
			Severity: nc.check.Severity,
			Source:   source,
			Message:  msg,
		}
		if nc.check.Code != "" {
			d.Code = nc.check.Code
		}
		if nc.check.CodeDescription != nil {
			d.CodeDescription = nc.check.CodeDescription
		}
		if nc.check.Tags != nil {
			d.Tags = nc.check.Tags
		}
		diags = append(diags, d)
	}
	return diags
}

func (e *DiagnosticEngine) runAnalyzersUnlocked(
	uri protocol.DocumentURI,
	tree *Tree,
	diff *TreeDiff,
	doc *document.Document,
	lang *tree_sitter.Language,
	analyzers []namedAnalyzer,
	udp UserDataProvider,
	fileCache map[string][]protocol.Diagnostic,
) {
	var userData interface{}
	if udp != nil {
		userData = udp(uri)
	}

	type result struct {
		name  string
		diags []protocol.Diagnostic
	}

	var eligible []namedAnalyzer
	for _, na := range analyzers {
		if diff.IsFullReparse || analyzerShouldRun(na.analyzer, diff) {
			eligible = append(eligible, na)
		}
	}

	if len(eligible) == 0 {
		return
	}

	results := make([]result, len(eligible))
	var wg sync.WaitGroup
	wg.Add(len(eligible))

	for i, na := range eligible {
		i, na := i, na
		go func() {
			defer wg.Done()

			actx := &AnalysisContext{
				Context:  context.Background(),
				Tree:     tree,
				Diff:     diff,
				Document: doc,
				Language: lang,
				Previous: fileCache[na.name],
				UserData: userData,
			}

			results[i] = result{name: na.name, diags: na.analyzer.Run(actx)}
		}()
	}

	wg.Wait()

	for _, r := range results {
		fileCache[r.name] = r.diags
	}
}

func analyzerShouldRun(a Analyzer, diff *TreeDiff) bool {
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

// hasChildOfSameKind reports whether node has any child whose Kind() matches
// the node's own Kind(). Used by DeduplicateNested to skip parent ERROR nodes
// when a more specific child ERROR exists.
func hasChildOfSameKind(node *tree_sitter.Node) bool {
	if node == nil {
		return false
	}
	kind := node.Kind()
	count := node.ChildCount()
	for i := uint(0); i < uint(count); i++ {
		if child := node.Child(i); child != nil && child.Kind() == kind {
			return true
		}
	}
	return false
}
