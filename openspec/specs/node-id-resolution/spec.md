# node-id-resolution Specification

## Purpose
Define how GDC interprets canonical node IDs and source-symbol lookup when code sync emits dotted node identifiers to disambiguate collisions.
## Requirements
### Requirement: Canonical IDs SHALL remain stable for dotted synced node IDs
When a node spec already stores a `node.id` that includes the namespace prefix used to avoid collisions, GDC SHALL not prepend the namespace a second time when deriving the canonical ID exposed by query, show, graph, or sync-backed lookups.

#### Scenario: Namespace-prefixed ID remains canonical
- **WHEN** a node spec has `namespace: tools` and `node.id: tools.Runtime`
- **THEN** the canonical ID is `tools.Runtime`
- **THEN** GDC does not emit `tools.tools.Runtime`

#### Scenario: Path-like synced ID remains canonical
- **WHEN** a node spec has `namespace: config` and `node.id: infra.config.env_lookup_os.osEnvLookup`
- **THEN** the canonical ID is `infra.config.env_lookup_os.osEnvLookup`
- **THEN** `query`, `show`, and `check` do not emit `config.infra.config.env_lookup_os.osEnvLookup`

### Requirement: Implementation lookup SHALL resolve dotted synced IDs to source symbols
When a synced node stores a dotted `node.id`, GDC SHALL still locate the implementation symbol in source files for diff and implementation verification commands.

#### Scenario: Diff resolves terminal symbol from dotted ID
- **WHEN** a node spec has `node.id: file.CapacityStore` and the parsed source symbol is `CapacityStore`
- **THEN** diff and implementation verification match that extracted symbol
- **THEN** GDC does not fail with a false "symbol not found" error solely because the stored ID is dotted

### Requirement: Commands SHALL resolve unique short dependency targets to canonical nodes
When a dependency target uses a short node ID and exactly one matching node exists in the project graph, GDC commands SHALL resolve that target to the canonical node ID for traversal and graph output.

#### Scenario: Trace resolves a namespaced dependency referenced by short ID
- **WHEN** `Game.Controllers.PlayerController` depends on `IInputManager`
- **AND** the graph contains a unique `Game.Input.IInputManager` node
- **THEN** `gdc trace Game.Controllers.PlayerController` treats the dependency as `Game.Input.IInputManager`
- **AND** the dependency is not reported as missing

#### Scenario: Graph export resolves a namespaced dependency referenced by short ID
- **WHEN** a graph edge references `IInputManager`
- **AND** the graph contains a unique `Game.Input.IInputManager` node
- **THEN** `gdc graph` emits the edge to `Game.Input.IInputManager`
- **AND** the export does not create a separate orphan `IInputManager` graph node

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
