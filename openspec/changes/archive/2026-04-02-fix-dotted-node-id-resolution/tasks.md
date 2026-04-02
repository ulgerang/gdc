## 1. Resolution Semantics

- [x] 1.1 Update canonical ID generation to avoid double-prefixing when `node.id` already starts with `namespace.`
- [x] 1.2 Update extracted-node lookup to resolve dotted synced IDs against parsed source symbols

## 2. Verification

- [x] 2.1 Add regression tests for canonical dotted IDs and dotted source lookup
- [x] 2.2 Run focused Go tests for the touched CLI and node packages
