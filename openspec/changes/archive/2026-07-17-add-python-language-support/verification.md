# Verification

## Scenario Evidence

| Scenario | Evidence |
|---|---|
| Initialize a Python project | Fresh CLI smoke persisted `language: python` in `.gdc/config.yaml`. |
| Query searches Python source extensions | `TestSourceExtensionsForLanguageIncludesPython` passes. |
| Code sync excludes Python tests | Fresh CLI smoke copied the same fixture to `src/services.py` and `src/test_ignored.py`; sync scanned one source file and produced four nodes. `TestIsTestSourceFileRecognizesPythonConventions` covers both naming forms. |
| Implementation verification finds Python symbols | `TestSymbolExistsInSourceSupportsPython` passes and the fresh project passed `gdc check --verify-impl --no-orphan-info`. |
| Sync extracts multiple nodes | The fixture produced `AuditSink`, `build_user_service`, `UserRepository`, and `UserService`. Parser unit and integration tests assert the four-node result. |
| Parser extracts class contracts | `TestPythonParserExtractsPublicNodesAndClassContracts` and `TestPythonParserHandlesDecoratorsAndMultilineTypeHints` cover constructors, async/class/static methods, properties/setters, annotated fields, privacy, and dependencies. |
| Parser orchestrator returns Python parser | Unit and integration orchestrator tests pass for `python` and `py`. |
| Extract renders a Protocol contract | Fresh CLI smoke verified `gdc extract UserRepository` contains `class UserRepository(Protocol):`; `TestGenerateInterfacePython` covers code generation. |

## Commands

- `go test -race ./... -count=1`
- `go vet ./...`
- `go build ./...`
- `git diff --check`
- `openspec validate add-python-language-support --strict`
- `gdc diff PythonParser`
- `gdc check --verify-impl --no-orphan-info --exit-on-warning`

All commands passed on 2026-07-14.
