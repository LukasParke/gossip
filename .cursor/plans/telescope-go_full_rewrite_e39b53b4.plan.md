---
name: Telescope-Go Full Rewrite
overview: Complete rewrite of Telescope as a single Go binary (LSP server + CLI) using the gossip framework with native tree-sitter integration. OpenAPI model and index built directly on tree-sitter primitives — no libopenapi runtime dependency. Rules implemented as gossip Checks and Analyzers. Vacuum-compatible ruleset loading with dramatically better DX.
todos:
  - id: foundation
    content: "Phase 1: Foundation — go.mod, openapi/ package (model types, tree-sitter YAML/JSON queries, parser, incremental index, $ref resolver)"
    status: pending
  - id: rule-engine
    content: "Phase 2: Rule engine — rule registry, rule metadata types, severity/category model, inline ignore (x-lint-ignore), shared lint runner for LSP and CLI"
    status: pending
  - id: syntactic-checks
    content: "Phase 3a: Syntactic Checks — YAML errors, duplicate keys, ASCII, trailing whitespace, structural OpenAPI checks (pattern-based via gossip Check API)"
    status: pending
  - id: semantic-analyzers
    content: "Phase 3b: Semantic Analyzers — all OpenAPI rules (~50), OWASP rules (~15), Telescope-specific rules (~15) implemented as gossip Analyzers using the OpenAPI index"
    status: pending
  - id: rulesets-config
    content: "Phase 4: Rulesets + Config — .telescope.yaml, built-in rulesets (recommended/all/owasp/strict), YAML ruleset loading, extends resolution, severity overrides"
    status: pending
  - id: lsp-server
    content: "Phase 5: LSP server — gossip server with tree-sitter, DiagnosticEngine integration, document state cache, all handlers (hover, completion, definition, references, code actions, symbols, etc.)"
    status: pending
  - id: cli
    content: "Phase 6: CLI — cobra root, lint command (shared runner), ci command (diff-aware), serve command, output formatters (text, JSON, SARIF, GitHub)"
    status: pending
  - id: plugin-system
    content: "Phase 7: Plugin system — plugin interface (Go Checks/Analyzers), YAML declarative rules, plugin manager"
    status: pending
  - id: testing
    content: "Phase 8: Testing — unit tests, integration tests with real specs, LSP protocol tests via gossiptest, CLI golden tests, benchmarks"
    status: pending
isProject: false
---

# Telescope-Go: Full Implementation Plan

## Architecture Overview

Single Go module at `github.com/LukasParke/telescope-go`. Single binary with subcommands: `telescope lint`, `telescope ci`, `telescope serve` (LSP). Uses `gossip` for LSP, `libopenapi` for OpenAPI parsing, custom rule engine inspired by vacuum's motor.

**Key design decisions:**

- **No tree-sitter** for OpenAPI files — libopenapi provides YAML/JSON AST with line/column via `*yaml.Node`
- **No vacuum dependency** — we build our own engine reusing vacuum's rule logic, with dramatically better DX
- **Spectral-compatible rulesets** — can load vacuum/spectral YAML ruleset files
- **Go-native rule API** — typed visitor pattern for Go-defined rules (compile-time safe)
- **Plugin system** — Go interfaces compiled in, plus YAML-defined declarative rules

---

## Module Structure

