## ADDED Requirements

### Requirement: Safe node deletion
The CLI SHALL reject deletion of a node referenced by another node unless the user supplies `--force`, and a forced deletion SHALL remove matching dependency references so the YAML graph remains valid.

#### Scenario: Referenced node deletion is rejected
- **WHEN** a user deletes a node that has one or more reverse dependency references without `--force`
- **THEN** the command fails, reports the referencing nodes, and leaves all node specifications unchanged

#### Scenario: Referenced node is forcibly deleted
- **WHEN** a user deletes a referenced node with `--force`
- **THEN** the node specification and matching dependency references are removed and the derived database is refreshed

### Requirement: Referentially complete node rename
The CLI SHALL rename the node specification, update its node ID, update all dependency targets that resolve to the renamed node, and refresh the derived database as one planned operation.

#### Scenario: Node with dependents is renamed
- **WHEN** a user renames a node referenced by other specifications
- **THEN** the new node file, node ID, dependency references, and database index consistently use the new identity

#### Scenario: Rename planning fails
- **WHEN** the rename target exists or an affected specification cannot be loaded or serialized
- **THEN** the command fails before changing any node specification

### Requirement: Source discovery reports incomplete traversal
The CLI SHALL report filesystem traversal or source read failures that would make query hints or explicitly scoped sync results incomplete.

#### Scenario: Explicit sync directory cannot be traversed
- **WHEN** a requested `sync --dirs` scope cannot be traversed completely
- **THEN** the sync command fails with the affected path and underlying error

#### Scenario: Query fallback scan cannot read project sources
- **WHEN** query fallback discovery encounters a traversal or source read failure
- **THEN** the query command reports the discovery failure instead of presenting partial hints as complete

### Requirement: Development launcher uses current source
Repository development launchers SHALL execute the checked-out source by default and SHALL use a prebuilt executable only through an explicit opt-in.

#### Scenario: Stale executable exists beside launcher
- **WHEN** a developer invokes a repository launcher while an older local executable is present
- **THEN** the launcher executes `cmd/gdc` from current source unless prebuilt mode was explicitly requested
