## ADDED Requirements

### Requirement: Schema 1.2 SHALL declare closed implementation profiles

GDC SHALL preserve profile identifiers, descriptions, required conditions,
forbidden conditions, and referenced acceptance scenarios in schema 1.2 nodes.

#### Scenario: Multiple profiles are available

- **WHEN** implementation preflight or extraction is requested without a profile
- **THEN** GDC fails closed and lists the available profile identifiers

#### Scenario: One profile is available

- **WHEN** implementation preflight is requested without a profile
- **THEN** GDC selects the sole profile deterministically

### Requirement: Schema 1.2 SHALL close selected external contracts

A sealed profile SHALL identify every required non-code contract by a
repository-relative path and reviewed raw-byte SHA-256.

#### Scenario: External contract hash is stale

- **WHEN** the selected external contract bytes do not match the reviewed hash
- **THEN** preflight fails and reports the observed replacement hash

#### Scenario: Selected packet is emitted

- **WHEN** every selected external contract exists and matches
- **THEN** the source-free packet embeds its path, hash, and authored text

### Requirement: Gates SHALL be scoped by profile and phase

GDC SHALL evaluate only contract-phase gates and gates that apply to the
requested profile and phase.

#### Scenario: Publish provenance is irrelevant to headless implementation

- **WHEN** a blocked provenance gate applies only to a different profile or the
  publish phase
- **THEN** it does not block headless implementation preflight

#### Scenario: Relevant implementation gate is blocked

- **WHEN** a blocked gate applies to the selected profile and implementation
  phase
- **THEN** implementation permission is false and the gate appears in
  `blocked_by`

### Requirement: Preflight SHALL separate readiness axes

`gdc preflight` SHALL report contract completeness, dependency closure,
external contract closure, gate satisfaction, sealing, and final phase
permission without reading implementation source.

#### Scenario: Contract is complete but not sealed

- **WHEN** a schema 1.2 contract has complete profiles and dependencies but
  `status: ready`
- **THEN** contract completeness is true
- **AND** implementation permission is false
- **AND** the report identifies sealing as the blocker

### Requirement: Schema 1.2 extraction SHALL be sealed and profile-selected

`extract --for-implementation` SHALL refuse a schema 1.2 node unless the
selected profile is complete, the contract is sealed, external contracts close,
and relevant contract/implementation gates are satisfied.

#### Scenario: Sealed headless packet is ready

- **WHEN** the headless profile is complete and no relevant gate is unresolved
- **THEN** extraction emits an implementation-ready source-free packet
- **AND** publish-only contracts and gates are absent
