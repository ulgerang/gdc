## ADDED Requirements

### Requirement: Diff SHALL resolve query node identities
GDC `diff` SHALL accept canonical IDs, qualified IDs, and unique short IDs returned or recognized by `query`.

#### Scenario: Query result is passed to diff
- **WHEN** query returns canonical ID `cli.runQuery` for a spec stored as `runQuery.yaml`
- **THEN** `gdc diff cli.runQuery` loads that spec
- **AND** JSON output attributes the result to `cli.runQuery`

#### Scenario: Short ID is ambiguous
- **WHEN** more than one node resolves to the requested short ID
- **THEN** diff fails with an ambiguity error
- **AND** it does not select an arbitrary spec
