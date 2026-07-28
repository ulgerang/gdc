## Context

GDC currently treats `implementation_contract.status: ready` as both contract
completeness and implementation permission. That is sufficient for a closed
code-dependency graph, but not for nodes with multiple build surfaces or
approval, evidence, and provenance contracts whose relevance changes by phase.

## Goals / Non-Goals

**Goals:**

- Detect missing profile selection, incomplete profile behavior, stale external
  contracts, and unresolved relevant gates before production implementation.
- Keep implementation packets source-free while embedding every selected
  non-code contract needed to implement the chosen profile.
- Separate contract completeness from permission to implement, verify, or
  publish.
- Keep project-specific names such as Gate A and Unity out of GDC core logic.

**Non-Goals:**

- Infer requirements that were never authored.
- Execute arbitrary approval systems or acceptance commands.
- Require source commits or implementation hashes for ordinary API contracts.
- Replace OpenSpec as product-intent authority.

## Decisions

1. **Use schema 1.2 as an opt-in closed-world contract.** Schema 1.1 remains
   accepted by the existing implementation extraction behavior. Schema 1.2
   adds profiles, external contracts, gates, and `sealed` status.

2. **Require explicit profile selection when multiple profiles exist.** A sole
   profile may be selected implicitly. A declared default is not supported:
   explicitness prevents silently choosing a weaker surface.

3. **Represent external contracts as hashed, repository-relative authored
   inputs.** Sealed packets require each selected contract to exist and match
   its raw-byte SHA-256. The packet embeds the bytes as text. No source commit is
   required.

4. **Scope gates by phase and profile.** `contract` and the selected requested
   phase are blocking. Later-phase gates do not block earlier work. Gate kinds
   are generic (`approval`, `evidence`, `provenance`, `policy`, `manual`).

5. **Separate readiness axes.** Preflight reports contract, dependency,
   external-contract, and gate completeness plus the final permission decision,
   selected profile, missing items, and blockers.

6. **Seal before implementation.** Schema 1.2 implementation extraction
   requires `status: sealed`; `ready` remains amendable and can be inspected by
   preflight without authorizing production code.

7. **Keep feedback application out of this slice.** Machine-readable preflight
   diagnostics provide the stable amendment input. Automatic mutation can be
   added later without weakening the authority boundary.

## Risks / Trade-offs

- External contract hashes require deliberate refresh after contract edits;
  diagnostics report the observed replacement hash.
- A sealed contract can still omit an unknown business requirement. OpenSpec
  review and requirement coverage remain necessary before sealing.
- Profile metadata is verbose, but only schema 1.2 users pay that cost.

## Migration

1. Existing schema 1.0/1.1 projects continue unchanged.
2. Multi-surface nodes migrate to schema 1.2 and add profiles, gates, and
   external contract references.
3. Run preflight while status is `ready`; resolve diagnostics and independently
   review the contract.
4. Change status to `sealed`, refresh reviewed hashes, and extract a selected
   profile packet.
