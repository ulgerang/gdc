## Why

GDC already supports a spec-first workflow, but the README reads more like a command reference than a product operating model. Users should be able to understand that GDC is intended to design structure before code, produce narrow implementation context for coders or agents, and verify drift after implementation.

The CLI also resolves short node aliases in some commands, but forward trace and graph export can still treat a short dependency target such as `IInputManager` as missing when the canonical node is `Game.Input.IInputManager`.

## What Changes

- Document the design-first loop prominently in the README.
- Clarify that `gdc query`, `trace`, and `extract` form the context packet used before implementation.
- Resolve unique short dependency targets to canonical node IDs in trace path traversal and graph exports.
- Add regression coverage for namespaced dependency targets referenced by short names.

## Impact

- Affected docs: `README.md`
- Affected code: `internal/cli/root.go`, `internal/cli/trace.go`, `internal/cli/graph.go`
- Affected tests: `internal/cli/*_test.go`
- User-facing impact: GDC's README better reflects its intended workflow, and graph/trace commands no longer show resolvable short dependency targets as missing or disconnected.
