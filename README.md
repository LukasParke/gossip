# Gossip

A best-in-class Go framework for building Language Server Protocol (LSP) servers.

Gossip gives you everything you need to build a production-quality LSP server in Go: code-generated protocol types, composable middleware, built-in document management, native tree-sitter integration with incremental diagnostics, config hot-reload, multi-root workspace support, and full cross-editor transport support.

[![CI](https://github.com/gossip-lsp/gossip/actions/workflows/ci.yml/badge.svg)](https://github.com/gossip-lsp/gossip/actions/workflows/ci.yml)

## Features

- **Zero-to-server in 15 lines** — functional handler registration, auto-detected capabilities
- **LSP 3.18 types** — code-generated from the official meta model
- **Built-in document store** — thread-safe, auto-wired to didOpen/didChange/didClose, with UTF-16 position handling
- **Native tree-sitter** — parser-per-document lifecycle, automatic incremental re-parsing, query API
- **Incremental diagnostics** — declarative checks and imperative analyzers with smart caching and merging
- **Multi-root workspaces** — native support for `workspace/didChangeWorkspaceFolders` with per-folder config resolution
- **Config hot-reload** — TOML config files with fsnotify watching, atomic swap, change callbacks
- **Composable middleware** — logging, panic recovery, tracing, telemetry (same pattern as `net/http`)
- **6 transports** — stdio, TCP, Unix sockets, named pipes, WebSocket, Node.js IPC
- **CLI flag parsing** — single binary works with every editor via `--stdio`, `--tcp`, `--socket`, etc.
- **First-class testing** — in-memory client, test harness, LSP-specific assertions
- **Extensible** — custom method handlers, break-glass accessors, bring your own tree-sitter grammars

## Quick Start

```go
package main

import (
    "log"

    "github.com/gossip-lsp/gossip"
    "github.com/gossip-lsp/gossip/protocol"
)

func main() {
    s := gossip.NewServer("my-lang", "0.1.0")

    s.OnHover(func(ctx *gossip.Context, p *protocol.HoverParams) (*protocol.Hover, error) {
        doc := ctx.Documents.Get(p.TextDocument.URI)
        word := doc.WordAt(p.Position)
        return &protocol.Hover{
            Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "**" + word + "**"},
        }, nil
    })

    if err := gossip.Serve(s, gossip.WithStdio()); err != nil {
        log.Fatal(err)
    }
}
```

## Architecture

```
gossip/
├── jsonrpc/        JSON-RPC 2.0 codec and bidirectional connection
├── transport/      stdio, TCP, Unix socket, named pipe, WebSocket, Node.js IPC
├── protocol/       LSP 3.18 types (handwritten + code-generated)
├── document/       Thread-safe document store with position utilities
├── treesitter/     Native tree-sitter integration with incremental diagnostics
├── config/         TOML config system with hot-reload
├── middleware/     Composable middleware (logging, recovery, tracing, telemetry)
├── gossiptest/     Testing utilities
└── examples/       Minimal, configurable, and full-featured example servers
```

## Handler Registration

Register handlers for the LSP methods you want to support. Gossip automatically detects capabilities from which handlers are registered — no manual wiring needed.

```go
s := gossip.NewServer("my-lang", "0.1.0")

// Language features
s.OnHover(myHoverHandler)
s.OnCompletion(myCompletionHandler)
s.OnDefinition(myDefinitionHandler)
s.OnDeclaration(myDeclarationHandler)
s.OnTypeDefinition(myTypeDefinitionHandler)
s.OnImplementation(myImplementationHandler)
s.OnReferences(myReferencesHandler)
s.OnDocumentSymbol(mySymbolHandler)
s.OnCodeAction(myCodeActionHandler)
s.OnCodeLens(myCodeLensHandler)
s.OnFormatting(myFormattingHandler)
s.OnRangeFormatting(myRangeFormattingHandler)
s.OnRename(myRenameHandler)
s.OnPrepareRename(myPrepareRenameHandler)
s.OnSignatureHelp(mySignatureHelpHandler)
s.OnDocumentHighlight(myDocumentHighlightHandler)
s.OnDocumentLink(myDocumentLinkHandler)
s.OnFoldingRange(myFoldingRangeHandler)
s.OnSelectionRange(mySelectionRangeHandler)
s.OnInlayHint(myInlayHintHandler)
s.OnSemanticTokens(mySemanticTokensHandler)
s.OnWorkspaceSymbol(myWorkspaceSymbolHandler)
s.OnExecuteCommand(myExecuteCommandHandler)

// Text document sync notifications
s.OnDidOpen(myDidOpenHandler)
s.OnDidChange(myDidChangeHandler)
s.OnDidClose(myDidCloseHandler)
s.OnDidSave(myDidSaveHandler)

// Workspace notifications
s.OnDidChangeWatchedFiles(myWatchedFilesHandler)
s.OnDidChangeWorkspaceFolders(myFoldersHandler)
```

## Tree-sitter Integration

Native tree-sitter support gives you incremental parsing for free. Trees are automatically maintained and re-parsed as documents change. Bring your own grammar packages — any tree-sitter grammar with Go bindings works.

```go
import (
    "github.com/gossip-lsp/gossip"
    "github.com/gossip-lsp/gossip/treesitter"
    tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

s := gossip.NewServer("my-lang", "0.1.0",
    gossip.WithTreeSitter(treesitter.Config{
        Languages: map[string]*tree_sitter.Language{
            ".go": tree_sitter_go.GetLanguage(),
        },
    }),
)

s.OnHover(func(ctx *gossip.Context, p *protocol.HoverParams) (*protocol.Hover, error) {
    doc := ctx.Documents.Get(p.TextDocument.URI)
    tree := gossip.TreeFor(doc)
    if tree == nil {
        return nil, nil
    }
    node := tree.NodeAt(p.Position)
    // Use node.Kind(), tree.NodeText(node), tree.QueryCaptures(), etc.
})
```

### Flexible Language Matching

Use `LanguageMatcher` to match files by extension, filename, glob pattern, or LSP language ID:

```go
gossip.WithTreeSitter(treesitter.Config{
    Matchers: []treesitter.LanguageMatcher{
        {
            Language:   yamlLang,
            Extensions: []string{".yml", ".yaml"},
            Filenames:  []string{".gitlab-ci.yml"},
            LanguageID: "yaml",
        },
        {
            Language:   dockerLang,
            Filenames:  []string{"Dockerfile", "Containerfile"},
            Pattern:    "Dockerfile.*",
        },
    },
})
```

Matching priority: exact filename > LSP languageID > glob pattern > file extension.

## Incremental Diagnostics

Gossip's diagnostic engine leverages tree-sitter's incremental parsing to run only the checks affected by each edit. Two APIs are provided:

### Declarative Checks

Pattern-based rules that the framework automatically scopes and caches:

```go
s.Check(treesitter.Check{
    ID:       "no-todo",
    Pattern:  `(comment) @c`,
    Severity: protocol.DiagnosticSeverityWarning,
    Run: func(node *tree_sitter.Node, text string) *string {
        if strings.Contains(text, "TODO") {
            msg := "Resolve TODO comment"
            return &msg
        }
        return nil
    },
})
```

### Imperative Analyzers

Full-control analyzers with lifecycle hooks, change-aware scoping, and previous-result merging:

```go
s.Analyze(treesitter.Analyzer{
    ID:            "complexity",
    InterestKinds: []string{"function_declaration", "method_declaration"},
    Run: func(actx *treesitter.AnalysisContext) []protocol.Diagnostic {
        // actx.Scope tells you what changed: Full, Incremental, or Skip
        // actx.AffectedNodes() returns only the nodes in changed ranges
        // actx.MergePrevious(fresh) intelligently merges with cached results
        // ...
    },
})
```

The `DiagnosticEngine` orchestrates everything: it determines which checks/analyzers are affected by each `TreeDiff`, runs them with scoped query cursors, caches results per-check, and publishes merged diagnostics to the client automatically.

## Multi-Root Workspace Support

Gossip natively supports multi-root workspaces. The server automatically tracks workspace folders and handles `workspace/didChangeWorkspaceFolders` notifications from the client.

```go
s.OnHover(func(ctx *gossip.Context, p *protocol.HoverParams) (*protocol.Hover, error) {
    // Determine which workspace folder owns this document
    folder := ctx.FolderFor(p.TextDocument.URI)

    // Access all workspace folders
    folders := ctx.WorkspaceFolders()

    // Access the primary workspace root
    root := ctx.WorkspaceRoot()
    // ...
})
```

For per-folder configuration, use `ctx.FolderFor(uri)` to resolve which workspace root a file belongs to, then load the appropriate config.

## Config with Hot-Reload

Define your config as a Go struct. Gossip handles TOML parsing, file watching, and atomic reload.

```go
type MyConfig struct {
    MaxCompletions int      `toml:"max_completions"`
    LintRules      []string `toml:"lint_rules"`
}

s := gossip.NewServer("my-lang", "0.1.0",
    gossip.WithConfig[MyConfig](".my-lang.toml", MyConfig{
        MaxCompletions: 50,
    }),
)

gossip.OnConfigChange(s, func(ctx *gossip.Context, old, new_ *MyConfig) {
    ctx.Client.LogMessage(ctx, protocol.Info, "Config reloaded")
})

s.OnCompletion(func(ctx *gossip.Context, p *protocol.CompletionParams) (*protocol.CompletionList, error) {
    cfg := gossip.Config[MyConfig](ctx)
    // use cfg.MaxCompletions
})
```

## Custom Method Handlers

Register handlers for custom or non-standard LSP methods using raw JSON params:

```go
// Custom request (expects a response)
s.HandleRequest("$/myCustomMethod", func(ctx *gossip.Context, params json.RawMessage) (interface{}, error) {
    var p MyCustomParams
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, err
    }
    return MyCustomResult{OK: true}, nil
})

// Custom notification (fire-and-forget)
s.HandleNotification("$/myCustomNotification", func(ctx *gossip.Context, params json.RawMessage) {
    // handle notification
})
```

## Client Proxy

The `ClientProxy` provides server-initiated communication back to the client:

```go
s.OnCodeAction(func(ctx *gossip.Context, p *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
    // Send messages to the client
    ctx.Client.ShowMessage(ctx, protocol.Info, "Processing...")

    // Request workspace edits
    ctx.Client.ApplyEdit(ctx, &protocol.ApplyWorkspaceEditParams{
        Label: "My Edit",
        Edit:  workspaceEdit,
    })

    // Read client configuration
    ctx.Client.Configuration(ctx, configItems)

    // Register/unregister dynamic capabilities
    ctx.Client.RegisterCapability(ctx, registrations)

    // Trigger refreshes
    ctx.Client.RefreshDiagnostics(ctx)
    ctx.Client.RefreshInlayHints(ctx)
    ctx.Client.RefreshSemanticTokens(ctx)
    // ...
})
```

## Break-Glass Accessors

When you need direct access to framework internals:

```go
// Access the tree-sitter manager (parsers, trees, registry)
tsManager := s.TreeSitter()

// Access the diagnostic engine (checks, analyzers, cache)
engine := s.DiagnosticEngine()

// Access the document store directly
docs := s.Documents()

// Access the underlying JSON-RPC connection
conn := s.Conn()

// Access the server's logger
logger := s.Logger()
```

These are also available from handler context:

```go
s.OnHover(func(ctx *gossip.Context, p *protocol.HoverParams) (*protocol.Hover, error) {
    logger := ctx.Logger()
    server := ctx.Server()
    caps := ctx.ClientCapabilities()
    initOpts := ctx.InitOptions()
    // ...
})
```

## Middleware

Gossip uses the same middleware pattern as Go's `net/http`. Middleware applies to both requests and notifications.

```go
s := gossip.NewServer("my-lang", "0.1.0",
    gossip.WithMiddleware(
        middleware.Logging(slog.Default()),
        middleware.Recovery(),
        middleware.Tracing(),
        middleware.Telemetry(metrics),
    ),
)
```

Custom middleware:

```go
func RateLimit(rps int) middleware.Middleware {
    return func(next middleware.Handler) middleware.Handler {
        return func(ctx context.Context, method string, params jsonrpc.RawMessage) (interface{}, error) {
            // your rate limiting logic
            return next(ctx, method, params)
        }
    }
}
```

## Cross-Editor Transport

A single binary supports every editor via CLI flags:

```bash
my-lang-lsp                          # default: stdio
my-lang-lsp --stdio                  # explicit stdio
my-lang-lsp --tcp :9257              # TCP listener
my-lang-lsp --socket /tmp/my.sock    # Unix domain socket
my-lang-lsp --pipe \\.\pipe\my-lang  # Windows named pipe
my-lang-lsp --ws :9258               # WebSocket
my-lang-lsp --node-ipc               # Node.js IPC (VS Code)
```

Use `gossip.FromArgs()` to enable automatic transport selection:

```go
gossip.Serve(s, gossip.FromArgs())
```

Or select programmatically:

```go
gossip.Serve(s, gossip.WithStdio())
gossip.Serve(s, gossip.WithTCP(":9257"))
gossip.Serve(s, gossip.WithSocket("/tmp/my.sock"))
gossip.Serve(s, gossip.WithWebSocket(":9258"))
gossip.Serve(s, gossip.WithNodeIPC())
```

## Testing

The `gossiptest` package provides an in-memory LSP client for testing:

```go
func TestHover(t *testing.T) {
    s := gossip.NewServer("test", "0.1.0")
    s.OnHover(myHoverHandler)

    c := gossiptest.NewClient(t, s)
    c.Open("file:///test.txt", "hello world")

    hover, err := c.Hover("file:///test.txt", gossiptest.Pos(0, 2))
    if err != nil {
        t.Fatal(err)
    }
    gossiptest.AssertHoverContains(t, hover, "hello")
}
```

Tree-sitter test helpers are also available:

```go
func TestDiagnostics(t *testing.T) {
    s := gossip.NewServer("test", "0.1.0",
        gossip.WithTreeSitter(treesitter.Config{...}),
    )
    s.Check(myCheck)

    c := gossiptest.NewClient(t, s)
    c.Open("file:///test.json", `{"key": }`)

    diags := gossiptest.WaitForDiagnostics(t, c, "file:///test.json")
    // assert on diags...

    gossiptest.ChangeIncremental(t, c, "file:///test.json", editRange, `"value"`)
}
```

## Editor Integration

### VS Code

In your extension's `activate()`:

```typescript
const serverOptions: ServerOptions = {
  command: "my-lang-lsp",
  args: ["--stdio"],
};
```

### Neovim

```lua
vim.lsp.start({
  name = "my-lang",
  cmd = { "my-lang-lsp", "--stdio" },
  -- Or connect to a running server:
  -- cmd = vim.lsp.rpc.connect("127.0.0.1", 9257),
})
```

### Emacs (Eglot)

```elisp
(add-to-list 'eglot-server-programs
             '(my-lang-mode . ("my-lang-lsp" "--stdio")))
```

### Helix

In `languages.toml`:

```toml
[[language]]
name = "my-lang"
language-servers = ["my-lang-lsp"]

[language-server.my-lang-lsp]
command = "my-lang-lsp"
args = ["--stdio"]
```

### Zed

In Zed settings:

```json
{
  "lsp": {
    "my-lang-lsp": {
      "binary": { "path": "my-lang-lsp", "arguments": ["--stdio"] }
    }
  }
}
```

## Benchmarks

Performance is tracked on every push to `main`. Benchmarks cover initial parse, incremental edits, query scoping, analyzer skip logic, merge performance, and end-to-end diagnostic cycles across Go, Python, JSON, and YAML.

View the [benchmark dashboard](https://gossip-lsp.github.io/gossip/dev/bench/) for historical trends.

On pull requests, benchmarks are compared against `main` using `benchstat` to catch performance regressions before they are merged.

## Dependencies

| Package                                 | Purpose                             |
| --------------------------------------- | ----------------------------------- |
| `github.com/tree-sitter/go-tree-sitter` | Tree-sitter Go bindings             |
| `github.com/BurntSushi/toml`            | TOML config parsing                 |
| `github.com/fsnotify/fsnotify`          | File watching for config hot-reload |
| `golang.org/x/net`                      | WebSocket transport                 |

The core framework (JSON-RPC + stdio) has zero external dependencies.

## License

MIT

This repo was 100% created by AI