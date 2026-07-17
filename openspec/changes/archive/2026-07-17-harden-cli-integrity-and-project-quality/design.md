## Context

GDC stores the editable graph contract in `.gdc/nodes/*.yaml` and derives a SQLite index from those files. The current node lifecycle commands mutate only one YAML file: deletion does not inspect reverse references, and rename does not update dependency targets or the database. Source discovery helpers also suppress filesystem traversal errors. Separately, repository wrappers prefer any existing local executable, even when it predates the checked-out source.

The change crosses CLI, persistence, developer tooling, tests, CI, GDC metadata, and documentation. OpenSpec remains the behavioral authority; GDC is used only to reconcile the touched structural scope.

## Goals / Non-Goals

**Goals:**

- Preserve graph referential integrity during node deletion and rename.
- Make lifecycle mutations plan-first so validation failures do not partially edit the graph.
- Keep the derived database consistent after successful lifecycle mutations.
- Report source traversal failures rather than silently accepting incomplete discovery.
- Ensure repository wrappers run current source unless prebuilt execution is explicitly requested.
- Establish automated validation and reconcile current documentation and structural metadata.

**Non-Goals:**

- Redesign the node storage format or replace SQLite.
- Normalize all historical `.gdc` graph debt or raise repository-wide coverage to an arbitrary percentage.
- Add a general filesystem transaction framework.
- Change successful query, sync, or node command output beyond necessary safety messages.

## Decisions

### Plan lifecycle mutations before writing

Node delete and rename will load all YAML specifications, resolve canonical and uniquely resolvable short dependency targets, and construct the complete mutation set before touching disk. Delete will fail when reverse references exist unless `--force` is set. Rename will update every matching dependency target.

This is preferred over fixing references after the primary file has already moved or disappeared, because validation and serialization failures can be detected before mutation.

### Use atomic per-file replacement with rollback snapshots

Each affected YAML file will be serialized to a temporary sibling and atomically renamed into place. Original bytes will be retained until all replacements succeed; a failure will restore already changed files. This provides practical multi-file safety without introducing a new persistence dependency.

### Rebuild the derived index after successful YAML mutation

The lifecycle command will invoke a shared YAML-to-database synchronization helper after filesystem changes. If database refresh fails, the command will return an error explaining that YAML is authoritative and a manual `gdc sync` retry is required. The mutation itself will not be rolled back merely because the derived cache could not refresh.

### Treat traversal failures as command failures for explicit scopes

Explicit `sync --dirs` scopes and query fallback scanning will collect and return walk/read errors. Excluded directories remain intentionally skipped. This favors complete, trustworthy results over partial success that appears complete.

### Separate development launch from prebuilt launch

Repository wrappers will use `go run ./cmd/gdc` by default. An explicit environment variable will opt into a prebuilt executable for release or offline use. This avoids timestamp heuristics and behaves consistently across branch switches.

### Make CI reproduce the repository acceptance contract

CI will build both documented entrypoints and run formatting, vet, tests, race tests, OpenSpec strict validation, and a freshly built self-hosted GDC check. Generated local binaries are never used as validation inputs.

## Risks / Trade-offs

- **Multi-file rename can still be interrupted by process termination** → write temporary files beside targets, replace only after all serialization succeeds, and keep rollback snapshots for ordinary errors.
- **Forced deletion intentionally creates dangling references** → `--force` will remove matching dependency edges from referencing YAML specs rather than preserve broken references.
- **Database refresh failure leaves stale derived data** → return a clear actionable error and document `gdc sync` as recovery; YAML remains authoritative.
- **Strict traversal errors may fail commands that previously returned partial results** → limit strictness to requested/project source roots and retain existing excluded-directory behavior.
- **Race checks increase CI duration** → run them as a separate job or step so basic validation feedback remains clear.

## Migration Plan

1. Add regression tests that capture safe delete, forced delete, rename propagation, traversal failure, and wrapper selection behavior.
2. Implement CLI and wrapper changes.
3. Reconcile touched GDC nodes and documentation.
4. Add CI and run the full acceptance stack from freshly built source.
5. Archive previously completed changes only after strict OpenSpec validation succeeds.

Rollback consists of reverting the change; no persistent schema migration is introduced.

## Open Questions

None. The existing SPEC behavior for safe delete and reference-updating rename is treated as the intended contract.
