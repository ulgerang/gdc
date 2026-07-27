## ADDED Requirements

### Requirement: Schema 1.1 SHALL preserve implementation behavior contracts
GDC SHALL parse and retain public type contracts, method preconditions, postconditions, side effects, parameter examples, return constraints, logic rules, dependency member requirements, lifecycle constraints, and acceptance scenarios from schema 1.1 node files.

#### Scenario: Behavioral fields round-trip through the node model
- **WHEN** a schema 1.1 node containing every behavioral contract field is loaded and saved
- **THEN** GDC preserves those fields without loss
- **AND** extraction can render them without consulting implementation source

### Requirement: Schema 1.1 parsing SHALL reject unknown fields
GDC SHALL fail loading a schema 1.1 node when the YAML contains an unknown or misspelled field instead of silently discarding it.

#### Scenario: Authored behavior field is misspelled
- **WHEN** a schema 1.1 method uses `postconditons` instead of `postconditions`
- **THEN** node loading fails with a field-specific parse error
- **AND** GDC does not emit an implementation packet

#### Scenario: Existing schema 1.0 node contains legacy extensions
- **WHEN** an existing schema 1.0 node is loaded
- **THEN** GDC retains its current permissive compatibility behavior

### Requirement: Implementation extraction SHALL fail closed on incomplete contracts
`gdc extract <node> --for-implementation` SHALL emit a packet only when the target and its required dependency closure satisfy the implementation-readiness contract.

#### Scenario: Contract contains unresolved placeholders
- **WHEN** a target or required dependency contains `TBD`, `TODO`, `REQUIRES_APPROVAL`, or `NEEDS DESCRIPTION` in an implementation-relevant field
- **THEN** extraction fails and identifies the node and field

#### Scenario: Method has no behavioral contract
- **WHEN** a ready node declares a public method without a description or without preconditions, postconditions, side effects, or declared errors
- **THEN** extraction fails with an implementation-readiness diagnostic

#### Scenario: Acceptance oracle is missing
- **WHEN** a ready target has no structured acceptance scenario
- **THEN** extraction fails instead of claiming the packet is sufficient

### Requirement: Dependency closure SHALL be exact and lossless
Implementation extraction SHALL resolve every required transitive dependency, verify its declared contract hash, validate every member named by `requires`, and include the full resolved dependency contract exactly once in deterministic order.

#### Scenario: Required dependency member is unavailable
- **WHEN** an edge requires a member that is absent from the dependency interface
- **THEN** extraction fails and names the edge and missing member

#### Scenario: Dependency contract changed after authoring
- **WHEN** an edge contract hash differs from the current canonical dependency spec hash
- **THEN** extraction fails and reports the current hash required for review

#### Scenario: Transitive dependency graph contains a cycle
- **WHEN** ready nodes form a dependency cycle
- **THEN** the packet includes each resolved contract once in deterministic order
- **AND** extraction terminates without recursion overflow

### Requirement: Implementation packet SHALL be source-free
Implementation extraction SHALL include only authored GDC contracts and generated interface representations and SHALL NOT include implementation files, repository tests, callers, or references.

#### Scenario: User requests source evidence with implementation mode
- **WHEN** `--for-implementation` is combined with `--with-impl`, `--with-tests`, or `--with-callers`
- **THEN** GDC rejects the incompatible options

#### Scenario: Ready packet is emitted
- **WHEN** the complete contract closure passes readiness validation
- **THEN** the packet includes target behavior, lifecycle, acceptance scenarios, and complete dependency contracts
- **AND** the packet identifies itself as implementation-ready and source-free

### Requirement: Ordinary extraction SHALL remain compatible
GDC SHALL preserve existing schema 1.0 loading and ordinary `gdc extract` behavior when implementation mode is not selected.

#### Scenario: Legacy project runs extract
- **WHEN** a schema 1.0 project invokes `gdc extract <node>` without the new flag
- **THEN** the command produces the existing prompt without requiring implementation-contract metadata
