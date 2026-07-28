## Why

Schema 1.1 can prove that a node and its code dependencies contain a complete
source-free behavioral contract, but it cannot distinguish execution surfaces
or represent non-code contracts and phase-specific approval/evidence gates. A
packet may consequently claim `implementation_ready: true` while a relevant
authority is unresolved, or require publish-only provenance for a headless
implementation.

## What Changes

- Add opt-in schema 1.2 implementation profiles, external contract references,
  and phase-scoped gates.
- Add `gdc preflight <node> --profile <id> --phase <phase>` with a structured,
  fail-closed readiness report.
- Make `extract --for-implementation --profile <id>` emit only a sealed,
  profile-selected contract closure whose implementation-phase gates pass.
- Preserve schema 1.0 and 1.1 behavior.
- Document contract amendment as distinct from production implementation and
  cover the Rand-Defense headless versus Unity-publish boundary with fixtures.

## Capabilities

### New Capabilities

- `profiled-implementation-readiness`: Closed-world implementation profiles,
  external contract closure, phase gates, sealing, and preflight diagnostics.

### Modified Capabilities

- `self-contained-implementation-packets`: Profile-selected schema 1.2 packets
  are emitted only when they are sealed and implementation-permitted.

## Impact

The change affects the node model, strict schema parsing, readiness validation,
contract hashing, extract output, a new preflight command, documentation, and
focused/full regression tests. Existing projects remain compatible.
