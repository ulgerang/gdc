## Context

The documented node schema already presents method preconditions, postconditions, examples, return constraints, and logic rules, but the Go model does not retain all of them. `yaml.Unmarshal` also ignores unknown fields, so a misspelled or unsupported behavior field can disappear without invalidating the node. During extraction, resolved dependency nodes are reduced primarily to generated interface code and, in JSON, method-signature strings. The resulting prompt says to use only the supplied context even when that context is behaviorally incomplete.

The implementation must preserve existing schema 1.0 repositories and normal extraction while creating an explicit, fail-closed path for users who want a source-free implementation packet.

## Goals / Non-Goals

**Goals:**

- Represent public DTO/record/enum contracts, behavior, lifecycle, effects, exact dependency-member usage, and acceptance scenarios required to implement a node.
- Detect unknown schema 1.1 fields, unresolved placeholders, missing behavioral contracts, missing dependencies, stale contract hashes, and nonexistent required dependency members before emitting an implementation packet.
- Emit the complete transitive dependency contract closure without implementation source, tests, callers, or other repository context.
- Keep schema 1.0 loading and ordinary `gdc extract` behavior compatible.

**Non-Goals:**

- Prove arbitrary business intent from structure alone.
- Embed dependency implementation source or pin implementation commits as a code-generation requirement.
- Execute acceptance commands or replace compilation and runtime tests after implementation.
- Make readiness checks a repository-wide default gate for legacy nodes.

## Decisions

1. **Use an explicit schema 1.1 readiness contract.** A root `implementation_contract` section carries `status`, lifecycle and global constraints, plus structured given/when/then acceptance scenarios. Existing 1.0 nodes remain permissive and are never implicitly called implementation-ready.

2. **Retain behavior at the method and logic levels.** Methods gain `preconditions`, `postconditions`, and `side_effects`; parameters gain examples; returns gain constraints; logic gains rules. Dependency edges gain `requires`, naming the exact public members the target consumes.

3. **Make strict parsing versioned.** `node.Load` first reads only `schema_version`. Version 1.1 uses `yaml.Decoder.KnownFields(true)` and rejects unknown fields; 1.0 keeps the current permissive loader for compatibility. Strictness is therefore chosen by the authored contract, not by global repository state.

4. **Validate readiness at packet assembly.** `gdc extract --for-implementation` requires the target and every required dependency in its transitive closure to have a ready implementation contract. It checks placeholders, member documentation and behavior, acceptance scenarios, dependency resolution, exact required members, and contract hashes. Schema 1.1 hashes cover portable node identity, language metadata, behavior, types, logic, readiness criteria, and dependency edge structure. Repository-local file paths and nested `contract_hash` values are excluded so hashes remain portable and computable for cyclic graphs.

5. **Keep the new packet source-free.** `--for-implementation` is incompatible with `--with-impl`, `--with-tests`, and `--with-callers`. It includes full target/dependency contracts and logic automatically. Ordinary extract remains the exploratory mode when repository evidence is intentionally desired.

6. **Reuse `extract` rather than add a parallel packet command.** Extract is already the documented AI-context surface. A mode flag keeps resolution, formatting, language-specific interface generation, and output handling in one command while making the stronger guarantee explicit.

7. **Use module type contracts as implementation-symbol authority.** For a `node.type: module` contract, `interface.types` names the concrete source symbols that must exist in the declared file. Implementation verification aggregates their public members instead of requiring a synthetic source type whose name equals the module node ID. C# verification selects a requested type from multi-type files, preserves nested generic return signatures and multiline declarations, and compares every overload before reporting a member missing.

## Risks / Trade-offs

- **[Ready contracts are more verbose]** → Only nodes opting into schema 1.1 and `implementation_contract.status: ready` pay the authoring cost.
- **[Behavior completeness cannot be mathematically proven]** → Readiness guarantees a concrete, testable contract floor and fail-closed closure, while post-implementation builds and tests remain required.
- **[Contract hashes become cumbersome]** → Reuse GDC's existing canonical spec hash and report the exact expected hash in readiness diagnostics.
- **[Cycles could expand packets indefinitely]** → Traverse by canonical node identity with a visited set and deterministic dependency order.
- **[Language-specific custom YAML fields may break under strict parsing]** → Schema 1.1 accepts only the documented cross-language contract. New language fields must first be added to the model and schema.
- **[Targeted C# extraction is still lexical]** → Keep the surface verification-only, cover multi-type/generic/multiline/overload cases with regression tests, and retain compilation/tests as the behavioral authority.

## Migration Plan

1. Add the 1.1 data model and tests without changing 1.0 behavior.
2. Add readiness validation and lossless closure assembly behind `--for-implementation`.
3. Update schema and user documentation with a complete 1.1 example.
4. Validate against a temporary source-free fixture modeled after the Rand Defense UJ bridge gap.
5. Existing repositories may migrate individual nodes incrementally; rollback is removal of the mode flag and continued use of schema 1.0.

## Open Questions

None for this slice. Executable acceptance-command orchestration can be added separately after the contract-only packet is proven useful.
