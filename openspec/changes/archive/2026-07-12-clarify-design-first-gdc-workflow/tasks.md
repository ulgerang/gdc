## 1. Design-First Workflow Documentation

- [x] 1.1 Add a README section that positions GDC as a design-first structural authority before coding
- [x] 1.2 Document the OpenSpec/BDD -> GDC YAML -> extract/query/trace -> implementation -> sync/check loop

## 2. Dependency Alias Resolution

- [x] 2.1 Add shared alias resolution for unique short dependency targets
- [x] 2.2 Update trace traversal/path finding to use canonical dependency targets
- [x] 2.3 Update graph exports and layer-violation edge detection to use canonical dependency targets
- [x] 2.4 Add regression tests for namespaced dependencies referenced by short names

## 3. Validation

- [x] 3.1 Run focused CLI tests for trace/graph behavior
- [x] 3.2 Run full Go tests and vet
- [x] 3.3 Validate OpenSpec and GDC consistency
