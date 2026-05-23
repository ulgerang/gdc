## ADDED Requirements

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
