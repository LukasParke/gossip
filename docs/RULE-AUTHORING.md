# Rule Authoring Guide

This document covers how to write diagnostic rules for gossip's tree-sitter diagnostic engine, how to test them with the JSON expectations framework, and a critical review of the current API ergonomics.

## Overview: Check vs Analyzer

Gossip provides two APIs for producing diagnostics:

| | Check | Analyzer |
|---|---|---|
| **Style** | Declarative, pattern-based | Imperative, full-control |
| **Input** | Tree-sitter query pattern | Full `AnalysisContext` with tree, diff, document |
| **Range scoping** | Automatic (framework scopes to changed ranges) | Manual (use `Diff.ChangedRanges` or `ScopeFile`) |
| **Caching** | Automatic per-check merge | Manual via `MergePrevious` |
| **Best for** | Structural patterns (ERROR nodes, specific AST shapes) | Semantic rules (duplicate keys, cross-node analysis) |

**Use a Check** when your rule can be expressed as "find all nodes matching pattern X." The framework handles incremental scoping, caching, and publishing.

**Use an Analyzer** when you need access to the full document, sibling/parent relationships, or cross-node logic that a single tree-sitter query cannot express.

## Writing a Check

A `Check` is registered via `Server.Check()` (or `DiagnosticEngine.RegisterCheck()` in tests):

```go
s.Check("syntax-errors", treesitter.Check{
    Pattern:           "(ERROR) @error",
    Severity:          protocol.SeverityError,
    DeduplicateNested: true,
    Message:           func(c treesitter.Capture) string { return "syntax error" },
})
```

### Check Fields

**`Pattern`** (required) -- A tree-sitter S-expression query. The framework runs this against the parse tree and produces one diagnostic per capture. Common patterns:

- `(ERROR) @error` -- all syntax error nodes
- `(comment) @c` -- all comments
- `(string) @s` -- all string literals
- `(function_declaration name: (identifier) @name)` -- function names

**`Severity`** (required) -- One of `protocol.SeverityError`, `SeverityWarning`, `SeverityInformation`, `SeverityHint`.

**`DeduplicateNested`** -- When `true`, the engine skips captured nodes that have a child of the same kind. This is critical for `(ERROR) @error` patterns: tree-sitter nests ERROR nodes inside other ERROR nodes, producing duplicate diagnostics with overlapping ranges. Enabling this keeps only the tightest (deepest) range per error. **Recommended for all ERROR-based checks.**

**`Source`** -- Diagnostic source string shown in the editor. Defaults to the check name if empty.

**`Code`** -- A rule identifier string (e.g., `"E001"`, `"no-trailing-comma"`).

**`CodeDescription`** -- Links to documentation for this diagnostic.

**`Tags`** -- LSP diagnostic tags like `DiagnosticTagUnnecessary` or `DiagnosticTagDeprecated`.

**`Filter`** -- Called for each capture. Return `true` to keep it, `false` to discard. Useful for narrowing a broad pattern:

```go
s.Check("todo-comments", treesitter.Check{
    Pattern:  "(comment) @c",
    Severity: protocol.SeverityInformation,
    Filter: func(c treesitter.Capture) bool {
        return strings.Contains(c.Text, "TODO")
    },
    Message: func(c treesitter.Capture) string {
        return "resolve TODO comment"
    },
})
```

**`Message`** -- Converts a capture into the diagnostic message string. Receives a `Capture` with `Name`, `Node`, and `Text` fields. If nil, defaults to the captured text.

## Writing an Analyzer

An `Analyzer` is registered via `Server.Analyze()` (or `DiagnosticEngine.RegisterAnalyzer()`):

```go
s.Analyze("duplicate-keys", treesitter.Analyzer{
    Scope:         treesitter.ScopeFile,
    InterestKinds: []string{"pair", "object"},
    Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
        // Full access to ctx.Tree, ctx.Diff, ctx.Document, ctx.Language
        // Return diagnostics with explicit ranges
    },
})
```

