## ADDED Requirements

### Requirement: Canonical IDs SHALL remain stable for dotted synced node IDs
When a node spec already stores a `node.id` that includes the namespace prefix used to avoid collisions, GDC SHALL not prepend the namespace a second time when deriving the canonical ID exposed by query, show, graph, or sync-backed lookups.

#### Scenario: Namespace-prefixed ID remains canonical
- **WHEN** a node spec has `namespace: tools` and `node.id: tools.Runtime`
- **THEN** the canonical ID is `tools.Runtime`
- **THEN** GDC does not emit `tools.tools.Runtime`

### Requirement: Implementation lookup SHALL resolve dotted synced IDs to source symbols
When a synced node stores a dotted `node.id`, GDC SHALL still locate the implementation symbol in source files for diff and implementation verification commands.

#### Scenario: Diff resolves terminal symbol from dotted ID
- **WHEN** a node spec has `node.id: file.CapacityStore` and the parsed source symbol is `CapacityStore`
- **THEN** diff and implementation verification match that extracted symbol
- **THEN** GDC does not fail with a false "symbol not found" error solely because the stored ID is dotted
