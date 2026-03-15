---
description: Implement a node from its GDC YAML specification
agent: build
subtask: false
---

Implement the node `$1` based on its GDC YAML specification.

## Step 1: Read Specification

Read the YAML spec from `.gdc/nodes/$1.yaml` and understand:
- The node's responsibility
- Interface methods and their signatures
- Required dependencies

## Step 2: Trace Dependencies

!`gdc trace $1 --depth 1`

## Step 3: Extract Implementation Context

!`gdc extract $1`

## Step 4: Implementation Guidelines

Based on the specification above, implement the code following these rules:

1. **Exact Signatures**: Implement methods exactly as defined in the YAML signature
2. **Dependency Injection**: Inject all dependencies via constructor as specified
3. **Responsibility Comment**: Include the `responsibility.summary` as a doc comment
4. **Error Handling**: Handle all error cases listed in `throws`
5. **File Location**: Place the file at the path specified in `node.file_path` (or derive from namespace)

## Step 5: Post-Implementation Sync

After implementation is complete, synchronize and validate:

!`gdc sync --direction yaml`
!`gdc check`

## Step 6: Update Status

Update the YAML spec's `metadata.status` from `specified` to `implemented`.
