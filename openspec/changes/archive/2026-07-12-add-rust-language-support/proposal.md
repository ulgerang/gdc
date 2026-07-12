## Why

GDC currently supports Go, C#, and TypeScript parsing, but Rust projects cannot use the normal `init`, `sync --direction code`, `query`, `check --verify-impl`, or `extract` flows with `project.language: rust`.

That prevents teams using Rust from adopting GDC as their graph-driven source-of-truth workflow, even though the rest of the CLI is language-agnostic.

## What Changes

- Add Rust as a first-class project language for CLI configuration and language-specific helpers.
- Implement a Rust parser that can extract public structs, enums, traits, and their public methods from `.rs` files.
- Extend code sync, query, implementation verification, extract helpers, and test discovery to recognize Rust files and conventions.
- Add regression coverage and fixtures for Rust parsing and end-to-end language routing.
- Update user-facing documentation to list Rust as a supported language.

## Impact

- Affected code: parser package, CLI language routing, extract/codegen helpers, fixtures, and docs.
- User-facing impact: Rust projects can initialize GDC with `--language rust` and use the existing code-driven workflows without unsupported-language errors.
