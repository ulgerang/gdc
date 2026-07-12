## 1. Node Lifecycle Integrity

- [x] 1.1 Add CLI regression tests for referenced delete rejection, forced delete cleanup, rename propagation, and no-partial-write failures
- [x] 1.2 Implement planned, rollback-capable YAML mutations for node delete and rename
- [x] 1.3 Refresh the derived database after successful lifecycle mutations and verify recovery messaging

## 2. Discovery And Launcher Reliability

- [x] 2.1 Add regression tests for explicit source traversal failures and query fallback scan failures
- [x] 2.2 Propagate filesystem traversal and read errors through query and sync commands
- [x] 2.3 Make Windows and POSIX repository launchers run current source by default with explicit prebuilt opt-in

## 3. Structural And Documentation Reconciliation

- [x] 3.1 Sync and reconcile touched GDC node contracts and calibrate non-actionable self-check warnings
- [x] 3.2 Update SPEC, usage, README, and roadmap statements to match lifecycle and launcher behavior
- [x] 3.3 Archive previously completed OpenSpec changes after strict validation

## 4. Automated Quality Gates

- [x] 4.1 Add CI checks for formatting, both build entrypoints, vet, tests, race tests, and strict OpenSpec validation
- [x] 4.2 Add a freshly built self-hosted GDC consistency check to repository validation

## 5. Acceptance Verification

- [x] 5.1 Run focused lifecycle, traversal, and wrapper verification
- [x] 5.2 Run full tests, vet, race checks, both builds, OpenSpec strict validation, and touched-scope GDC checks
- [x] 5.3 Map each OpenSpec scenario to direct evidence and record any remaining limitation
