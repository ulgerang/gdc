## Context

The existing CI covers builds, vet, tests, race tests, OpenSpec validation, and a fresh self-hosted graph check, but it does not run the analyzers that exposed the current debt. The findings span several packages, yet most are mechanical; the behavioral exception is `Database.GetStats`, which currently ignores SQL query and row-scan failures and can return plausible but incomplete values.

## Goals / Non-Goals

**Goals:**

- Make database statistics fail closed on every query, scan, iteration, or close error.
- Resolve all findings from the pinned repository lint command without broad exclusions.
- Preserve current CLI syntax, successful output, parser results, and data formats.
- Keep CI and self-hosted GDC validation capable of detecting regression.

**Non-Goals:**

- Repo-wide refactoring or formatting unrelated to analyzer findings.
- Changing the graph schema or parser feature set.
- Treating pre-existing heuristic GDC warnings outside the touched scope as implementation failures.

## Decisions

1. `GetStats` will retain its public signature but return no partial result when a SQL operation fails. Each failure will be wrapped with operation context, and row iteration and close errors will be checked.
2. Read-only parser file close failures and terminal color writer failures will be acknowledged explicitly at the point where the result cannot affect the completed read or command semantics. Database close failures in mutating paths will be propagated when there is no earlier error.
3. Dead helpers will be removed, and deprecated or redundant patterns will be replaced with local, dependency-free equivalents. No new runtime dependency is required.
4. CI will run the official golangci-lint action at a pinned major and tool version. The configuration keeps correctness analyzers enabled and excludes only `QF` quick-fix style suggestions to avoid a behavior-neutral repo-wide rewrite; the accepted baseline is zero findings from the configured command.
5. The self-hosted graph step will include implementation verification so the command matches the OpenSpec requirement it claims to enforce.

## Risks / Trade-offs

- **A stricter stats path can now fail where it previously returned zeros** → This is intentional fail-closed behavior and will be covered by a closed-database regression test.
- **Pinned lint tooling can become stale** → Pinning keeps CI reproducible; upgrades can be reviewed independently.
- **Cleanup edits touch many files** → Changes stay limited to analyzer-reported lines, with full tests, race tests, vet, and diff review before publication.

## Migration Plan

No user data migration is required. The branch can be reverted as one commit if a regression is found; database and CLI formats remain unchanged.

## Open Questions

None.
