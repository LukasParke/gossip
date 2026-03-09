package treesitter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/LukasParke/gossip/document"
	"github.com/LukasParke/gossip/protocol"
)

// #region agent log
func debugPublishDirect(uri string, preCount, postCount int) {
	entry := map[string]interface{}{"sessionId": "f3b0db", "location": "engine.go:PublishDirect", "message": "PublishDirect called", "data": map[string]interface{}{"uri": uri, "preXformCount": preCount, "postXformCount": postCount}, "timestamp": time.Now().UnixMilli()}
	b, _ := json.Marshal(entry)
	f, err := os.OpenFile("/home/luke/Documents/GitHub/telescope/.cursor/debug-f3b0db.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	f.Write(append(b, '\n'))
	f.Close()
}

var _ = fmt.Sprintf // keep fmt import

// #endregion

// PublishFunc sends diagnostics to the LSP client. The server layer provides
// the concrete implementation backed by ClientProxy.
type PublishFunc func(ctx context.Context, params *protocol.PublishDiagnosticsParams) error

// DiagnosticTransformer is a post-processing function applied to the merged
// diagnostic slice just before publishing. It can filter, modify, or reorder
// diagnostics. Use SetDiagnosticTransformer to install one. A nil transformer
// is a no-op.
type DiagnosticTransformer func(uri protocol.DocumentURI, diags []protocol.Diagnostic) []protocol.Diagnostic

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
	transformer      DiagnosticTransformer
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

// SetDiagnosticTransformer installs a post-processing function that is applied
// to the merged diagnostic slice just before publishing. The transformer can
// filter disabled rules, override severities, or reorder diagnostics. It is
// safe to call at any time; the new transformer takes effect on the next
// publish cycle. Pass nil to remove a previously installed transformer.
func (e *DiagnosticEngine) SetDiagnosticTransformer(fn DiagnosticTransformer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.transformer = fn
}

// PublishDirect sends diagnostics for any URI directly to the client,
// bypassing the store gate and the per-check cache. This is used by the
// project-level diagnostic system to publish diagnostics for files that may
// not be open in the editor. The transformer is still applied.
func (e *DiagnosticEngine) PublishDirect(ctx context.Context, uri protocol.DocumentURI, diags []protocol.Diagnostic) error {
	e.mu.Lock()
	publishFn := e.publish
	xform := e.transformer
	e.mu.Unlock()

	if publishFn == nil {
		return nil
	}

	preXformCount := len(diags)
	if xform != nil {
		diags = xform(uri, diags)
	}
	// #region agent log
	debugPublishDirect(string(uri), preXformCount, len(diags))
	// #endregion

	if diags == nil {
		diags = []protocol.Diagnostic{}
	}

	return publishFn(ctx, &protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diags,
	})
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

// InvalidateAll clears cached diagnostics for all documents and triggers a
// full re-evaluation. Use this when a configuration change (e.g., ruleset
// reload) affects the output of all rules across all open files.
func (e *DiagnosticEngine) InvalidateAll() {
	e.mu.Lock()
	uris := make([]protocol.DocumentURI, 0, len(e.cache))
	for uri := range e.cache {
		uris = append(uris, uri)
	}
	e.mu.Unlock()

	for _, uri := range uris {
		e.Invalidate(uri)
	}
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
	xform := e.transformer

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

	totalDiags := 0
	for _, diags := range cacheCopy {
		totalDiags += len(diags)
	}
	all := make([]protocol.Diagnostic, 0, totalDiags)
	for _, diags := range cacheCopy {
		all = append(all, diags...)
	}

	e.mu.Lock()
	e.cache[uri] = cacheCopy
	e.mu.Unlock()

	if xform != nil {
		all = xform(uri, all)
	}
	if all == nil {
		all = []protocol.Diagnostic{}
	}

	// #region agent log
	debugPublishDirect(string(uri)+"#onTreeUpdate", -1, len(all))
	// #endregion
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

		// ChangedRanges are in post-edit coordinates, while cached diagnostics
		// are from the previous document snapshot. Merging by range overlap can
		// therefore retain stale diagnostics or drop valid ones. Re-run the check
		// for the whole file to keep results deterministic.
		diags := e.executeCheck(tree, lang, nc, enc)
		fileCache[nc.name] = diags
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
	diags := make([]protocol.Diagnostic, 0, len(captures))
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
		rng := enc.NodeRange(c.Node)
		// Constrain multi-line ranges to a single line so diagnostics don't
		// highlight leading whitespace on subsequent lines.
		if rng.End.Line > rng.Start.Line {
			rng.End = protocol.Position{Line: rng.Start.Line, Character: rng.Start.Character + 1000}
		}
		d := protocol.Diagnostic{
			Range:    rng,
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

	const parallelThreshold = 4

	if len(eligible) <= parallelThreshold {
		for _, na := range eligible {
			actx := &AnalysisContext{
				Context:  context.Background(),
				Tree:     tree,
				Diff:     diff,
				Document: doc,
				Language: lang,
				Previous: fileCache[na.name],
				UserData: userData,
			}
			fileCache[na.name] = na.analyzer.Run(actx)
		}
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
