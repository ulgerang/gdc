## ADDED Requirements

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
