---
name: LSP Feature Gaps Plan
overview: Fix bugs in existing LSP handlers, enrich gossip protocol types to match LSP 3.18, and implement missing standard LSP features across both gossip and telescope-go.
todos:
  - id: fix-rename-security
    content: "P0: Fix renameSecurityUsages no-op bug -- add SecurityRequirementEntry with NameLoc to openapi types, populate in index builder, emit TextEdits in rename.go"
    status: completed
  - id: fix-document-links
    content: "P0: Fix document links -- set Target URI on DocumentLink for internal and external $ref targets, improve tooltip"
    status: completed
  - id: fix-rename-capability
    content: "P0: Fix RenameProvider capability -- add RenameOptions struct, advertise prepareProvider: true in capabilities.go"
    status: completed
  - id: enrich-completion-item
    content: "P1: Enrich CompletionItem in protocol/types.go -- add InsertTextFormat, SortText, FilterText, AdditionalTextEdits, CommitCharacters, Command, Tags, Preselect"
    status: completed
  - id: enrich-code-action
    content: "P1: Enrich CodeAction and CodeActionContext in protocol/types.go -- add IsPreferred, Disabled, Data, Only, TriggerKind"
    status: completed
  - id: enrich-inlay-hint
    content: "P1: Enrich InlayHint in protocol/types.go -- add Tooltip, PaddingLeft, PaddingRight, TextEdits, Data"
    status: completed
  - id: snippet-completions
    content: "P1: Add snippet completions in telescope-go/lsp/completion.go using InsertTextFormat for $ref, status codes, security schemes"
    status: completed
  - id: preferred-code-actions
    content: "P1: Mark preferred code actions in telescope-go/lsp/code_actions.go -- IsPreferred on structural typo fixes and suppressions"
    status: completed
  - id: semantic-tokens-range
    content: "P1: Implement SemanticTokensRange handler -- extract shared token builder, filter by range, register in server.go"
    status: completed
  - id: expand-folding
    content: "P1: Expand folding ranges to cover info, servers, tags, all component groups and entries, use comment kind for descriptions"
    status: completed
  - id: expand-references
    content: "P1: Expand references to all component kinds, tags, IncludeDeclaration, cross-workspace search via cache.All()"
    status: completed
  - id: expand-inlay-hints
    content: "P1: Expand inlay hints -- required markers, deprecated badges, parameter in: hints"
    status: completed
  - id: formatting-handler
    content: "P2: Implement formatting handler -- YAML re-indent via tree-sitter AST, JSON via MarshalIndent"
    status: completed
  - id: type-definition
    content: "P2: Implement type definition handler -- resolve to schema type from $ref, parameters, responses, properties"
    status: completed
  - id: security-highlights
    content: "P2: Add security scheme usages to document highlights (depends on location tracking from fix-rename-security)"
    status: completed
isProject: false
---

# LSP Feature Gaps Implementation Plan

This plan is organized into three tiers: P0 (correctness bugs), P1 (high-impact enrichment), and P2 (completeness). All changes span both repos: `gossip` (framework) and `telescope-go` (LSP server).

---

## P0: Fix Bugs in Existing Handlers

### 1. Fix `renameSecurityUsages` no-op bug

`[telescope-go/lsp/rename.go](telescope-go/lsp/rename.go)` lines 231-258: the function detects security scheme usages but assigns to `_` instead of appending `TextEdit`s. Both root-level and operation-level security arrays need location tracking.

**Problem:** Security scheme keys in `security:` arrays lack tracked locations in the OpenAPI index. The `openapi.SecurityRequirement` is `map[string][]string` with no location info.

**Fix:**

- In `[telescope-go/openapi/types.go](telescope-go/openapi/types.go)`, add a `SecurityRequirementEntry` struct with `Name string`, `Scopes []string`, `NameLoc openapi.Loc` fields. Update `Operation.Security` and `Document.Security` to use `[]SecurityRequirementEntry` (or add a parallel `SecurityLocs` field).
- In the OpenAPI index builder, capture the YAML key node range for each security requirement entry.
- In `renameSecurityUsages`, use the tracked `NameLoc` to emit proper `TextEdit`s.

### 2. Fix document links (missing `Target`)

`[telescope-go/lsp/document_links.go](telescope-go/lsp/document_links.go)`: emits `DocumentLink` with no `Target`, making links non-navigable.

**Fix:**

- For internal `$ref` targets (e.g. `#/components/schemas/Foo`), set `Target` to the document's own URI. The tooltip should show the ref path.
- For external file refs (e.g. `./common.yaml#/components/schemas/Bar`), resolve the file path relative to the document and set `Target` to the resolved file URI.
- The `Tooltip` should be the `$ref` value itself, not the generic `"$ref target"`.

