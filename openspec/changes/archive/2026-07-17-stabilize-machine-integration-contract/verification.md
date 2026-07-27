## Verification

### Machine query scenarios

- Multi-node JSON query: a fresh binary queried `internal/cli/root.go` with `--all --format json`; PowerShell decoded one JSON document containing 9 nodes.
- Empty all-match query: a missing symbol decoded as an empty JSON array with exit code 0.
- Structured ambiguity isolation: the same multi-node file decoded successfully in single-result JSON mode without removing a text prefix.
- Unit coverage: `TestMarshalQueryMatchesAllReturnsEveryMatchAsJSONArray`, `TestMarshalQueryMatchesAllReturnsEmptyArrayForNoMatches`, and `TestStructuredQueryOutputSuppressesHumanGuidance` pass.

### Diff identity scenarios

- Query-to-diff composition: `gdc diff cli.runQuery --format json` succeeds and returns `node: cli.runQuery`.
- Canonical and ambiguous ID unit coverage: `TestResolveDiffNodeSpecAcceptsCanonicalID` and `TestResolveDiffNodeSpecRejectsAmbiguousShortID` pass.

### Validation stack

- `go test ./...`: PASS
- `go vet ./...`: PASS
- `openspec validate stabilize-machine-integration-contract --strict`: PASS
- `gdc check --verify-impl --no-orphan-info --format json`: PASS with 0 errors, 0 warnings, 0 info
- Touched graph sync: `internal/cli/query.go` and `internal/cli/diff.go` synchronized; 15 nodes created and 26 existing query nodes refreshed.

### Remaining limitation

Function-node `gdc diff` can still report the enclosing function itself as an `extra_method`; this pre-existing parser granularity does not appear in `gdc check --verify-impl` and is outside this machine-contract change.
