package lspclient

import (
	"context"
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
	}
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
	a.mu.Lock()
	defer a.mu.Unlock()

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
	a.mu.Lock()
	if t, ok := a.timers[uri]; ok {
		t.Stop()
		delete(a.timers, uri)
	}
	a.mu.Unlock()
	a.flush(uri)
}

// Clear removes all source data for a URI and cancels any pending timer.
func (a *DiagnosticAggregator) Clear(uri protocol.DocumentURI) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if t, ok := a.timers[uri]; ok {
		t.Stop()
		delete(a.timers, uri)
	}
	delete(a.sources, uri)
}

func (a *DiagnosticAggregator) flush(uri protocol.DocumentURI) {
	a.mu.Lock()
	sourcesMap := a.sources[uri]
	delete(a.timers, uri)

	var merged []protocol.Diagnostic
	for _, diags := range sourcesMap {
		merged = append(merged, diags...)
	}
	publishFn := a.publish
	a.mu.Unlock()

	if publishFn != nil {
		_ = publishFn(context.Background(), &protocol.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: merged,
		})
	}
}
