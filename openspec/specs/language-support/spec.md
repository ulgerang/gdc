# language-support Specification

## Purpose
TBD - created by archiving change add-rust-language-support. Update Purpose after archive.
## Requirements
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

### Requirement: Rust parser SHALL only skip actual receiver parameters
The Rust parser SHALL skip Rust receiver parameters without dropping ordinary parameters whose names or types contain the string `self`.

#### Scenario: Parameter name contains self
- **WHEN** a Rust method has a receiver and a parameter named `self_user`
- **THEN** parser extraction keeps `self_user`
- **AND** dependency extraction includes the parameter type

#### Scenario: Receiver uses lifetime or typed receiver syntax
- **WHEN** a Rust method receiver is written as `&'a mut self` or `self: Box<Self>`
- **THEN** parser extraction skips the receiver itself
- **AND** keeps the remaining ordinary parameters
