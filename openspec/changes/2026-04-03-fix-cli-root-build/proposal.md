## Why

Building from the repository root currently produces the debug sample binary instead of the real CLI because `debug_sample.go` is a default `package main` in the root package. The repository documentation and `Makefile` also point to `./cmd/gdc`, but that entrypoint is missing.

This makes common installation flows unreliable and breaks normal commands outside the repository root.

## What Changes

- Exclude the debug sample from default builds.
- Restore explicit CLI entrypoints for both repository-root builds and `./cmd/gdc` builds.
- Support `gdc --version` for smoke verification and common CLI expectations.
- Add regression coverage for both documented build targets.
- Update build documentation and scripts to match the restored entrypoints.

## Impact

- Affected code: root package, `cmd/gdc`, CLI root command, docs, and build scripts.
- User-facing impact: `go build` from the repo root and `go build ./cmd/gdc` both produce the real CLI.
