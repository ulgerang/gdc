## Why

GDC advertises Python in `init` help and configuration examples, but the parser
orchestrator rejects it and code-driven workflows do not scan `.py` files.
Python projects therefore cannot use the normal `sync --direction code`,
`query`, `check --verify-impl`, or language-specific extraction flows.

## What Changes

- Add Python and the `py` alias as first-class project languages.
- Add a lightweight Python parser for public classes, Protocol/ABC contracts,
  module functions, constructors, methods, properties, and type-hint-based
  dependencies.
- Route Python source files through sync, query, implementation verification,
  extraction, and code generation.
- Exclude conventional Python test files from production code sync while
  retaining the existing Python test discovery support.
- Add parser, routing, integration, and code-generation coverage and update
  user-facing documentation.

## Impact

- Affected code: parser, CLI language routing, code generation, fixtures, and
  language-support documentation.
- User-facing impact: Python projects can initialize GDC with
  `--language python` and use the existing code-driven workflows on `.py`
  source files.
