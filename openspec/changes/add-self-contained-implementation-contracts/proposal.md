## Why

GDC describes itself as a design-first source of implementation context, but schema fields documented for behavioral contracts are currently dropped by the runtime model and dependency extraction reduces resolved nodes to interface signatures. An agent can therefore receive a structurally valid packet that is not sufficient to implement the target without reopening source code.

## What Changes

- Add an opt-in schema 1.1 implementation contract that records public type contracts, lifecycle constraints, and executable acceptance scenarios alongside method preconditions, postconditions, effects, examples, rules, and exact dependency-member requirements.
- Parse schema 1.1 node files strictly so unknown or misspelled contract fields fail instead of disappearing silently; preserve permissive loading for existing schema 1.0 files.
- Add implementation-readiness validation for unresolved placeholders, incomplete behavioral contracts, missing dependencies, stale dependency hashes, and unavailable required dependency members.
- Add `gdc extract <node> --for-implementation` to produce a source-free, transitive, lossless dependency-contract packet and fail when the contract closure is not implementation-ready.
- Make `gdc check --verify-impl` honor schema 1.1 module type contracts as the module's concrete implementation symbols and select exact C# generic types and overloads from multi-type source files.
- Preserve existing extract, sync, query, and verification behavior unless the new mode or schema is selected.

## Capabilities

### New Capabilities
- `self-contained-implementation-packets`: Defines strict implementation-ready node contracts and source-free dependency-closure packets.

### Modified Capabilities
- `design-first-workflow`: Clarifies that GDC may claim a packet is sufficient for implementation only after strict contract-readiness validation succeeds.

## Impact

The change affects node schema parsing and validation, extract dependency assembly and output, CLI flags and documentation, and focused Go/OpenSpec tests. Existing schema 1.0 repositories and ordinary `gdc extract` output remain compatible.
