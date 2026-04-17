package jsonschema

// Adapter is a plug point that lets consumers inject an external
// validation engine in place of (or in addition to) the built-in
// gossip/jsonschema interpreter.
//
// Background: downstream toolchains (for example barrelman) own a
// conformant JSON Schema 2020-12 engine built on santhosh-tekuri/jsonschema.
// Gossip cannot import barrelman directly because that would introduce a
// dependency cycle (barrelman depends on navigator, navigator depends on
// gossip). The Adapter interface lets those consumers register their
// engine through dependency injection while keeping gossip's package
// self-contained for users who do not want the extra dependency.
//
// Call RegisterAdapter at startup (for example from telescope's main
// package) to route validation through the external engine. When no
// adapter is registered, validators fall back to the original
// gossip/jsonschema interpreter.
//
// This adapter shim is deliberately minimal (an opaque validator function
// plus a rule-id normalizer). Tier-up to a richer contract only when an
// actual consumer needs it.
type Adapter struct {
	// Validate receives the raw schema bytes (JSON) and a decoded
	// instance value, and returns a slice of AdapterIssue values. nil
	// means the instance conforms; no error return to keep the hot path
	// simple — wrap the underlying engine's error into an AdapterIssue
	// at severity level "error".
	Validate func(schema []byte, instance any) []AdapterIssue
}

// AdapterIssue is a diagnostic shape the adapter can emit. It is
// intentionally a subset of the consumer's native Issue shape; the
// consumer is responsible for translating to LSP protocol.Diagnostic.
type AdapterIssue struct {
	Code       string
	Message    string
	Pointer    string
	Path       string
	Expected   string
	Received   string
	Suggestion string
}

var registered *Adapter

// RegisterAdapter installs the process-global adapter. Passing nil clears
// it and restores the built-in interpreter. Safe to call once from
// program init.
func RegisterAdapter(a *Adapter) {
	registered = a
}

// CurrentAdapter returns the installed adapter, or nil when the built-in
// interpreter should be used.
func CurrentAdapter() *Adapter {
	return registered
}
