## ADDED Requirements

### Requirement: Rust projects SHALL be supported as a primary language
GDC SHALL accept Rust as a valid project language anywhere the CLI routes language-specific behavior.

#### Scenario: Initialize a Rust project
- **WHEN** a user runs `gdc init --language rust`
- **THEN** the generated configuration stores `project.language: rust`
- **AND** the documented CLI help lists Rust as a supported language

### Requirement: Code-driven workflows SHALL recognize Rust source files
GDC SHALL treat Rust source files as valid inputs for code extraction, query hints, and implementation verification.

#### Scenario: Query searches Rust source extensions
- **WHEN** a project is configured with `project.language: rust`
- **THEN** source probing uses `.rs` files

#### Scenario: Implementation verification finds Rust symbols
- **WHEN** `gdc check --verify-impl` verifies a node backed by a Rust file
- **THEN** Rust declarations and functions can satisfy the symbol lookup

### Requirement: GDC SHALL extract public Rust nodes from code
GDC SHALL parse public Rust items into extracted node specifications for code sync and parser-driven workflows.

#### Scenario: Sync extracts multiple nodes from a Rust file
- **WHEN** a Rust source file contains multiple public traits, structs, enums, or functions
- **THEN** code sync can extract each public node from that file

#### Scenario: Parser orchestrator returns a Rust parser
- **WHEN** the parser orchestrator is asked for `rust` or `rs`
- **THEN** it returns a parser whose language is `rust`