```
telescope-go/
├── main.go                       # Entry point → cli.Execute()
├── engine/                       # Core engine (public for plugins)
│   ├── model.go                  # Rule, RuleAction, RuleCategory, Severity
│   ├── motor.go                  # ApplyRules() — vacuum-inspired applicator
│   ├── context.go                # RuleFunctionContext (spec, index, document, dr)
│   ├── result.go                 # RuleFunctionResult, ResultSet
│   ├── function.go               # RuleFunction interface + FunctionRegistry
│   └── ignore.go                 # Inline x-lint-ignore support
├── functions/                    # Built-in functions (public, reusable by plugins)
│   ├── core/                     # 12 core functions
│   │   ├── truthy.go             # Truthy / Falsy
│   │   ├── pattern.go            # Regex match / notMatch
│   │   ├── casing.go             # camelCase, PascalCase, kebab-case, etc.
│   │   ├── length.go             # Min/max length
│   │   ├── alphabetical.go       # Sorted keys
│   │   ├── enumeration.go        # Value in allowed set
│   │   ├── xor.go                # Exactly one of N properties
│   │   ├── defined.go            # Field exists / undefined
│   │   ├── schema.go             # JSON Schema validation
│   │   └── register.go           # RegisterCoreFunctions()
│   └── openapi/                  # 50+ OpenAPI functions
│       ├── operations.go         # operationId, unique, tags, security, params
│       ├── paths.go              # pathParam, kebabCase, noVerbs, duplicates
│       ├── info.go               # description, contact, license, SPDX
│       ├── schemas.go            # typedEnum, examples, unnecessaryCombinator
│       ├── security.go           # securityDefined, apiKeyLocation
│       ├── descriptions.go       # descriptions present, no duplicates
│       ├── tags.go               # tagDefined, tagDescription
│       ├── servers.go            # serversDefined, httpsUrls
│       ├── components.go         # componentDescriptions, naming
│       ├── owasp.go              # All 15 OWASP functions
│       └── register.go           # RegisterOpenAPIFunctions()
├── rules/                        # Rule definitions (Go constants → Rule structs)
│   ├── openapi.go                # ~50 OpenAPI recommended rules
│   ├── openapi_extended.go       # Additional rules from Telescope + new
│   ├── owasp.go                  # ~15 OWASP rules
│   └── all.go                    # AllRules(), RecommendedRules(), OWASPRules()
├── rulesets/                     # Ruleset system
│   ├── model.go                  # RuleSet, RuleDefinition types
│   ├── builtin.go                # Built-in rulesets (recommended, all, owasp)
│   ├── loader.go                 # Load from YAML file, URL, or embed
│   ├── resolver.go               # Extends resolution (spectral-compatible)
│   └── merge.go                  # Merge rulesets, override severity
├── lsp/                          # LSP server (gossip-based)
│   ├── server.go                 # NewServer() using gossip.NewServer
│   ├── cache.go                  # DocState: parsed doc + libopenapi model + index
│   ├── scheduler.go              # Async diagnostic scheduling with debounce
│   ├── diagnostics.go            # Convert RuleFunctionResult → LSP Diagnostic
│   ├── hover.go                  # $ref preview, schema info, rule docs
│   ├── completion.go             # $ref paths, status codes, media types
│   ├── definition.go             # Go-to-definition for $refs
│   ├── references.go             # Find all references to component
│   ├── code_actions.go           # Quick fixes, suppress rule, see docs
│   ├── symbols.go                # Document symbols (paths, schemas, operations)
│   ├── code_lens.go              # Reference counts, endpoint summaries
│   ├── document_links.go         # Clickable $ref links
│   ├── rename.go                 # Rename operationId, components
│   ├── inlay_hints.go            # Type hints, required markers
│   └── semantic_tokens.go        # OpenAPI syntax highlighting
├── cli/                          # CLI commands
│   ├── root.go                   # Cobra root command
│   ├── lint.go                   # lint: validate files, output diagnostics
│   ├── ci.go                     # ci: diff-aware, PR comments, gating
│   ├── serve.go                  # serve: start LSP server
│   ├── output.go                 # Output formatters (text, JSON, SARIF, GitHub)
│   └── report.go                 # Markdown/JSON report generation
├── config/                       # Configuration
│   ├── config.go                 # Config struct, defaults
│   ├── loader.go                 # Find + load .telescope.yaml
│   └── schema.go                 # Config validation
├── plugin/                       # Plugin system
│   ├── types.go                  # Plugin interface, RuleProvider
│   ├── manager.go                # Load + register plugins
│   └── yaml_plugin.go            # YAML-defined custom rules
├── testutil/                     # Test helpers
│   ├── fixtures.go               # Embedded test OpenAPI specs
│   └── helpers.go                # Assert functions
├── go.mod
├── go.sum
└── README.md
```

---

## Phase 1: Foundation — Core Types and Engine

### `engine/model.go` — Core types

