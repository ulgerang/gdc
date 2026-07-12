## Overview

Rust support should follow the same lightweight model as the existing regex/default parsers for C# and TypeScript:

- `project.language: rust` becomes a valid language choice.
- Rust source discovery uses `.rs` files.
- Code sync extracts named nodes from Rust source into YAML specs.
- Validation and extract helpers understand Rust naming and file conventions.

## Parser Strategy

Use a regex- and line-oriented Rust parser for the initial implementation rather than adding a heavy AST dependency.

The first slice should support:

- `pub struct`, `pub enum`, and `pub trait` declarations
- `impl Type` blocks for inherent methods
- `impl Trait for Type` blocks for trait implementations
- `pub fn` methods and free functions
- constructor-style detection for `new(...) -> Self|Type`

This matches the current project posture where Go has AST parsing but other non-Go languages are supported with lighter parsing strategies.

## Multi-Node Extraction

Rust files often contain multiple public items. The Rust parser should therefore implement `MultiNodeParser` so `sync --direction code` and `check --verify-impl` can resolve the correct node within a file.

For backward compatibility, `ParseFile` should still return the first extracted node.

## Dependency Heuristics

The parser should keep dependency detection conservative:

- treat trait-like or service-like parameter types as dependencies
- ignore obvious primitive/std wrapper types
- normalize references such as `&Type`, `Arc<Type>`, `Option<Type>`, and `Result<T, E>` to the underlying named type when possible

This keeps Rust support aligned with the existing heuristic approach used by the TypeScript and C# parsers.
