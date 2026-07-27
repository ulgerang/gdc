## MODIFIED Requirements

### Requirement: GDC SHALL document implementation context retrieval
The README SHALL explain how `gdc query`, `gdc trace`, and ordinary `gdc extract` retrieve dependency-aware exploratory context, and SHALL document `gdc extract --for-implementation` as the only mode that claims a source-free contract packet is sufficient for implementation after readiness validation.

#### Scenario: A new feature needs exploratory context
- **WHEN** a developer or agent prepares to investigate an existing feature
- **THEN** the README points them to GDC node specs and dependency traces before editing code
- **AND** the README explains that implementation and test evidence are opt-in additions to ordinary extraction

#### Scenario: An agent must implement without repository source context
- **WHEN** a developer requests a self-contained implementation packet
- **THEN** the README directs them to `gdc extract <node> --for-implementation`
- **AND** it explains that the command fails unless the complete authored dependency contract is implementation-ready