```go
type Severity string
const (
    SeverityError   Severity = "error"
    SeverityWarn    Severity = "warn"
    SeverityInfo    Severity = "info"
    SeverityHint    Severity = "hint"
)

type RuleCategory struct {
    ID, Name, Description string
}

type Rule struct {
    ID          string
    Description string
    Message     string
    Severity    Severity
    Given       []string              // JSONPath expressions
    Then        []RuleAction
    Formats     []string              // "oas2", "oas3", "oas3.1"
    Resolved    bool
    Recommended bool
    Type        string                // "style" | "validation"
    Category    *RuleCategory
    HowToFix    string
    DocURL      string                // link to rule documentation
}

type RuleAction struct {
    Field           string
    Function        string
    FunctionOptions map[string]interface{}
}
```

### `engine/function.go` — Function interface

```go
type RuleFunction interface {
    RunRule(nodes []*yaml.Node, ctx RuleFunctionContext) []RuleFunctionResult
    GetSchema() RuleFunctionSchema
}

type FunctionRegistry struct { /* map[string]RuleFunction */ }
func NewFunctionRegistry() *FunctionRegistry
func (r *FunctionRegistry) Register(name string, fn RuleFunction)
func (r *FunctionRegistry) Get(name string) (RuleFunction, bool)
```

### `engine/context.go` — Execution context

```go
type RuleFunctionContext struct {
    Rule        *Rule
    RuleAction  *RuleAction
    Given       string
    Options     map[string]interface{}
    Document    libopenapi.Document
    Index       *index.SpecIndex
    SpecInfo    *datamodel.SpecInfo
    DrDocument  *drModel.DrDocument  // pb33f doctor model
    Logger      *slog.Logger
}
```

### `engine/result.go` — Results

```go
type RuleFunctionResult struct {
    Message    string
    Range      Range          // StartLine, StartCol, EndLine, EndCol
    Path       string         // JSONPath to the violation
    RuleID     string
    Severity   Severity
    Rule       *Rule
    StartNode  *yaml.Node
    EndNode    *yaml.Node
    HowToFix   string
}

type ResultSet struct {
    Results    []*RuleFunctionResult
    // Filtering, sorting, grouping methods
}
```

### `engine/motor.go` — Rule applicator

Vacuum-inspired but cleaner:

```go
type Execution struct {
    Spec          []byte
    RuleSet       *rulesets.RuleSet
    Functions     *FunctionRegistry
    CustomFuncs   map[string]RuleFunction
    Document      libopenapi.Document
    DrDocument    *drModel.DrDocument
    Logger        *slog.Logger
    SkipRules     []string
    Timeout       time.Duration
}

func ApplyRules(exec *Execution) (*ResultSet, error)
```

Flow: parse spec → build models → for each rule → resolve JSONPath nodes → run function → collect results.

---

## Phase 2: Functions — Core + OpenAPI + OWASP

### Core functions (12) — `functions/core/`


| Function       | Purpose                                                   |
| -------------- | --------------------------------------------------------- |
| `truthy`       | Value is truthy                                           |
| `falsy`        | Value is falsy                                            |
| `defined`      | Field exists                                              |
| `undefined`    | Field absent                                              |
| `pattern`      | Regex match/notMatch                                      |
| `length`       | Min/max length check                                      |
| `casing`       | camelCase, PascalCase, kebab-case, snake_case, MACRO_CASE |
| `alphabetical` | Keys sorted alphabetically                                |
| `enumeration`  | Value in allowed set                                      |
| `xor`          | Exactly one of N properties set                           |
| `schema`       | JSON Schema validation                                    |
| `blank`        | No-op (for resolved-only rules)                           |


### OpenAPI functions (~50) — `functions/openapi/`

Port from vacuum with improvements. All existing vacuum OpenAPI functions plus Telescope-specific additions:

