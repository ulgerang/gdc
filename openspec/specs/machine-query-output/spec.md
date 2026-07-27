# machine-query-output Specification

## Purpose
TBD - created by archiving change stabilize-machine-integration-contract. Update Purpose after archive.
## Requirements
### Requirement: Structured query output SHALL be machine-clean
When query uses JSON or YAML output, GDC SHALL write only the structured document to stdout and SHALL NOT prefix it with human-oriented ambiguity or suggestion text.

#### Scenario: Ambiguous file query in JSON mode
- **WHEN** multiple nodes map to the queried source file and the user selects JSON output
- **THEN** stdout contains one valid JSON document
- **AND** a machine consumer can decode it without removing text prefixes

### Requirement: Query SHALL expose all ranked matches
GDC SHALL provide an opt-in query mode that returns every matching node in deterministic rank order.

#### Scenario: Source file contains multiple nodes
- **WHEN** a machine consumer queries the source file with all-match JSON mode
- **THEN** GDC returns a JSON array containing every mapped node
- **AND** each entry includes its canonical ID and implementation path

#### Scenario: Source file is not represented in the graph
- **WHEN** all-match JSON mode finds no matching node
- **THEN** GDC returns an empty JSON array successfully
