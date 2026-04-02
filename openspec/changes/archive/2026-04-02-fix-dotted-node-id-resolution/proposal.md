## Why

Code sync can emit node IDs that already include qualifying segments such as package or path context. Today, `query` and `show` derive canonical IDs differently from `diff`, so the same synced node can appear addressable in one command and unresolved in another.

## What Changes

- Make canonical node IDs stable when `node.id` already contains its namespace prefix.
- Make implementation lookup in `diff` and `check` resolve dotted synced IDs back to the source symbol name found in code.
- Add regression coverage for dotted synced IDs across canonical ID generation and source lookup.

## Capabilities

### New Capabilities
- `node-id-resolution`: Ensure GDC commands interpret dotted synced node IDs consistently across query, show, diff, and implementation verification.

### Modified Capabilities

## Impact

- Affected code: `internal/node`, `internal/cli/diff.go`, `internal/cli/check.go`, and related tests.
- User-facing impact: fixes false "symbol not found" failures for code-synced dotted IDs.
- No new dependencies or external API changes.
