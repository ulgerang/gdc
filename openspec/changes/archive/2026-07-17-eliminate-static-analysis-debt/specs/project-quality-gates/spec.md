## MODIFIED Requirements

### Requirement: Automated repository acceptance gate
The repository SHALL provide automated validation that builds both documented CLI entrypoints and runs formatting checks, vet, static analysis, tests, race checks, and strict OpenSpec validation against current source.

#### Scenario: Proposed change is validated
- **WHEN** the automated workflow runs for a branch or pull request
- **THEN** it fails if either build entrypoint, formatting, vet, static analysis, tests, race checks, or strict OpenSpec validation fails

### Requirement: Self-hosted structural consistency
Repository validation SHALL build a fresh GDC executable and use it to verify implementation contracts without relying on stale local binaries.

#### Scenario: GDC validates its own repository
- **WHEN** the self-hosted consistency step runs
- **THEN** node contracts produce no implementation mismatch and any intentionally non-actionable heuristic warning is explicitly filtered or documented
