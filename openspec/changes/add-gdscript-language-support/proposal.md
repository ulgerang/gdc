## Why

GDC recognizes `.gd` files as query hints, but code-driven workflows reject
`project.language: gdscript` because no parser, sync routing, implementation
verification, or contract generation path exists. Godot projects therefore
cannot use the same touched-scope structural workflow as the other supported
languages.

## What Changes

- Add `gdscript` and `gd` as first-class language aliases.
- Parse Godot 4 `class_name`, `extends`, typed variables, signals, constructors,
  methods, return types, and conservative application-type dependencies.
- Route `.gd` files through code sync, implementation verification, extraction,
  and contract generation while excluding conventional GDScript test files.
- Add focused parser/routing/codegen tests and a real `init -> sync -> check`
  CLI smoke.

## Impact

- Affected code: parser, CLI language routing, implementation verification,
  code generation, fixtures, and language-support contracts.
- User-facing impact: Godot projects can configure `project.language: gdscript`
  and use GDC code-driven workflows on production `.gd` scripts.
