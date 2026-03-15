---
description: Synchronize GDC YAML specs with codebase or database
agent: build
subtask: false
---

Synchronize the GDC graph.

## Direction: $1

### If direction is "yaml" or empty (default)

Sync YAML specifications to the SQLite database:

!`gdc sync --direction yaml`

### If direction is "code"

Extract interfaces from source code and generate/update YAML specs:

!`gdc sync --direction code`

You can also specify target sources to extract specific specs:
!`gdc sync --direction code --source src/`
!`gdc sync --direction code --files src/services/user_service.go`
!`gdc sync --direction code --dirs src/services --symbols UserService`

## Validation

After synchronization, validate the graph:

!`gdc check`

## Result Summary

Report:
1. Number of nodes added/updated/removed
2. Any validation errors or warnings
3. Suggestions for resolving issues (if any)