### Analyzer Fields

**`Scope`** -- Controls re-run behavior:
- `ScopeChanged` -- Restricts to changed ranges only (more efficient, but you only see diff regions)
- `ScopeFile` -- Re-runs on the entire file, but only when `InterestKinds` intersect with affected node kinds

**`InterestKinds`** -- When non-empty, the analyzer is skipped if none of these node kinds appear in the diff's `AffectedKinds`. An empty slice means "run on every edit." This is the primary optimization lever.

**`Run`** -- The analysis function. Receives an `AnalysisContext` with:

| Field | Description |
|---|---|
| `Tree` | The current parse tree (post-edit) |
| `Diff` | What changed: `IsFullReparse`, `ChangedRanges`, `AffectedKinds` |
| `Document` | The managed text document |
| `Language` | The tree-sitter language for queries |
| `Previous` | Cached diagnostics from the last run (nil on first run) |
| `UserData` | Arbitrary state from `SetUserDataProvider` |

### Using MergePrevious

For `ScopeFile` analyzers that only re-check affected regions but must return a complete diagnostic set:

```go
Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
    if ctx.Diff.IsFullReparse {
        return fullAnalysis(ctx.Tree)
    }
    fresh := analyzeChangedRegions(ctx)
    return ctx.MergePrevious(fresh)
},
```

`MergePrevious` drops any previous diagnostic whose range overlaps `ChangedRanges`, then appends your fresh diagnostics.

### Detecting MISSING Nodes

Tree-sitter inserts MISSING nodes during error recovery (e.g., a missing closing brace). These cannot be found via query patterns -- use `Tree.MissingNodes()`:

```go
s.Analyze("missing-tokens", treesitter.Analyzer{
    Scope: treesitter.ScopeFile,
    Run: func(ctx *treesitter.AnalysisContext) []protocol.Diagnostic {
        missing := ctx.Tree.MissingNodes()
        var diags []protocol.Diagnostic
        for _, node := range missing {
            diags = append(diags, protocol.Diagnostic{
                Range:    ctx.Tree.NodeRange(node),
                Severity: protocol.SeverityError,
                Source:   "missing-tokens",
                Message:  "expected " + node.Kind(),
            })
        }
        return diags
    },
})
```

## Testing Rules

Three testing approaches are available, from highest-level to lowest:

### 1. JSON Expectations (data-driven, recommended for range validation)

The file `treesitter/testdata/diagnostic_expectations.json` defines test scenarios declaratively. Each scenario specifies source text, checks to register, and the exact expected diagnostics with ranges.

**Schema:**

```json
{
  "scenarios": [
    {
      "name": "my_test_case",
      "description": "What this tests",
      "source": "{\"broken\": bad}",
      "language": ".json",
      "checks": [
        {
          "name": "syntax-errors",
          "pattern": "(ERROR) @error",
          "severity": 1,
          "message": "syntax error",
          "deduplicate_nested": true
        }
      ],
      "analyzers": [
        {
          "name": "missing-tokens",
          "type": "missing-nodes"
        }
      ],
      "expected": [
        {
          "message": "syntax error",
          "severity": 1,
          "source": "syntax-errors",
          "range": {
            "start": { "line": 0, "character": 11 },
            "end": { "line": 0, "character": 14 }
          }
        }
      ]
    }
  ]
}
```

**Adding a new scenario:**

1. Copy an existing scenario entry in the JSON
2. Set `name`, `description`, `source`, and configure `checks`/`analyzers`
3. Set `expected` to an empty array or placeholder ranges
4. Run `go test ./treesitter/ -run TestDiagnosticRanges -update`
5. Review the updated JSON -- the `-update` flag captures actual ranges
6. Commit the updated `diagnostic_expectations.json`

**Validating:**

```bash
go test ./treesitter/ -run TestDiagnosticRanges -v
```

**Updating after grammar or rule changes:**

