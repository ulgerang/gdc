## Why

Godot commonly places annotations and declarations on the same line, such as
`@export var`, `@onready var`, and `@rpc(...) func`. The initial GDScript parser
treats every line beginning with `@` as annotation-only and silently omits the
following inline declaration from the graph contract.

## What Changes

- Recognize one or more leading GDScript annotations while preserving an
  inline variable, signal, or function declaration.
- Handle annotation arguments with nested parentheses and quoted strings.
- Preserve the existing behavior for annotation-only lines.
- Add regression coverage for exported/onready properties and RPC methods.

## Impact

- Affected code: GDScript parser and its touched GDC nodes.
- User-facing impact: code sync no longer loses common Godot annotated members.
