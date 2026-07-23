## MODIFIED Requirements

### Requirement: GDC SHALL extract public GDScript contracts
GDC SHALL extract a script node with its constructor, public methods, typed
properties, signals, conservative application-type dependencies, and members
whose declarations follow one or more leading GDScript annotations.

#### Scenario: Parser keeps inline annotated declarations
- **WHEN** a `.gd` file declares `@export var`, `@onready var`, or
  `@rpc(...) func` on one line
- **THEN** the parser extracts the underlying property or method
- **AND** annotation arguments do not corrupt signature boundaries

#### Scenario: Annotation-only lines preserve following declarations
- **WHEN** an annotation occupies its own line before a declaration
- **THEN** the parser continues to extract the following declaration
- **AND** pending documentation remains attached to that declaration