### 3. Fix `RenameProvider` capability advertisement

`[gossip/capabilities.go](gossip/capabilities.go)` line 57: when `PrepareRename` is registered, `RenameProvider` should be `map[string]interface{}{"prepareProvider": true}` instead of bare `true`. Without this, clients don't know to call `textDocument/prepareRename`.

**Fix:** Add a `RenameOptions` struct to `[gossip/protocol/types.go](gossip/protocol/types.go)`:

```go
type RenameOptions struct {
    PrepareProvider bool `json:"prepareProvider,omitempty"`
}
```

In `capabilities.go`, when `has(MethodPrepareRename)`, set `caps.RenameProvider = RenameOptions{PrepareProvider: true}`.

---

## P1: Enrich Protocol Types (gossip)

### 4. Enrich `CompletionItem`

Add missing fields to `[gossip/protocol/types.go](gossip/protocol/types.go)` `CompletionItem`:

```go
type CompletionItem struct {
    Label               string              `json:"label"`
    Kind                CompletionItemKind  `json:"kind,omitempty"`
    Tags                []CompletionItemTag `json:"tags,omitempty"`
    Detail              string              `json:"detail,omitempty"`
    Documentation       interface{}         `json:"documentation,omitempty"`
    Preselect           bool                `json:"preselect,omitempty"`
    SortText            string              `json:"sortText,omitempty"`
    FilterText          string              `json:"filterText,omitempty"`
    InsertText          string              `json:"insertText,omitempty"`
    InsertTextFormat    *InsertTextFormat   `json:"insertTextFormat,omitempty"`
    TextEdit            *TextEdit           `json:"textEdit,omitempty"`
    AdditionalTextEdits []TextEdit          `json:"additionalTextEdits,omitempty"`
    CommitCharacters    []string            `json:"commitCharacters,omitempty"`
    Command             *Command            `json:"command,omitempty"`
    Data                interface{}         `json:"data,omitempty"`
}

type CompletionItemTag int
const CompletionItemTagDeprecated CompletionItemTag = 1

type InsertTextFormat int
const (
    InsertTextFormatPlainText InsertTextFormat = 1
    InsertTextFormatSnippet   InsertTextFormat = 2
)
```

### 5. Enrich `CodeAction` and `CodeActionContext`

Add missing fields to `CodeAction`:

```go
type CodeAction struct {
    Title       string       `json:"title"`
    Kind        string       `json:"kind,omitempty"`
    Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
    IsPreferred bool         `json:"isPreferred,omitempty"`
    Disabled    *CodeActionDisabled `json:"disabled,omitempty"`
    Edit        *WorkspaceEdit `json:"edit,omitempty"`
    Command     *Command     `json:"command,omitempty"`
    Data        interface{}  `json:"data,omitempty"`
}

type CodeActionDisabled struct {
    Reason string `json:"reason"`
}
```

Add missing fields to `CodeActionContext`:

```go
type CodeActionContext struct {
    Diagnostics []Diagnostic        `json:"diagnostics"`
    Only        []CodeActionKind    `json:"only,omitempty"`
    TriggerKind *CodeActionTriggerKind `json:"triggerKind,omitempty"`
}

type CodeActionKind = string
type CodeActionTriggerKind int
const (
    CodeActionTriggerInvoked  CodeActionTriggerKind = 1
    CodeActionTriggerAutomatic CodeActionTriggerKind = 2
)
```

### 6. Enrich `InlayHint`

```go
type InlayHint struct {
    Position     Position    `json:"position"`
    Label        interface{} `json:"label"`
    Kind         *int        `json:"kind,omitempty"`
    Tooltip      interface{} `json:"tooltip,omitempty"`
    PaddingLeft  bool        `json:"paddingLeft,omitempty"`
    PaddingRight bool        `json:"paddingRight,omitempty"`
    TextEdits    []TextEdit  `json:"textEdits,omitempty"`
    Data         interface{} `json:"data,omitempty"`
}
```

---

## P1: High-Impact Feature Work (telescope-go)

### 7. Snippet completions

Leverage the new `InsertTextFormat` on `CompletionItem` to provide snippet completions in `[telescope-go/lsp/completion.go](telescope-go/lsp/completion.go)`:

- **Schema `$ref` completions**: insert `$ref: '#/components/schemas/${1:SchemaName}'` as a snippet.
- **Status code completions**: append cursor-positioned description: `'200':\n  description: ${1:OK}`.
- **Security scheme completions**: insert with scopes placeholder: `- schemeName:\n  - ${1:scope}`.
- Set `InsertTextFormat: &InsertTextFormatSnippet` on these items.

### 8. Mark preferred code actions

