# cli-build Specification

## Purpose
Define how documented GDC build targets map to the real CLI entrypoints and version smoke checks.

## Requirements
### Requirement: Repository root builds SHALL produce the CLI
When a user runs `go build` against the repository root, the resulting binary SHALL execute the real GDC CLI instead of a debug-only sample program.

#### Scenario: Root build runs the CLI
- **WHEN** the repository root is built as a Go main package
- **THEN** the resulting binary prints CLI version output for `version`
- **AND** the binary does not execute the debug sample parser flow

### Requirement: Documented CLI build target SHALL exist
The repository SHALL provide a working `./cmd/gdc` entrypoint for documented builds.

#### Scenario: cmd/gdc build runs the CLI
- **WHEN** a user runs `go build ./cmd/gdc`
- **THEN** the build succeeds
- **AND** the resulting binary executes the GDC CLI

### Requirement: CLI version flag SHALL be available
The root command SHALL support a version smoke check without requiring the `version` subcommand.

#### Scenario: Root command prints version
- **WHEN** a user runs `gdc --version`
- **THEN** the CLI prints version information