```bash
go test ./treesitter/ -run TestDiagnosticRanges -update
git diff treesitter/testdata/diagnostic_expectations.json
```

### 2. Integration Tests with gossiptest.Client

For full server-level testing with a real LSP handshake:

```go
func TestMyRule(t *testing.T) {
    s := gossip.NewServer("test", "0.1.0",
        gossip.WithTreeSitter(treesitter.Config{
            Languages: map[string]*tree_sitter.Language{".json": jsonLang()},
        }),
    )
    s.Check("my-check", treesitter.Check{...})

    c := gossiptest.NewClient(t, s)
    c.Open(gossiptest.FileURI("/test.json"), `{"broken": bad}`)

    diags := c.WaitForDiagnostics(gossiptest.FileURI("/test.json"), 2*time.Second)

    gossiptest.AssertDiagnosticRanges(t, diags, []gossiptest.DiagnosticExpectation{
        {
            Message:  "syntax error",
            Severity: protocol.SeverityError,
            Source:   "my-check",
            Range:    gossiptest.Rng(0, 11, 0, 14),
        },
    })
}
```

**Available helpers:**

| Helper | Purpose |
|---|---|
| `gossiptest.NewClient(t, s)` | In-memory LSP client, auto-initializes |
| `c.Open(uri, text)` | Send `didOpen` |
| `c.Change(uri, version, text)` | Full content replacement |
| `c.ChangeIncremental(uri, version, rng, text)` | Range-based edit |
| `c.WaitForDiagnostics(uri, timeout)` | Block until diagnostics arrive |
| `c.LatestDiagnostics(uri)` | Non-blocking latest diagnostics |
| `gossiptest.AssertDiagnosticRanges(t, actual, expected)` | Range-aware comparison |
| `gossiptest.AssertDiagnosticCount(t, params, uri, count)` | Count assertion |
| `gossiptest.Pos(line, char)` | Position helper |
| `gossiptest.Rng(sLine, sChar, eLine, eChar)` | Range helper |
| `gossiptest.FileURI(path)` | URI helper |

### 3. Direct Engine Testing

For low-level testing without the full server:

```go
func TestMyCheck(t *testing.T) {
    store := document.NewStore()
    cfg := treesitter.Config{
        Languages: map[string]*tree_sitter.Language{".json": jsonLang()},
    }
    mgr := treesitter.NewManager(cfg, store)
    t.Cleanup(mgr.Close)

    engine := treesitter.NewDiagnosticEngine(mgr, store, slog.Default())

    pc := &publishCollector{}
    engine.SetPublish(pc.publish)

    engine.RegisterCheck("my-check", treesitter.Check{...})

    uri := protocol.DocumentURI("file:///test.json")
    store.Open(&protocol.DidOpenTextDocumentParams{
        TextDocument: protocol.TextDocumentItem{
            URI: uri, LanguageID: "json", Version: 1,
            Text: `{"broken": bad}`,
        },
    })

    p := pc.waitFor(t, string(uri), 2*time.Second)
    // Assert on p.Diagnostics
}
```

This pattern is used extensively in `treesitter/engine_test.go`. The `publishCollector`, `setup()`, and `waitFor` helpers are defined there but are not exported -- you must define equivalent helpers in your own test package.

### When to Use Each Approach

| Approach | Best for |
|---|---|
| JSON expectations | Range regression testing, grammar migration, bulk validation |
| gossiptest.Client | Full integration tests, testing server capabilities wiring |
| Direct engine | Unit-testing check/analyzer logic, debugging capture behavior |

## Ergonomic Friction Points and Recommendations

### 1. Nested ERROR Duplication

The `(ERROR) @error` pattern is the most common Check pattern, but without `DeduplicateNested: true` it silently produces duplicate diagnostics for every error. Tree-sitter nests ERROR nodes inside other ERROR nodes, so a single syntax error like `{"broken": bad}` generates both a broad range (`0:1-0:14`) and a tight range (`0:11-0:14`).

