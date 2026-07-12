## ADDED Requirements

### Requirement: Rust parser SHALL only skip actual receiver parameters
The Rust parser SHALL skip Rust receiver parameters without dropping ordinary parameters whose names or types contain the string `self`.

#### Scenario: Parameter name contains self
- **WHEN** a Rust method has a receiver and a parameter named `self_user`
- **THEN** parser extraction keeps `self_user`
- **AND** dependency extraction includes the parameter type

#### Scenario: Receiver uses lifetime or typed receiver syntax
- **WHEN** a Rust method receiver is written as `&'a mut self` or `self: Box<Self>`
- **THEN** parser extraction skips the receiver itself
- **AND** keeps the remaining ordinary parameters
