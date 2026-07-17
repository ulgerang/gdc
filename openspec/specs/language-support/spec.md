# language-support Specification

## Purpose
Define the languages GDC accepts and the parsing, discovery, verification, and contract-generation behavior each supported language provides.
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

### Requirement: Python projects SHALL be supported as a primary language
GDC SHALL accept Python as a valid project language anywhere the CLI routes
language-specific behavior.

#### Scenario: Initialize a Python project
- **WHEN** a user runs `gdc init --language python`
- **THEN** the generated configuration stores `project.language: python`
- **AND** the documented CLI help lists Python as a supported language

### Requirement: Code-driven workflows SHALL recognize Python source files
GDC SHALL treat Python source files as valid inputs for code extraction, query
hints, implementation verification, and extraction evidence.

#### Scenario: Query searches Python source extensions
- **WHEN** a project is configured with `project.language: python`
- **THEN** source probing uses `.py` files

#### Scenario: Code sync excludes Python tests
- **WHEN** code sync scans a Python source tree
- **THEN** it extracts production `.py` files
- **AND** excludes files named `test_*.py` or `*_test.py`

#### Scenario: Implementation verification finds Python symbols
- **WHEN** `gdc check --verify-impl` verifies a node backed by a Python file
- **THEN** Python class and function declarations can satisfy symbol lookup

### Requirement: GDC SHALL extract public Python nodes from code
GDC SHALL parse public Python declarations into extracted node specifications.

#### Scenario: Sync extracts multiple nodes from a Python module
- **WHEN** a Python module contains public classes, Protocol or ABC contracts,
  and public module functions
- **THEN** code sync can extract each public declaration from that module

#### Scenario: Parser extracts class contracts
- **WHEN** a public Python class declares `__init__`, public methods,
  `@property` getters, annotated fields, and type hints
- **THEN** GDC extracts the constructor, methods, properties, and conservative
  application-type dependencies

#### Scenario: Parser orchestrator returns a Python parser
- **WHEN** the parser orchestrator is asked for `python` or `py`
- **THEN** it returns a parser whose language is `python`

### Requirement: Extraction SHALL render Python-shaped contracts
GDC SHALL generate Python-shaped interface or class stubs for Python projects.

#### Scenario: Extract renders a Protocol contract
- **WHEN** a Python interface node is rendered for an implementation packet
- **THEN** the generated contract uses a `Protocol` class with Python method stubs
