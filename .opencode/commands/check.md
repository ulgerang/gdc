---
description: Validate GDC graph consistency and architecture rules
agent: plan
subtask: true
---

Validate the GDC graph for consistency and architecture violations.

## Run Validation

!`gdc check`

## Interpretation Guide

### Error Categories

| Category | Severity | Description |
|----------|----------|-------------|
| `missing_ref` | ERROR | Dependency target doesn't exist |
| `cycle` | ERROR | Circular dependency detected |
| `layer_violation` | WARNING | Architecture layer rule broken |
| `hash_mismatch` | WARNING | Contract changed without update |
| `orphan` | INFO | Node not referenced by anything |
| `srp_violation` | WARNING | Too many dependencies |

## Resolution Suggestions

For each issue found, suggest:

1. **For `missing_ref`**: Create the missing node or fix the typo
2. **For `cycle`**: Introduce an interface to break the cycle
3. **For `layer_violation`**: Move the node to the correct layer or refactor dependency
4. **For `orphan`**: Consider if the node is truly unused or should be connected

## Graph Statistics

!`gdc stats`

Provide a health summary of the codebase graph.
