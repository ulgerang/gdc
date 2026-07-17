## Overview

Python support follows GDC's existing lightweight parser model. It does not
execute imported project code or require a Python runtime; source is inspected
with deterministic line, indentation, and signature heuristics.

## Parser Strategy

The first slice extracts:

- public top-level classes and functions
- `Protocol` and `ABC` classes as interface nodes
- `__init__` as a constructor
- public instance, class, static, and async methods
- `@property` getters as public properties
- annotated public class fields
- dependencies from base classes and parameter/property type hints

Private names beginning with `_` are excluded, except `__init__`. The parser
implements `MultiNodeParser`; `ParseFile` retains backward compatibility by
returning the first extracted node.

## Type Hint Heuristics

Dependency extraction unwraps common typing containers and keeps named
application types while ignoring primitives and standard typing helpers. This
is intentionally conservative and does not attempt full Python name or import
resolution.

## Workflow Routing

- `python` and `py` resolve to the Python parser.
- code sync and query probing recognize `.py`.
- code sync skips `test_*.py` and `*_test.py` files.
- implementation verification recognizes class and function declarations.
- extract/codegen emit Python-shaped interface or class stubs.
