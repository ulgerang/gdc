---
description: Search and query GDC nodes
agent: build
subtask: false
---

Search and query nodes based on ID, file path, name, or metadata.

## Basic Search

Fuzzy query by node name or ID:
!`gdc query $1`
!`gdc query $1 --verbose`

## Advanced Search (Codebase Patterns)

Search codebase for patterns:
!`gdc search "$1"`
!`gdc search "$1" --file-pattern "*.go" --context 2`

## Find by Provenance

Look for nodes matching a file path or qualified name:
!`gdc query src/path/to/file.go`
