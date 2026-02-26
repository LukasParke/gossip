---
name: Benchmarks and Test Specs
overview: Copy the existing telescope test specifications into telescope-go, build a typed registry with go:embed, and add comprehensive benchmarks for parsing, indexing, LSP handlers, and rule execution following the gossip framework's bench_test.go patterns.
todos:
  - id: test-specs
    content: Copy spec files from telescope/packages/test-files/openapi/ into testutil/specs/, create registry.go with typed Spec catalog using go:embed, and update testutil/helpers.go
    status: completed
  - id: openapi-bench
    content: Create openapi/bench_test.go with benchmarks for Parse, BuildIndex, Classify, ResolveRef, IncrementalReindex across the spec size matrix, plus a spec generator for scaling benchmarks
    status: completed
  - id: lsp-bench
    content: Create lsp/bench_test.go with benchmarks for all LSP handlers (hover, completion, definition, references, symbols, code actions, folding, code lens, inlay hints, semantic tokens) against small/medium/large specs
    status: completed
  - id: rules-bench
    content: Create rules/bench_test.go with benchmarks for syntactic checks and semantic analyzers against specs of varying sizes
    status: completed
  - id: integration
    content: Verify all benchmarks compile and run, ensure specs are properly embedded and the registry works end-to-end
    status: completed
isProject: false
---

# Telescope-Go Benchmarks and Test Specs

## Phase 1: Test Specifications Registry

Copy the existing test specs from the original telescope TypeScript repo at `telescope/packages/test-files/openapi/` into `telescope-go/testutil/specs/`. Build a typed Go registry with `//go:embed` to serve them in tests and benchmarks.

### Source Specs (from `telescope/packages/test-files/openapi/`)

The existing collection provides excellent coverage across sizes, versions, formats, and error conditions:

**Small (~7-35 lines) -- for fast iteration and unit tests:**

- `api-minimal.yaml` -- Minimal valid OpenAPI 3.1 (7 lines)
- `test-unique-operation-ids.yaml` -- Two unique operations (16 lines)
- `test-duplicate-operation-ids.yaml` -- Duplicate operationId error case (25 lines)
- `test-root-valid.yaml` -- Valid root with tags (17 lines)
- `test-root-errors.yaml` -- Missing info section (10 lines)
- `missing-path-parameters.yaml` -- Missing path params (15 lines)

**Medium (~68-210 lines) -- realistic single-service APIs:**

- `api-standalone.yaml` -- Self-contained spec with schemas, security, params (90 lines)
- `openapi-3.0.yaml` -- OpenAPI 3.0 petstore (52 lines)
- `openapi-3.1.yaml` -- OpenAPI 3.1 petstore (48 lines)
- `openapi-3.2.yaml` -- OpenAPI 3.2 with Error schema, CRUD ops (115 lines)
- `OpenAPI-example.yaml` -- Customer API with rich markdown descriptions (209 lines)
- `test-warnings.yaml` -- Warning-level test cases (68 lines)
- `test-ascii-errors.yaml` -- Non-ASCII character test cases (22 lines)
- `custom-openapi-valid.yaml` -- Valid custom rules spec (23 lines)
- `custom-openapi-invalid.yaml` -- Invalid custom rules spec (18 lines)

**Large (~568-939 lines) -- comprehensive real-world APIs:**

- `test-valid.yaml` -- Comprehensive valid spec with many operations, schemas (716 lines)
- `test-errors.yaml` -- Comprehensive error cases covering many rule violations (939 lines)

**Extra-Large (~17K lines) -- stress test:**

- `Plex-API.yaml` -- Real-world Plex Media Server API (17,044 lines)

**JSON format:**

- `api-v3.json` -- OpenAPI 3.1 in JSON with markdown descriptions

**Multi-file references (for future cross-file support):**

- `api-v1.yaml` -- Spec with external `$ref` to v1/ components
- `api-v2.yaml` -- Spec with invalid external refs
- `test-multi-file.yaml` -- Ref cycle and cross-file tests

Copy all standalone (non-external-ref) YAML/JSON files into `testutil/specs/`. Multi-file specs that reference `./v1/`, `./v2/`, `./v3/` subdirectories are copied for completeness but won't resolve their external refs in single-file mode.

### Registry (`[testutil/specs/registry.go](testutil/specs/registry.go)`)

