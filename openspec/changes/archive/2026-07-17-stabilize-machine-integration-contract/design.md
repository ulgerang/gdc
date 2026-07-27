## Context

`query` already has path-aware matching and canonical IDs, while `diff` still opens a node file by concatenating the argument with `.yaml`. JSON query output also shares the human-output path, so ambiguity guidance can precede the JSON document. Sentinel and other machine consumers need these commands to compose without scraping text.

## Goals / Non-Goals

**Goals:**
- Preserve the current interactive query experience by default.
- Provide JSON-only, all-match query results for machine consumers.
- Allow the canonical ID emitted by query to be accepted by diff.

**Non-Goals:**
- Redesign fuzzy matching or ranking.
- Change the persisted node schema.
- Add a daemon or network API.

## Decisions

1. Add `query --all`. With JSON output it returns an array in ranked order; no match returns `[]` successfully. This gives scanners a deterministic file-to-nodes operation without a separate index API.
2. Suppress human ambiguity output whenever a structured format is selected. Default text output remains unchanged.
3. Resolve diff arguments by loading node specs and using the shared canonical/short-ID lookup. The resolved canonical ID is returned in JSON so the response remains attributable.

## Risks / Trade-offs

- [Existing scripts may expect query JSON to be an object] → The object shape remains the default; arrays are opt-in through `--all`.
- [Unique short IDs can become ambiguous] → Reuse the existing lookup rule and fail rather than select an arbitrary node.

## Migration Plan

No migration is required. Consumers can adopt `query --all --format json` incrementally.