The `DeduplicateNested` field addresses this but is opt-in. This is a footgun: every new Check using ERROR patterns will produce duplicates unless the author knows to enable it.

**Recommendation:** Consider making `DeduplicateNested` the default behavior for checks whose pattern contains `ERROR`, or at minimum document this prominently in the `Check` GoDoc.

### 2. No MISSING Node Detection in Check

`Check` is purely query-based and cannot detect MISSING nodes -- a common class of syntax errors representing absent tokens (unterminated brackets, missing semicolons, etc.). Users must write a separate `Analyzer` for what conceptually belongs in the same "syntax errors" category.

**Recommendation:** Consider an `IncludeMissing bool` field on `Check` that automatically walks the tree for MISSING nodes and includes them in the diagnostic set. Alternatively, provide a built-in `"syntax-errors"` meta-check that combines `(ERROR) @error` with deduplication and MISSING node detection out of the box.

### 3. Capture Struct is Thin

`Capture` exposes `Name`, `Node`, and `Text`. Common operations in `Filter` and `Message` functions -- like checking the parent node's kind, testing if the capture is a leaf, or reading sibling text -- require importing `tree_sitter` directly and navigating the raw node API.

**Recommendation:** Add convenience methods to `Capture`:
- `ParentKind() string` -- the parent node's kind, or `""` if root
- `IsLeaf() bool` -- whether the node has no named children
- `HasChildOfKind(kind string) bool` -- for structural checks

### 4. Filter and Message Lack Context

`Check.Filter` and `Check.Message` receive only a `Capture`. They cannot access the document text outside the captured node, the full tree, or the language. For moderately complex rules (e.g., "flag this node only if a sibling of kind X exists"), users are forced to upgrade to an `Analyzer`.

**Recommendation:** Consider a `CheckContext` parameter that includes the tree and document, similar to how `AnalysisContext` works for analyzers. This would bridge the gap between simple pattern matching and full imperative analysis.

### 5. JSON Expectations Are Check-Only

The `diagnostic_expectations.json` schema supports `checks` with full configuration but `analyzers` only by reference to built-in types (like `"missing-nodes"`). Custom analyzer logic cannot be expressed in JSON, breaking the "edit JSON, not code" workflow for analyzer-based scenarios.

**Recommendation:** For the common case, expand the built-in analyzer registry with more types (e.g., `"duplicate-keys"`, `"deep-nesting"`). For custom analyzers, accept that Go code is required and document the pattern clearly.

### 6. Test Setup Boilerplate

The `setup()` function, `publishCollector`, and `waitFor` pattern from `treesitter/engine_test.go` are in the `treesitter_test` package and not exported. External consumers building LSP servers with gossip must recreate this infrastructure to unit-test their checks and analyzers.

**Recommendation:** Export a `treesittertest` package (or fold these helpers into `gossiptest`) with:
- `SetupEngine(t, config) (*document.Store, *Manager, *DiagnosticEngine, *Collector)`
- `Collector` with typed `WaitFor`, `Latest`, and `All` methods

### 7. No Built-In Range Assertions Before This Work

Before the diagnostic range validation framework, existing tests in `engine_test.go` checked diagnostic count, severity, source, and message -- but never the range. This gap allowed the nested-ERROR duplication issue to go unnoticed through multiple development cycles.

The `AssertDiagnosticRanges` helper in `gossiptest/assert.go` now provides sorted, positional comparison with clear diffs. All new diagnostic tests should use it.

### 8. time.Sleep in Tests

Several engine tests use `time.Sleep(50 * time.Millisecond)` for synchronization after edits instead of the `publishCollector.waitFor` method. This is fragile and can cause flaky failures in CI.

**Recommendation:** Migrate all test synchronization to `waitFor` or a channel-based signal. The `publishCollector.waitFor` pattern already exists and works reliably -- it should be the only synchronization mechanism used.
