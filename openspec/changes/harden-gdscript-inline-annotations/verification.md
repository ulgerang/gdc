# Verification

- Parser regression first failed by extracting zero inline annotated properties.
- Focused parser/codegen/CLI tests: PASS.
- Integration tests: PASS.
- `go vet ./...`: PASS.
- `openspec validate harden-gdscript-inline-annotations --strict`: PASS.
- `gdc check --verify-impl --no-orphan-info`: PASS with zero findings.
- `GDScriptParser`, `stripLeadingGDScriptAnnotations`, and
  `findGDScriptAnnotationEnd` nodes: `has_drift: false`.
- Installed CLI smoke extracted `@export_range var`, `@onready var`, and
  `@rpc(...) func`, then passed implementation verification with zero findings.

## Installed binary

- Version: `1.0.0-dev+8f571a9b71f7.gdscript-hardened.2`
- Build time: `2026-07-23T10:10:15Z`
- SHA-256: `2A52AF70936FC14215C7674DD116741D422515D7026A4995DF8B65EC6338027C`
- Installed at `D:\git\26\gdc\gdc.exe` and `C:\bin\gdc.exe`.
- Previous binary retained at
  `D:\w\gdc-repo-before-inline-annotations-20260723.exe` and
  `C:\bin\gdc.exe.backup-before-inline-annotations-20260723`.