- `unresolvedRef` — flag unresolved $refs
- `schemaNameCapital` — schema names start uppercase
- `exampleNameCapital` — example names start uppercase
- `allofMixedTypes` — allOf shouldn't mix types
- `allofStructure` — allOf structural validation
- `discriminatorMapping` — discriminator consistency
- `requestBodyContent` — request body has content
- `typeRequired` — type field present on schemas
- `noUnknownFormats` — only known string formats
- `idUniqueInPath` — no duplicate IDs in same path
- `casingConsistency` — consistent path casing
- `pathParamValuesNoGenericSyntax` — no generic param names
- `ascii` — ASCII-only content check

### OWASP functions (15) — `functions/openapi/owasp.go`

All 15 vacuum OWASP rules ported directly.

---

## Phase 3: Rules and Rulesets

### `rules/` — Rule definitions as Go constants

Each rule is a `engine.Rule` struct with full metadata: ID, description, message, given paths, then actions, severity, category, howToFix, docURL.

Organized into:

- `openapi.go` — ~50 recommended OpenAPI rules
- `openapi_extended.go` — additional rules (telescope-specific + new)
- `owasp.go` — 15 OWASP rules

### `rulesets/` — Spectral-compatible ruleset loading

**Ruleset YAML format** (Spectral/vacuum compatible):

```yaml
extends: [[telescope:recommended, all]]
rules:
  operation-operationId:
    severity: error
  my-custom-rule:
    description: "Custom check"
    given: "$.paths[*]"
    then:
      function: pattern
      functionOptions:
        match: "^/api/v"
```

**Built-in rulesets:**

- `telescope:recommended` — curated set of ~35 most important rules
- `telescope:all` — all OpenAPI rules
- `telescope:owasp` — all OWASP rules
- `telescope:strict` — recommended + owasp

**Extends resolution:** local files, remote URLs, built-in names.

---

## Phase 4: Configuration

### `.telescope.yaml` format

```yaml
extends: telescope:recommended
rules:
  operation-operationId:
    severity: error
  no-trailing-slash: off

plugins:
  - ./my-rules.yaml
  - github.com/org/telescope-plugin-sailpoint

include:
  - "**/*.yaml"
  - "**/*.json"
exclude:
  - "node_modules/**"
  - "vendor/**"

output:
  format: text          # text | json | sarif | github
  color: auto

lsp:
  debounce: 300ms
  maxFileSize: 5MB
```

### Config resolution

1. CLI flags (highest priority)
2. `.telescope.yaml` in workspace root
3. `.telescope/config.yaml` (alternative location)
4. Built-in defaults

---

## Phase 5: LSP Server (gossip-based)

### `lsp/server.go`

```go
func NewServer(cfg *config.Config) *gossip.Server {
    s := gossip.NewServer("telescope", version,
        gossip.WithMiddleware(middleware.Logging(logger), middleware.Recovery()),
    )
    cache := NewDocumentCache()
    sched := NewScheduler(cache, cfg)

    s.OnDidOpen(sched.HandleOpen)
    s.OnDidChange(sched.HandleChange)
    s.OnDidClose(sched.HandleClose)
    s.OnHover(NewHoverHandler(cache))
    s.OnCompletion(NewCompletionHandler(cache))
    s.OnDefinition(NewDefinitionHandler(cache))
    s.OnReferences(NewReferencesHandler(cache))
    s.OnCodeAction(NewCodeActionHandler(cache))
    s.OnDocumentSymbol(NewSymbolHandler(cache))
    // ... all handlers
    return s
}
```

### `lsp/cache.go` — Document state cache

Per-document state:

```go
type DocState struct {
    URI         protocol.DocumentURI
    Version     int32
    Text        string
    Document    libopenapi.Document     // parsed spec
    Model       *libopenapi.DocumentModel[v3high.Document]
    Index       *index.SpecIndex
    DrDocument  *drModel.DrDocument
    Results     *engine.ResultSet       // latest lint results
    ParseErr    error
}
```

Reparse on change with debounce. Cache libopenapi models for hover/completion/definition.

### `lsp/scheduler.go` — Diagnostic scheduling

- Debounced (configurable, default 300ms)
- Cancellable (new edit cancels in-flight lint)
- Per-document goroutines
- Publishes via `ctx.Client.PublishDiagnostics()`

### LSP handlers


