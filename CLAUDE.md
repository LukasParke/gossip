# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gossip is a Go framework for building LSP (Language Server Protocol) servers. It provides code-generated LSP 3.18 protocol types, composable middleware, built-in document management, native tree-sitter integration with incremental diagnostics, TOML config with hot-reload, multi-root workspace support, and pluggable transports (stdio, TCP, Unix socket, WebSocket, named pipes, Node.js IPC).

## Commands

```bash
# Build
go build ./...

# Lint
go vet ./...

# Test (with race detection and coverage)
go test -race -coverprofile=coverage.out -covermode=atomic ./... -timeout 10m

# Run a single test
go test -race ./treesitter/ -run TestDiagnosticRanges -v

# Benchmarks
go test -run='^$' -bench=. -benchmem -count=1 ./... -timeout 15m

# Update diagnostic expectation snapshots after grammar/rule changes
go test ./treesitter/ -run TestDiagnosticRanges -update
```

## Architecture

### Core Packages

- **Root package (`gossip`)** — Central server (`server.go`), handler registration (`handler.go`), request context (`context.go`), client proxy (`client.go`), capability auto-detection (`capabilities.go`), server entry point (`serve.go`)
- **`jsonrpc/`** — JSON-RPC 2.0 bidirectional connection with Content-Length framing
- **`protocol/`** — LSP 3.18 type definitions (`types.go`), method constants (`methods.go`), range utilities
- **`document/`** — Thread-safe document store with incremental edit handling and UTF-16 position conversion
- **`treesitter/`** — Parser-per-document lifecycle (`manager.go`), diagnostic engine with incremental processing (`engine.go`), Check (declarative) and Analyzer (imperative) APIs (`analysis.go`), query helpers (`node.go`, `query.go`)
- **`jsonschema/`** — JSON Schema validation engine, completion suggestions, AST types
- **`config/`** — TOML config loading with validation, fsnotify hot-reload, per-folder workspace config resolution
- **`middleware/`** — Composable `net/http`-style middleware chain: logging, recovery, tracing, telemetry
- **`transport/`** — 6 pluggable transports behind a common `io.ReadWriteCloser` interface
- **`gossiptest/`** — In-memory LSP client (`harness.go`), test fixtures, LSP-specific assertions
- **`examples/`** — minimal (15-line hover server), configurable (TOML + hot-reload), complete (full-featured)

### Key Patterns

**Handler registration:** Functional `Server.On*` methods (e.g., `OnHover`, `OnCompletion`). Capabilities are auto-detected from registered handlers. Custom methods via `HandleRequest`/`HandleNotification`.

**Request context:** `*gossip.Context` wraps `context.Context` with `Client`, `Documents`, `Server()`, `Logger()`, `WorkspaceRoot()`, `FolderFor(uri)`.

**Diagnostics — two APIs** (see `docs/RULE-AUTHORING.md` for full guide):
- **Check** — Declarative, tree-sitter query pattern. Framework handles incremental scoping and caching. Register via `Server.Check()`.
- **Analyzer** — Imperative, full `AnalysisContext` access. Manual scoping via `Diff.ChangedRanges` or `ScopeFile`. Register via `Server.Analyze()`.

**Testing:** Use `gossiptest.NewClient(t, server)` for in-memory integration tests. `WaitForDiagnostics`, `AssertDiagnosticRanges`, `AssertHoverContains` for assertions. Diagnostic range tests use data-driven JSON expectations in `treesitter/testdata/diagnostic_expectations.json`.

**Concurrency:** Thread-safe document store, `sync.RWMutex` on server state, atomic config swaps. Document lifecycle auto-wired to `didOpen`/`didChange`/`didClose`.

### Dependencies

Core JSON-RPC + stdio has zero external dependencies. Tree-sitter requires `github.com/tree-sitter/go-tree-sitter` plus grammar packages. Config uses `github.com/BurntSushi/toml` and `github.com/fsnotify/fsnotify`. WebSocket uses `golang.org/x/net`.
