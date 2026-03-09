package lspclient

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/LukasParke/gossip/protocol"
)

// PublishFunc sends merged diagnostics for a URI to the editor.
type PublishFunc func(ctx context.Context, params *protocol.PublishDiagnosticsParams) error

// DiagnosticAggregator collects diagnostics from multiple sources (e.g.
// tree-sitter engine, child YAML LSP, child JSON LSP) per document URI and
// publishes the merged set after a short debounce window. This prevents the
// editor from seeing partial diagnostic updates as each source reports in.
type DiagnosticAggregator struct {
	mu      sync.Mutex
	sources map[protocol.DocumentURI]map[string][]protocol.Diagnostic // uri -> source -> diags
	timers  map[protocol.DocumentURI]*time.Timer
	publish PublishFunc
	delay   time.Duration
	logger  *slog.Logger
}

// NewDiagnosticAggregator creates an aggregator that publishes via the given
// function. Pass nil for publish if it will be set later via SetPublishFunc.
// The delay controls the debounce window; 80ms is a reasonable default for
// merging near-simultaneous updates.
func NewDiagnosticAggregator(publish PublishFunc, delay time.Duration) *DiagnosticAggregator {
	return &DiagnosticAggregator{
		sources: make(map[protocol.DocumentURI]map[string][]protocol.Diagnostic),
		timers:  make(map[protocol.DocumentURI]*time.Timer),
		publish: publish,
		delay:   delay,
		logger:  slog.Default(),
	}
}

// SetLogger sets the logger used for debug tracing of aggregator operations.
func (a *DiagnosticAggregator) SetLogger(l *slog.Logger) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logger = l
}

// SetPublishFunc sets (or replaces) the function used to publish merged
// diagnostics to the editor. This is safe to call at any time; subsequent
// flushes will use the new function.
func (a *DiagnosticAggregator) SetPublishFunc(fn PublishFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.publish = fn
}

// Set records diagnostics from a named source for the given URI and schedules
// a debounced flush. Multiple rapid calls for the same URI reset the timer.
func (a *DiagnosticAggregator) Set(uri protocol.DocumentURI, source string, diags []protocol.Diagnostic) {
	uri = protocol.NormalizeURI(uri)
	a.mu.Lock()
	defer a.mu.Unlock()

	a.logger.Debug("aggregator.Set", "uri", uri, "source", source, "count", len(diags))

	if a.sources[uri] == nil {
		a.sources[uri] = make(map[string][]protocol.Diagnostic)
	}
	a.sources[uri][source] = diags

	if t, ok := a.timers[uri]; ok {
		t.Stop()
	}
	a.timers[uri] = time.AfterFunc(a.delay, func() {
		a.flush(uri)
	})
}

// FlushNow immediately publishes the merged diagnostics for a URI, bypassing
// the debounce timer. Useful on didClose to ensure a clean state.
func (a *DiagnosticAggregator) FlushNow(uri protocol.DocumentURI) {
	uri = protocol.NormalizeURI(uri)
	a.logger.Debug("aggregator.FlushNow", "uri", uri)
	a.mu.Lock()
	if t, ok := a.timers[uri]; ok {
		t.Stop()
		delete(a.timers, uri)
	}
	a.mu.Unlock()
	a.flush(uri)
}

// ClearSource removes diagnostics for a single named source and schedules a
// debounced flush so the merged result is updated without wiping other sources.
// Use this instead of Clear when only one contributor (e.g. a child LSP) is
// being closed while others (e.g. the main diagnostic engine) should keep
// their diagnostics intact.
func (a *DiagnosticAggregator) ClearSource(uri protocol.DocumentURI, source string) {
	uri = protocol.NormalizeURI(uri)
	a.mu.Lock()
	defer a.mu.Unlock()

	a.logger.Debug("aggregator.ClearSource", "uri", uri, "source", source)
	if sources, ok := a.sources[uri]; ok {
		delete(sources, source)
	}

	if t, ok := a.timers[uri]; ok {
		t.Stop()
	}
	a.timers[uri] = time.AfterFunc(a.delay, func() {
		a.flush(uri)
	})
}

// Clear removes all source data for a URI, cancels any pending timer, and
// publishes an empty diagnostic set so the client has a clean slate. This
// ensures that a reclassification cycle (didClose + didOpen) does not leave
// the client with stale diagnostics or miss the new set.
func (a *DiagnosticAggregator) Clear(uri protocol.DocumentURI) {
	uri = protocol.NormalizeURI(uri)
	a.mu.Lock()

	a.logger.Debug("aggregator.Clear", "uri", uri)

	if t, ok := a.timers[uri]; ok {
		t.Stop()
		delete(a.timers, uri)
	}
	delete(a.sources, uri)

	publishFn := a.publish
	a.mu.Unlock()

	if publishFn != nil {
		_ = publishFn(context.Background(), &protocol.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: []protocol.Diagnostic{},
		})
	}
}

func (a *DiagnosticAggregator) flush(uri protocol.DocumentURI) {
	a.mu.Lock()
	sourcesMap := a.sources[uri]
	delete(a.timers, uri)

	merged := make([]protocol.Diagnostic, 0)
	numSources := len(sourcesMap)
	srcCounts := make(map[string]int)
	for src, diags := range sourcesMap {
		merged = append(merged, diags...)
		srcCounts[src] = len(diags)
	}
	publishFn := a.publish
	a.mu.Unlock()

	a.logger.Debug("aggregator.flush", "uri", uri, "totalDiags", len(merged), "sources", numSources)

	if publishFn != nil {
		_ = publishFn(context.Background(), &protocol.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: merged,
		})
	}
}