| Handler              | Features                                                                      |
| -------------------- | ----------------------------------------------------------------------------- |
| `hover.go`           | $ref target preview, schema type info, rule documentation on diagnostic hover |
| `completion.go`      | $ref component paths, HTTP status codes, media types, common fields           |
| `definition.go`      | Go-to-definition for $ref targets (local and cross-file)                      |
| `references.go`      | Find all references to a component/operationId                                |
| `code_actions.go`    | Quick fix suggestions, suppress rule inline (x-lint-ignore), open rule docs   |
| `symbols.go`         | Document outline: paths → operations → parameters → schemas                   |
| `code_lens.go`       | Reference counts on components, endpoint method summaries                     |
| `document_links.go`  | Clickable $ref links                                                          |
| `rename.go`          | Rename operationId and component names across files                           |
| `inlay_hints.go`     | Required markers, resolved type names                                         |
| `semantic_tokens.go` | OpenAPI-aware syntax token types                                              |


---

## Phase 6: CLI

### Commands

`**telescope lint**`

```
telescope lint [files/dirs...] [flags]
  --config, -c     Config file path
  --ruleset, -r    Ruleset file/URL
  --format, -f     Output: text, json, sarif, github (default: text)
  --severity, -s   Min severity: error, warn, info, hint
  --fail-on        Exit code 1 on: error, warn (default: error)
  --fix            Apply auto-fixes
  --no-color       Disable color output
```

`**telescope ci**`

```
telescope ci [flags]
  --diff-base      Git ref for base (e.g. main)
  --diff-head      Git ref for head (e.g. HEAD)
  --report-md      Write markdown report to file
  --report-json    Write JSON report to file
  --comment-pr     Post comment to GitHub PR (requires GITHUB_TOKEN)
  --fail-on        Quality gate severity
```

`**telescope serve**`

```
telescope serve [flags]
  --stdio          Use stdio transport (default)
  --tcp=ADDR       Use TCP transport
  --socket=PATH    Use Unix socket
```

Uses `gossip.Serve(server, gossip.FromArgs())` for transport selection.

---

## Phase 7: Plugin System

### Plugin interface

```go
type Plugin interface {
    Name() string
    Version() string
    Rules() []engine.Rule
    Functions() map[string]engine.RuleFunction
}
```

### Plugin types

1. **Go plugins** — implement `Plugin` interface, compiled into binary or via `plugin.Open`
2. **YAML ruleset plugins** — declarative rules using existing functions
3. **Remote rulesets** — loaded from URL at startup

### Plugin loading

```yaml
# .telescope.yaml
plugins:
  - path: ./rulesets/sailpoint.yaml        # local YAML ruleset
  - url: https://example.com/rules.yaml    # remote ruleset
  - module: github.com/org/telescope-plugin # Go module (future)
```

---

## Phase 8: Testing Strategy

- **Unit tests** for every function, rule, and engine component
- **Integration tests** with real OpenAPI specs (petstore, various real-world APIs)
- **LSP tests** using `gossiptest.NewClient` for full protocol testing
- **CLI tests** using exec + golden files
- **Benchmark tests** for parsing and linting large specs

---

## Dependencies

```
github.com/LukasParke/gossip          # LSP framework
github.com/pb33f/libopenapi           # OpenAPI parsing
github.com/pb33f/doctor               # Doctor model (typed wrappers)
github.com/spf13/cobra                # CLI
github.com/santhosh-tekuri/jsonschema # JSON Schema validation
github.com/pb33f/jsonpath             # JSONPath for rule given paths
gopkg.in/yaml.v3                      # YAML types (yaml.Node)
```

---

## Rule Count Summary


| Category                 | Count   | Source                       |
| ------------------------ | ------- | ---------------------------- |
| Core functions           | 12      | Vacuum core                  |
| OpenAPI recommended      | ~35     | Vacuum + Telescope merged    |
| OpenAPI extended         | ~15     | Telescope-specific + new     |
| OWASP                    | 15      | Vacuum OWASP                 |
| **Total built-in rules** | **~65** |                              |
| **Total functions**      | **~77** | 12 core + ~65 rule functions |