In `[telescope-go/lsp/code_actions.go](telescope-go/lsp/code_actions.go)`:

- Set `IsPreferred: true` on the `invalidKeyQuickFix` "Rename to..." action (structural validation typo fix).
- Set `IsPreferred: true` on the "Suppress for this line" action when it's the only quick fix for a diagnostic.
- Use `CodeActionKind` constants (`"quickfix"`, `"refactor"`, `"source"`) consistently.

### 9. Semantic tokens range handler

Register `OnSemanticTokensRange` in `[telescope-go/lsp/server.go](telescope-go/lsp/server.go)`. The implementation reuses the full-document logic from `[semantic_tokens.go](telescope-go/lsp/semantic_tokens.go)` but filters tokens to only those within `params.Range`:

```go
func NewSemanticTokensRangeHandler(cache *openapi.IndexCache) gossip.SemanticTokensRangeHandler {
    return func(ctx *gossip.Context, params *protocol.SemanticTokensRangeParams) (*protocol.SemanticTokens, error) {
        // Same token-building logic, then filter:
        // keep only tokens where token.line >= params.Range.Start.Line
        // && token.line <= params.Range.End.Line
    }
}
```

Extract the shared token-building logic into a helper `buildTokens(idx) []semanticToken` to avoid duplication.

### 10. Expand folding ranges

Extend `[telescope-go/lsp/folding.go](telescope-go/lsp/folding.go)` to fold:

- `info:` section
- `servers:` section and individual server entries
- `tags:` section and individual tag entries
- All component groups (`parameters`, `responses`, `requestBodies`, `headers`, `securitySchemes`, `links`, `callbacks`) and their individual entries
- Use `"comment"` kind for multi-line `description` values

### 11. Expand references handler

Extend `[telescope-go/lsp/references.go](telescope-go/lsp/references.go)`:

- Add `requestBodies`, `headers`, `links`, `examples` to the component kinds search.
- Add tag references: find all operations that use a tag.
- Respect `params.Context.IncludeDeclaration` by optionally including the component definition location.
- Search across all workspace documents via `cache.All()` (currently only searches the current document's index).

### 12. Expand inlay hints

Extend `[telescope-go/lsp/inlay_hints.go](telescope-go/lsp/inlay_hints.go)`:

- Show `*required` marker hints after required property names in schemas.
- Show `deprecated` hints on deprecated operations and parameters.
- Show parameter `in:`  hints (query/path/header/cookie) next to parameter names.
- Use the new `PaddingLeft` and `Tooltip` fields for readability.

---

## P2: Completeness Features

### 13. Formatting handler

Register `OnFormatting` in `[telescope-go/lsp/server.go](telescope-go/lsp/server.go)` with a new `[telescope-go/lsp/formatting.go](telescope-go/lsp/formatting.go)`:

- For YAML files: re-indent the entire document using the tree-sitter AST to determine structure depth, applying `params.Options.TabSize` spaces per level. Normalize trailing newlines.
- For JSON files: use `json.MarshalIndent` after parsing to re-format with consistent indentation.
- Return a single `TextEdit` replacing the entire document content.

This is a straightforward implementation since tree-sitter already provides the full AST for both YAML and JSON.

### 14. Type definition handler

Register `OnTypeDefinition` in `[telescope-go/lsp/server.go](telescope-go/lsp/server.go)` with a new `[telescope-go/lsp/type_definition.go](telescope-go/lsp/type_definition.go)`:

- When cursor is on a `$ref`, resolve to the target schema and jump to it (same as definition for `$ref`).
- When cursor is on a parameter name, jump to the parameter's `schema` or `$ref` target.
- When cursor is on a response, jump to the response content schema.
- When cursor is on a property name inside a schema, jump to the property's type schema.

### 15. Document highlights: add security scheme usages

In `[telescope-go/lsp/document_highlights.go](telescope-go/lsp/document_highlights.go)`, add a section after the tag highlights to also highlight security scheme name occurrences in `security:` arrays within the current document. This requires the location tracking from item 1.

---

## Not Included (Future Considerations)

The following were identified as gaps but intentionally excluded from this plan due to lower ROI or significant complexity:

- **Workspace diagnostics / cross-file validation**: Requires major architecture changes to the diagnostic pipeline.
- **Pull-based diagnostics**: Current push model works; pull would be an optimization.
- **Declaration vs Definition separation**: Marginal UX benefit for OpenAPI.
- **Implementation handler**: Useful for discriminators but very niche.
- **Document color / color presentation**: Extremely niche for OpenAPI.
- **ClientCapabilities parsing**: Feature negotiation is a nice-to-have; can be added incrementally.
- **Progress reporting**: Useful for large workspaces but requires client-side wiring.

