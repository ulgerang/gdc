## Acceptance Evidence

### CLI integrity

| Scenario | Direct evidence |
|---|---|
| Referenced node deletion is rejected | `TestNodeDeleteRejectsReferencedNodeWithoutForce` verifies the error names the referencing node and both YAML files remain unchanged. |
| Referenced node is forcibly deleted | `TestNodeDeleteForceRemovesReferencesAndRefreshesDatabase` verifies node removal, dependency cleanup, and the refreshed SQLite index. |
| Node with dependents is renamed | `TestNodeRenameUpdatesReferencesAndRefreshesDatabase` verifies the file, node ID, dependency target, and SQLite index all use the new identity. |
| Rename planning fails | `TestNodeRenameTargetExistsLeavesSpecsUnchanged` verifies an existing target rejects the operation before modifying the source spec. |
| Database refresh recovery | `TestNodeDeleteReportsDatabaseRefreshRecovery` verifies YAML remains authoritative and the error directs the user to `gdc sync`. |
| Explicit sync directory cannot be traversed | `TestCollectExplicitSourceScopeFilesReportsTraversalFailure` verifies the missing requested path is returned as an error. |
| Query fallback scan cannot read project sources | `TestFindSourceHintsReportsTraversalFailure` verifies query hint discovery returns the missing source-root error. |
| Stale executable exists beside launcher | `TestRepositoryLaunchersRequireExplicitPrebuiltOptIn` plus `gdc.bat version` verify current source is the default and prebuilt use is guarded by `GDC_USE_PREBUILT`. |

### Project quality gates

| Scenario | Direct evidence |
|---|---|
| Proposed change is validated | `.github/workflows/ci.yml` builds both entrypoints and runs changed-file formatting, vet, tests, race tests, and `openspec validate --all --strict`. The workflow YAML was parsed locally. |
| GDC validates its own repository | A freshly built executable completed `gdc check --verify-impl --no-orphan-info --exit-on-warning` with `No issues found`. |
| Completed changes and roadmap are reviewed | Three prior completed changes were archived with delta specs merged; `openspec list --json` now contains only this active change, and strict validation reports five passing items. |

## Full Validation

- `go test ./...`: passed
- `go vet ./...`: passed
- `go test -race ./...`: passed with exit code 0
- `go build .`: passed
- `go build ./cmd/gdc`: passed
- `openspec validate --all --strict`: passed
- Fresh self-hosted `gdc check --verify-impl --no-orphan-info --exit-on-warning`: passed with zero issues
- `git diff --check`: passed after removing archive-generated trailing whitespace

## Remaining Non-Blocking Limitations

- `gdc diff` currently reports a function node's own function as an `extra_method`, and reports the private `Database.resetDerivedTablesForLegacyNodeTypes` helper as extra. The implementation-aware `gdc check` is clean; this diff granularity limitation predates the change and is outside its lifecycle-integrity scope.
- Much of the pre-existing Go tree is not `gofmt` clean. CI therefore rejects formatting regressions in changed Go files without creating a repository-wide mechanical formatting diff in this change.
