## Approach

Add a deterministic lexical helper that removes only leading annotations. It
tracks balanced annotation parentheses and quoted strings, then returns the
remaining declaration text. Declaration collection applies the helper to the
first line so multiline annotated functions still use the existing signature
parser.

Annotation-only lines return an empty declaration and continue to preserve any
pending `##` documentation for the next declaration.

## Non-goals

- Interpreting annotation semantics or arguments
- Evaluating annotation expressions
- Treating annotations as graph nodes
