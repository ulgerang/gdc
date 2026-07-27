## Verification

- `go test ./...` passed.
- `go test -race ./...` passed.
- `go vet ./...` passed.
- `gofmt -d` on all touched Go files produced no diff.
- `git diff --check` passed.
- `openspec validate --all --strict` passed for all 10 changes and specs.
- `go run . check --verify-impl --no-orphan-info --format json` passed with zero errors. It reports the existing structural warning that `node.Spec` now has eight dependencies after adding the root implementation contract.
- `TestRunExtractForImplementationUsesOnlyGDCContracts` creates a temporary project containing only ready `.gdc` files, emits a complete source-free packet, and then proves an unresolved placeholder is rejected.
