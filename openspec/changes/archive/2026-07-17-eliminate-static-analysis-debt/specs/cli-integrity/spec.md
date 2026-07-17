## ADDED Requirements

### Requirement: Database-backed results fail closed
Database-backed operations SHALL report query, scan, iteration, and finalization failures instead of returning partial results that appear complete.

#### Scenario: Statistics storage is unavailable
- **WHEN** statistics are requested after the database connection becomes unavailable
- **THEN** the operation returns a contextual error
- **AND** it does not return partial statistics as a successful result
