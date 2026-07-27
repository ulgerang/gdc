## Verification

- `go test ./...` passed.
- `go test -race ./...` passed.
- `go vet ./...` passed.
- `gofmt -d` on all touched Go files produced no diff.
- `git diff --check` passed.
- `openspec validate --all --strict` passed for all 10 changes and specs.
- `go run . check --verify-impl --no-orphan-info --format json` passed with zero errors. It reports the existing structural warning that `node.Spec` now has eight dependencies after adding the root implementation contract.
- `TestRunExtractForImplementationUsesOnlyGDCContracts` creates a temporary project containing only ready `.gdc` files, emits a complete source-free packet, and then proves an unresolved placeholder is rejected.
- The live Rand Defense `check --verify-impl --format json` probe passed with zero errors. It no longer reports false missing symbols for the `RuntimePrimitives` module, `ComponentStore<T>`, or multiline `ScriptContractCatalog.ComposeHostFunctions` declarations.
- The same live probe still reports the real `DeterministicPropertyGraphAdapter` contract drift as one `impl_mismatch` warning, proving the hardening does not suppress genuine mismatches.
- The live Rand Defense `extract package-portability-verifier --for-implementation --format json` probe succeeded and emitted a source-free, implementation-ready transitive contract packet.
- The live Rand Defense `diff RuntimePrimitives --format json` probe returned exit 0 with `node=RuntimePrimitives`, proving `diff` shares the module implementation binding instead of requiring a synthetic source symbol.
- The live Rand Defense implementation packet probe resolved `deterministic-property-adapter`, `DeterministicPropertyGraphAdapter`, and `deterministic-property-graph-adapter` to the same ready, source-free node contract.
