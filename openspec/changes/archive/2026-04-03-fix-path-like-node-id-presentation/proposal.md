## Why

Code sync can generate path-like node IDs such as `infra.config.env_lookup_os.osEnvLookup` when duplicate symbols share the same namespace. Those IDs are already canonical collision-resolution identifiers, but the current canonical ID logic prepends `namespace` again unless the ID starts with `namespace.`. That makes `query` display awkward values like `config.infra.config.env_lookup_os.osEnvLookup`.

## What Changes

- Treat path-like synced IDs as already canonical when deriving qualified IDs.
- Preserve existing dotted namespace-prefixed behavior for IDs like `tools.Runtime`.
- Add regression coverage for path-like IDs so query and check outputs stay stable.

## Impact

- Affected code: `internal/node`, `internal/cli/query.go`, related tests.
- User-facing impact: query/show/check output will stop double-qualifying path-like synced IDs.
