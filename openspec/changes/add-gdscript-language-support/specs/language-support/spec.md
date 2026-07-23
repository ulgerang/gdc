## ADDED Requirements

### Requirement: GDScript projects SHALL be supported as a primary language
GDC SHALL accept `gdscript` and `gd` anywhere the CLI routes language-specific
behavior.

#### Scenario: Parser orchestrator returns a GDScript parser
- **WHEN** the parser orchestrator is asked for `gdscript` or `gd`
- **THEN** it returns a parser whose language is `gdscript`

### Requirement: Code-driven workflows SHALL recognize GDScript source files
GDC SHALL treat production `.gd` scripts as inputs to code extraction, query
hints, implementation verification, and extraction evidence.

#### Scenario: Code sync excludes GDScript tests
- **WHEN** code sync scans a GDScript source tree
- **THEN** it extracts production `.gd` files
- **AND** excludes files named `test_*.gd` or `*_test.gd`

#### Scenario: Implementation verification finds GDScript symbols
- **WHEN** `gdc check --verify-impl` verifies a node backed by a `.gd` file
- **THEN** `class_name`, inner class, and function declarations can satisfy
  symbol lookup

### Requirement: GDC SHALL extract public GDScript contracts
GDC SHALL extract a script node with its constructor, public methods, typed
properties, signals, and conservative application-type dependencies.

#### Scenario: Parser extracts a Godot script contract
- **WHEN** a `.gd` file declares `class_name`, `extends`, typed variables,
  signals, `_init`, and typed public methods
- **THEN** the extracted node preserves those public contract members
- **AND** excludes private methods and Godot built-in types from dependencies

### Requirement: Extraction SHALL render GDScript-shaped contracts
GDC SHALL render GDScript class stubs for GDScript projects.

#### Scenario: Extract renders a GDScript class
- **WHEN** an extracted GDScript node is rendered for implementation context
- **THEN** the contract uses `class_name`, `extends RefCounted`, signals,
  variables, and function stubs
