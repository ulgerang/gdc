# Verification

## Deterministic gates

- `go test -count=1 ./internal/parser ./internal/codegen ./internal/cli ./tests/integration`: PASS
- remaining package tests for root, database, extraction, and node packages: PASS
- `go vet ./...`: PASS
- `openspec validate add-gdscript-language-support --strict`: PASS
- `git diff --check`: PASS; existing Windows line-ending warnings only
- `gdc diff GDScriptParser --json`: `has_drift: false`
- GDScript codegen helper and `gatherDependencies` nodes: `has_drift: false`
- repository `gdc check --verify-impl --json`: PASS with informational orphan findings only

## GDScript CLI smoke

A clean three-script Godot fixture completed:

1. `gdc init --language gdscript`
2. `gdc sync --direction code --source .`
3. `gdc sync`
4. `gdc extract SlotController --template implement`
5. `gdc check --verify-impl --json`

The graph contained `SlotController`, `ReelBoard`, and `SpinResult`; extraction
rendered the target and dependency contracts as GDScript; the final check
reported zero errors, zero warnings, and `PASS`.

## Installed binaries

- Version: `1.0.0-dev+8f571a9b71f7.gdscript-hardened`
- Build time: `2026-07-23T10:01:00Z`
- SHA-256: `52F753B80DFE28F0CBCF9536CF414FA3D3C200A40F6FA00D875592D810B381C7`
- Repository binary: `D:\git\26\gdc\gdc.exe`
- Installed binary: `C:\bin\gdc.exe`
- Previous binary SHA-256: `F4CE5DE9F81EC70CD14F42671D663509C86EFC551F1FA8A1030527BC43B603B2`
- Rollback copies: `D:\w\gdc-repo-before-gdscript-20260723.exe` and
  `C:\bin\gdc.exe.backup-before-gdscript-20260723`

## Hardening findings closed

- Parser and codegen helpers now use distinct GDC node IDs, preventing a
  filename-level graph-node collision.
- Top-level function nodes preserve their own contract even when the Go
  function is unexported, preventing immediate `extra_methods` drift after
  code sync.
- The pre-hardening GDScript binary is retained at
  `D:\w\gdc-repo-before-gdscript-hardening-20260723.exe` and
  `C:\bin\gdc.exe.backup-before-gdscript-hardening-20260723`.
