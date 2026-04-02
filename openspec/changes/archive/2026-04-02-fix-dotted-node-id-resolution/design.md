## Context

`gdc sync --direction code` resolves duplicate extracted IDs by writing a dotted identifier into `node.id`. The current `QualifiedID()` implementation always prepends `namespace`, which double-prefixes synced nodes like `tools.Runtime` into `tools.tools.Runtime`. Separately, `diff` and implementation verification search source using the literal stored `node.id`, which fails because parsed source symbols remain bare (`Runtime`, `CapacityStore`, and similar).

## Goals / Non-Goals

**Goals:**
- Preserve dotted synced IDs as valid canonical identifiers without double-prefixing the namespace.
- Allow `diff` and implementation verification to locate the underlying source symbol for dotted synced IDs.
- Lock the behavior with regression tests close to the affected helpers.

**Non-Goals:**
- Redesign the sync ID scheme or migrate existing specs to a different storage model.
- Normalize every historical GDC node file in the repository.
- Change how non-dotted node IDs are resolved.

## Decisions

- Treat `node.id` as already canonical when it exactly equals `namespace` or starts with `namespace.`.
  Rationale: this is the smallest fix that makes `query/show` agree with code-synced specs without changing the sync format.
- Resolve source lookup for dotted IDs by matching both the stored ID and its terminal symbol segment.
  Rationale: synced IDs intentionally carry collision context, but parsers still return the real source symbol name. Matching the terminal segment fixes `diff/check` while keeping exact matches preferred.
- Add targeted unit tests instead of a full CLI integration test.
  Rationale: the bug sits in shared helpers (`QualifiedID` and extracted-node lookup), so focused tests give fast coverage with less setup noise.

## Risks / Trade-offs

- [Risk] A dotted ID that is also a real source symbol string could ambiguously match two candidates. → Mitigation: prefer exact full-ID matches before falling back to the terminal segment.
- [Risk] Existing callers may rely on double-prefixed canonical IDs in snapshots or logs. → Mitigation: limit the change to cases where `node.id` already starts with the namespace.
- [Risk] Future sync formats might encode more than one qualifier segment. → Mitigation: source lookup uses the terminal segment generically rather than assuming a single prefix.
