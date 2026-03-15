---
description: Extract AI implementation prompt for a GDC node
agent: build
subtask: false
---

Extract focused implementation context for node `$1`.

## Extract Prompt

!`gdc extract $1`

## Usage

The extracted prompt above contains:
1. **Target Node Specification** - The YAML spec of the node to implement
2. **Dependency Specifications** - YAML specs of all dependencies
3. **Implementation Guidelines** - Instructions for correct implementation

## Next Steps

Use the extracted context to implement the node. The prompt is designed to give the AI:
- Minimal but sufficient context
- Exact interface signatures to follow
- Clear understanding of dependencies

You can also run the command with optional evidence if required:
!`gdc extract $1 --with-impl`
!`gdc extract $1 --with-impl --with-tests --with-callers`

Would you like me to proceed with implementing `$1` based on this specification?
