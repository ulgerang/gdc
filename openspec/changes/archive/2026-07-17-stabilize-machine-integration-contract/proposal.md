## Why

Machine consumers cannot safely compose `query` and `diff` today: ambiguous JSON queries write human guidance before the JSON document, and `diff` does not resolve the canonical IDs returned by `query`. GDC needs a stable machine-facing contract before companion tools can inspect multi-node source files reliably.

## What Changes

- Add an opt-in query mode that returns every matching node as one JSON array without human-oriented stdout.
- Make all JSON query modes emit JSON-only stdout.
- Make `diff` resolve canonical, qualified, and unique short node IDs through the same lookup model used by other traversal commands.
- Add regression coverage for multi-node file queries and query-to-diff composition.

## Capabilities

### New Capabilities
- `machine-query-output`: Defines deterministic JSON-only query output and multi-match results for machine consumers.

### Modified Capabilities
- `node-id-resolution`: Extends canonical ID resolution guarantees to `gdc diff`.

## Impact

Affected code is limited to `internal/cli/query.go`, `internal/cli/diff.go`, their tests, CLI documentation, and the corresponding OpenSpec contracts. Existing default human-readable query behavior remains unchanged.
