## 1. Versioned contract schema

- [x] 1.1 Add failing tests for schema 1.1 strict-field rejection and schema 1.0 compatibility
- [x] 1.2 Extend the node model with behavior, dependency-member, lifecycle, and acceptance contract fields
- [x] 1.3 Update the documented node schema and examples to match the executable model

## 2. Implementation readiness

- [x] 2.1 Add failing tests for placeholders, incomplete methods, missing acceptance, unresolved dependencies, missing required members, and stale hashes
- [x] 2.2 Implement deterministic target and dependency-closure readiness validation

## 3. Source-free implementation packets

- [x] 3.1 Add failing extract tests for complete transitive contracts and incompatible source-evidence flags
- [x] 3.2 Implement `extract --for-implementation` with full lossless closure output in text and JSON
- [x] 3.3 Document the implementation-ready workflow and backward-compatible ordinary extract behavior

## 4. Verification and reconciliation

- [x] 4.1 Run focused tests, full tests, race tests, vet, and formatting checks
- [x] 4.2 Validate OpenSpec strictly and reconcile touched GDC nodes with implementation
- [x] 4.3 Prove a temporary Rand-Defense-style contract fixture emits a source-free ready packet and rejects an incomplete variant

## 5. Post-implementation verification hardening

- [x] 5.1 Treat schema 1.1 module `interface.types` as concrete implementation symbols during `--verify-impl`
- [x] 5.2 Select named C# generic types, preserve nested generic and multiline signatures, and match exact overloads
- [x] 5.3 Run focused/full tests, strict OpenSpec validation, and the live Rand Defense verification probe
- [x] 5.4 Reuse module implementation binding in `gdc diff`
- [x] 5.5 Preserve exact YAML stems and resolve unambiguous kebab-case node IDs in `extract`
- [x] 5.6 Run full regression, strict OpenSpec, and both live Rand Defense resume-condition probes
