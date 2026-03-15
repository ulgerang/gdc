---
description: Create a new GDC node with YAML specification
agent: build
subtask: false
---

Create a new GDC node named `$1` with type `$2` in layer `$3`.

## Step 1: Create Node Skeleton

!`gdc node create $1 --type $2 --layer $3`

## Step 2: Edit Specification

Open and edit the generated YAML file at `.gdc/nodes/$1.yaml`.

The specification should include:
- Clear `responsibility.summary`
- Well-defined `interface.methods` with proper signatures
- Appropriate `dependencies` if any
- Correct `metadata.status` (start with "draft" or "specified")

## Step 3: @spec-architect Review

Consider invoking the spec-architect agent to review the specification:

```
@spec-architect Review the specification for $1
```

## Step 4: Sync to Database

After the specification is complete:

!`gdc sync`

## Step 5: Validate

!`gdc check`

## Template for $2 Type

### If interface:
```yaml
interface:
  methods:
    - name: "MethodName"
      signature: "MethodName(params) ReturnType"
      description: "..."
```

### If class/service:
```yaml
interface:
  constructors:
    - signature: "New$1(deps) *$1"
      description: "..."
  methods:
    - name: "Execute"
      signature: "..."
dependencies:
  - target: "IDependencyName"
    type: "interface"
    injection: "constructor"
```
