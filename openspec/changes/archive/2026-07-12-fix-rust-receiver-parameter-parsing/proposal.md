## Why

Rust parser parameter extraction treated any parameter text containing `self` as a receiver. That dropped valid parameters such as `self_user: User` and caused dependency extraction to miss those types.

## What Changes

- Add a Rust parser regression test for parameters whose names contain `self`.
- Narrow Rust receiver detection to actual receiver forms such as `self`, `&self`, `&mut self`, lifetime-qualified receivers, and typed receivers such as `self: Box<Self>`.
- Preserve real parameters and their dependency extraction when their names only contain `self` as a substring.

## Impact

- Affected code: `internal/parser/rust_parser.go`
- Affected tests: `internal/parser/rust_parser_test.go`
- User-facing impact: Rust code sync and parser-driven workflows keep valid parameters instead of silently losing them.
