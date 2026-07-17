## 1. Behavioral Hardening

- [x] 1.1 Add regression coverage proving database statistics return a contextual error instead of partial data when storage is unavailable
- [x] 1.2 Propagate statistics query, scan, iteration, and close failures while preserving the public API
- [x] 1.3 Propagate database close failures in mutating CLI paths and explicitly handle best-effort read/output cleanup

## 2. Static Analysis Cleanup

- [x] 2.1 Remove dead helpers and ineffectual assignments reported by static analysis
- [x] 2.2 Replace deprecated and redundant patterns without adding runtime dependencies
- [x] 2.3 Run golangci-lint and staticcheck until both report zero findings

## 3. Quality Contracts

- [x] 3.1 Add a pinned golangci-lint CI gate and enable implementation verification in the fresh self-hosted check
- [x] 3.2 Replace placeholder main-spec purposes and reconcile the completed OpenSpec archive state
- [x] 3.3 Synchronize and verify touched GDC structural metadata

## 4. Acceptance

- [x] 4.1 Run formatting, vet, focused tests, full tests, race tests, and both documented builds
- [x] 4.2 Run strict OpenSpec validation and fresh-binary self-hosted GDC validation
- [x] 4.3 Review the final diff and record verification evidence
