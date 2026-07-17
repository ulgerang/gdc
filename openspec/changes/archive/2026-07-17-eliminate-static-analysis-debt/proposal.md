## Why

The repository's documented validation currently passes while independent static analysis still reports unchecked database, file, and output errors plus dead and deprecated code. These gaps can hide incomplete results and allow the same quality debt to return after this review.

## What Changes

- Surface database statistics query and scan failures instead of returning partial zero-valued results.
- Make resource cleanup and CLI output handling explicit where failures are currently ignored.
- Remove dead helpers and replace deprecated, redundant, or ineffectual code patterns reported by the repository's static analyzers.
- Extend automated repository validation with a pinned static-analysis gate and keep the fresh self-hosted GDC check aligned with implementation verification.
- Reconcile completed OpenSpec changes and replace archived-spec placeholder purposes with current capability descriptions.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `cli-integrity`: Database-backed operations must report retrieval failures rather than presenting partial results as valid.
- `project-quality-gates`: Repository acceptance must include static analysis and implementation-aware self-hosted graph validation.

## Impact

The change touches database access, CLI presentation and cleanup paths, parser utilities, tests, CI validation, and the OpenSpec/GDC metadata for those files. It does not change command-line syntax or persisted data formats.
