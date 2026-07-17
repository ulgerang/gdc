## Why

GDC presents its YAML graph as a structural source of truth, but destructive node commands can currently leave broken references, filesystem traversal can silently omit source files, and development wrappers may run stale binaries. The repository also lacks automated quality gates and contains completed-but-unarchived changes and stale documentation, making it harder to trust both the tool and its development workflow.

## What Changes

- Make `gdc node delete` reject referenced nodes unless `--force` is supplied, and keep the YAML graph and database synchronized after deletion.
- Make `gdc node rename` update dependency references across node specifications and refresh the graph database.
- Surface filesystem traversal failures instead of returning silently incomplete query or sync results.
- Make repository development wrappers execute current source by default rather than preferring arbitrary stale local binaries.
- Add regression coverage and continuous integration for build, tests, vet, race checks, OpenSpec validation, and self-hosted GDC consistency.
- Reconcile touched GDC node specifications, current product documentation, and completed OpenSpec change state with implemented behavior.

## Capabilities

### New Capabilities

- `cli-integrity`: Safe node lifecycle operations, explicit source traversal failures, and current-source development launcher behavior.
- `project-quality-gates`: Automated repository validation and self-hosted specification consistency requirements.

### Modified Capabilities

None.

## Impact

- Affected CLI areas: `internal/cli/node.go`, source discovery helpers, and command-level tests.
- Affected development surfaces: `gdc.bat`, `gdc.sh`, CI configuration, Makefile or validation scripts.
- Affected structural/documentation surfaces: touched `.gdc/nodes/*.yaml`, `docs/SPEC.md`, README/usage guidance, and completed OpenSpec changes.
- No intentional breaking change to successful node command syntax; `node delete` gains the documented `--force` behavior and unsafe deletions become explicit failures.