```go
package specs

type Spec struct {
    Name    string
    Content []byte
    Format  openapi.FileFormat
    Version openapi.Version
    Lines   int
    Size    SpecSize  // Small, Medium, Large, XLarge
    Tags    []string  // "valid", "invalid", "errors", "warnings", "json", "multi-file"
}

type SpecSize int
const (
    Small SpecSize = iota
    Medium
    Large
    XLarge
)

func All() []Spec
func ByName(name string) Spec
func BySize(size SpecSize) []Spec
func ByTag(tag string) []Spec
func YAML() []Spec
func JSON() []Spec
func BenchmarkSpecs() []Spec  // returns one small, one medium, one large, one xlarge
```

Uses `//go:embed *.yaml *.json` to embed all spec files. Each `Spec` carries metadata. `BenchmarkSpecs()` returns a curated subset for benchmarks:

- Small: `api-standalone.yaml` (90 lines)
- Medium: `OpenAPI-example.yaml` (209 lines) or `openapi-3.2.yaml` (115 lines)
- Large: `test-valid.yaml` (716 lines)
- XLarge: `Plex-API.yaml` (17,044 lines)

### Update existing code

- `[testutil/helpers.go](testutil/helpers.go)`: Keep `PetstoreYAML` pointing at the existing fixture for backward compatibility; add `specs.ByName("petstore")` as an alias

## Phase 2: OpenAPI Benchmarks (`[openapi/bench_test.go](openapi/bench_test.go)`)

Following the patterns in `[gossip/treesitter/bench_test.go](/home/luke/Documents/GitHub/gossip/treesitter/bench_test.go)` -- `b.ReportAllocs()`, `b.SetBytes()`, `b.ResetTimer()` after setup, table-driven with size matrix.

- `**BenchmarkParse**` -- Parse tree-sitter tree into typed OpenAPI `Document`. Runs across `BenchmarkSpecs()` (small/medium/large/xlarge).
- `**BenchmarkBuildIndex**` -- Full `BuildIndex` call (parse + index paths/components/refs). Same matrix.
- `**BenchmarkClassify**` -- `Classify()` on each spec.
- `**BenchmarkResolveRef**` -- Resolve repeated `$ref` lookups against a pre-built index.
- `**BenchmarkIncrementalReindex**` -- Simulate an edit via `store.Change`, rebuild tree, rebuild index. Measures the cost of re-indexing after an incremental edit.

### Spec Generator (`openapi/bench_test.go`)

Also includes `genOpenAPIYAML(paths, schemasPerPath int) string` for generating synthetic specs at arbitrary sizes, used for scaling benchmarks beyond the fixed specs. Size matrix: `{10 paths, 100 paths, 500 paths}`.

## Phase 3: LSP Handler Benchmarks (`[lsp/bench_test.go](lsp/bench_test.go)`)

Pre-builds an `openapi.IndexCache` from specs, then benchmarks individual handler invocations directly (no JSON-RPC overhead):

- `**BenchmarkHover**` -- Hover on a `$ref`, schema name, and operationId
- `**BenchmarkCompletion**` -- Completion at a `$ref: ""` position
- `**BenchmarkDefinition**` -- Go-to-definition on a `$ref` target
- `**BenchmarkReferences**` -- Find all references to a schema
- `**BenchmarkDocumentSymbol**` -- Document symbols for a spec
- `**BenchmarkCodeAction**` -- Code actions for a diagnostic context
- `**BenchmarkFoldingRange**` -- Folding ranges across a spec
- `**BenchmarkCodeLens**` -- Code lenses for reference counts
- `**BenchmarkInlayHints**` -- Inlay hints for `$ref` type resolution
- `**BenchmarkSemanticTokens**` -- Semantic token encoding

Each benchmark tests against the small, medium, large, and xlarge specs from the registry.

## Phase 4: Rule Benchmarks (`[rules/bench_test.go](rules/bench_test.go)`)

Benchmarks the diagnostic rule execution pipeline:

- `**BenchmarkChecks**` -- Runs each syntactic check (`syntax-error`, `duplicate-keys`, `ascii`) against specs of varying sizes
- `**BenchmarkAnalyzers**` -- Runs each analyzer group (references, naming, documentation, structure, types, security, servers, paths, owasp, extended) against specs of varying sizes
- `**BenchmarkAllRules**` -- Full rule suite on each spec size (simulates what happens on file open)

Uses the `treesitter.AnalysisContext` directly with pre-built indexes from the specs registry, matching how the gossip `bench_test.go` does it.

## File Summary

New files:

- `testutil/specs/*.yaml` and `*.json` -- Copied from telescope test-files
- `testutil/specs/registry.go` -- Typed spec catalog with embed
- `openapi/bench_test.go` -- OpenAPI parsing/indexing benchmarks
- `lsp/bench_test.go` -- LSP handler benchmarks
- `rules/bench_test.go` -- Rule execution benchmarks

Modified files:

- `testutil/helpers.go` -- Add specs registry convenience alias

