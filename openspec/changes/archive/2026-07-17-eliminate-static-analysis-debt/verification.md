# Verification

## Acceptance scenario evidence

### Database-backed results fail closed

- `go test ./internal/db -run TestGetStatsReturnsContextualErrorWhenDatabaseIsClosed -count=1`
- The regression test first failed against the previous implementation because `GetStats` returned `total_nodes: 0` and `total_edges: 0` after the database was closed.
- After implementation, the test passes and verifies a nil result plus contextual `count nodes` error.

### Proposed change is statically validated

- `golangci-lint config verify`
- `golangci-lint run ./...` → `0 issues.`
- `staticcheck ./...` → no findings.
- CI uses `golangci/golangci-lint-action@v9` with `version: v2.11`.

### GDC validates its own repository

- Fresh binaries built successfully from both `.` and `./cmd/gdc`.
- Fresh `cmd/gdc` binary: `gdc check --verify-impl --no-orphan-info --exit-on-warning` → `No issues found!`
- Removed the four GDC nodes corresponding to deleted dead helpers and synchronized the named-return signature for `Database.GetStats`.
- Targeted code-sync dry-run for all touched production files exposed broad pre-existing extraction drift (`87 created, 201 updated, 1 deleted`), so it was not applied. Symbol-scoped sync was used instead to avoid unrelated graph churn.
- `gdc diff Database` still reports the private `resetDerivedTablesForLegacyNodeTypes` helper as an extra method. This is the existing diff granularity limitation; implementation-aware `gdc check` is clean.

## Full validation

- `go build -o <temp>/gdc-root.exe .`
- `go build -o <temp>/gdc-cmd.exe ./cmd/gdc`
- `go vet ./...`
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `golangci-lint config verify`
- `golangci-lint run ./...`
- `staticcheck ./...`
- `openspec validate eliminate-static-analysis-debt --strict`
- `openspec validate --all --strict`
- `git diff --check`

All blocking validations passed. The only remaining reported item is the documented non-blocking `gdc diff Database` private-method granularity limitation above.
