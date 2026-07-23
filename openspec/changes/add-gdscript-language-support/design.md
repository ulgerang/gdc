## Overview

GDScript support follows the existing deterministic heuristic parser model.
It reads source text only and does not launch Godot or execute project scripts.

## Parser Boundary

The first implementation treats one `.gd` file as one primary node. A
`class_name` declaration supplies the node ID; scripts without `class_name`
use the file stem. The parser extracts top-level declarations:

- `extends` as an inheritance dependency when it names an application type
- `_init` as a constructor
- public `func` and `static func` declarations
- typed `var` declarations as properties
- `signal` declarations as events
- application types referenced by parameters, returns, properties, and signals

Names beginning with `_` remain outside the public graph contract except
`_init`. Godot built-in and primitive types are excluded from dependency edges.

## Workflow Routing

- `gdscript` and `gd` resolve to the GDScript parser and code generator.
- code sync scans `.gd` files and excludes `test_*.gd` and `*_test.gd`.
- implementation verification recognizes `class_name`, inner `class`, and
  function declarations.
- extraction emits GDScript-shaped class stubs because GDScript has no native
  interface declaration.

## Limitations

The heuristic parser does not attempt full expression parsing, dynamic type
inference, preload/load dependency resolution, nested class body extraction,
or Godot scene/resource analysis. These are explicit follow-up surfaces rather
than silent claims of support.
