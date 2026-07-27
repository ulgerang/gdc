# GDC - Graph-Driven Codebase

**GDC** is a specification-driven development tool for AI-assisted software development.  
It models software systems as graphs (nodes & edges) to maximize the accuracy of AI code generation.

## ✨ Core Principles

- **Single Source of Truth**: YAML specs are the single source of truth for all design
- **Context Isolation**: Provide only minimal, accurate context to AI
- **Graph-First Design**: Express systems with classes (nodes) and dependencies (edges)
- **Opt-in Evidence**: Code evidence (implementation/tests/callers) is introduced gradually as opt-in

## Design-First Operating Model

GDC is intended to be used before code is written, not only after code exists.
Write `.gdc/nodes/*.yaml` first to describe the structure a coder or AI agent
should implement:

- `node`: the component identity, type, layer, namespace, and optional future
  implementation path.
- `responsibility`: what the component owns, its boundaries, and invariants.
- `interface`: the public contract to implement exactly.
- `dependencies`: the other nodes this component may use, including injection
  style and usage notes.
- `logic`: optional state machines, algorithms, data flow, or pseudocode.
- `metadata.status`: the lifecycle state, such as `draft`, `specified`,
  `implemented`, `tested`, or `deprecated`.

The normal loop is:

```text
OpenSpec or BDD acceptance
  -> GDC YAML node specs
  -> gdc sync
  -> gdc query/show/trace to collect dependency context
  -> gdc extract <node> to create a focused coder/agent packet
  -> implementation and tests
  -> gdc sync --direction code or gdc sync --direction both
  -> gdc diff/check to detect structural drift
```

In this model OpenSpec or BDD decides what should be built and why. GDC decides
where it belongs, which contracts it may depend on, and what minimal context the
coder receives. Code evidence is useful, but it is opt-in: start from the YAML
contract, then add `--with-impl`, `--with-tests`, or `--with-callers` only when
the implementation task needs that extra context.

## 🚀 Quick Start

### Build

```bash
# Requires Go 1.23+
go build -o gdc .

# Windows
go build -o gdc.exe .

# Explicit entrypoint
go build -o gdc ./cmd/gdc

# Using Makefile
make build
```

### Usage

```bash
# 1. Initialize project
gdc init
gdc init --language typescript
gdc init --language rust
gdc init --language python
gdc init --language go --storage distributed

# 2. Create nodes
gdc node create PlayerController
gdc node create IInputManager --type interface
gdc node create GameService --type service --layer application

# 3. Manage nodes
gdc node delete OldController                 # Refuses referenced nodes
gdc node delete OldController --force         # Also removes dependency references
gdc node rename PlayerController CharacterController  # Updates references and DB

# 4. Write YAML specs before implementation (edit .gdc/nodes/*.yaml)

# 5. Sync the design graph and verify it
gdc sync                              # Sync YAML to DB
gdc sync --dry-run                    # Preview changes
gdc sync --force                      # Force full resync
gdc sync --direction code             # Extract from code → YAML
gdc sync --direction code --source src/
gdc sync --direction code --files src/services/user_service.go
gdc sync --direction code --dirs src/services --symbols UserService
gdc sync --direction both --strategy merge
gdc sync --timing --profile --profile-output .gdc/sync-profile.json

gdc check                             # Consistency check
gdc check --category hash_mismatch    # Filter by category
gdc check --severity error            # Filter by severity
gdc check --verify-impl --fail-on-missing
gdc check --format json               # JSON output

# 6. List and show nodes
gdc list
gdc list --filter "layer=domain"
gdc list --filter "type=interface"
gdc list --format json
gdc show PlayerController
gdc show PlayerController --deps --refs
gdc show IInputManager --full
gdc show IInputManager --interface-only
gdc show PlayerController --format json

# 7. Generate a focused implementation packet for a coder or AI agent
gdc extract PlayerController --clipboard
gdc extract PlayerController --output prompt.md
gdc extract PlayerController --template implement
gdc extract PlayerController --format json

# 8. Include code evidence only when needed (opt-in)
gdc extract PlayerController --with-impl
gdc extract PlayerController --with-impl --with-tests
```

## 📋 Key Commands

| Command | Description |
|---------|-------------|
| `gdc init` | Initialize project |
| `gdc version` | Show version information |
| `gdc node create <name>` | Create a node |
| `gdc node delete <name>` | Delete a node |
| `gdc node rename <old> <new>` | Rename a node |
| `gdc list` | List nodes |
| `gdc show <node>` | Show node details |
| `gdc trace <node>` | Trace dependencies |
| `gdc trace <node> --reverse` | Trace reverse dependencies (nodes referencing this node) |
| `gdc graph` | Export graph (DOT/Mermaid/JSON) |
| `gdc sync` | Sync YAML ↔ DB |
| `gdc check` | Consistency check |
| `gdc extract <node>` | Generate AI implementation prompt |
| `gdc diff <node>` | Compare YAML spec against current code |
| `gdc stats` | Project statistics |
| `gdc search <pattern>` | Search patterns in codebase |
| `gdc query <symbol>` | Query node info by symbol name |
| `gdc deps <node>` | List dependencies (JSON) |
| `gdc refs <node>` | List referencing nodes (JSON) |
| `gdc context <node>` | Full extraction context (JSON) |

Node lifecycle commands preserve graph integrity. `node delete` reports reverse
references and refuses the operation unless `--force` is supplied; forced deletion
removes those dependency edges. `node rename` updates dependency targets across YAML
specifications. Both commands refresh `.gdc/graph.db` after the YAML mutation.

## 🔧 Global Flags

| Flag | Description |
|------|-------------|
| `-c, --config` | Config file path (default: .gdc/config.yaml) |
| `-v, --verbose` | Verbose output |
| `-q, --quiet` | Minimal output |
| `--json` | Output in JSON format |
| `--no-color` | Disable colored output |

## 🔍 Search and Query Commands

### gdc search
Search for patterns in the codebase.

```bash
# Basic search
gdc search "PlayerController"

# Specify file pattern
gdc search "TODO" --file-pattern "*.go"

# Regex search
gdc search "func.*Handler" --regex

# Case-sensitive
gdc search "UserService" --case-sensitive

# Limit results
gdc search "import" --max-results 20

# Include context lines (grep-like)
gdc search "error" --context 2

# JSON output
gdc search "PlayerController" --format json
```

### gdc trace --reverse
Trace all nodes referencing a specific node (reverse dependencies).

```bash
# Show all nodes depending on PlayerController
gdc trace PlayerController --reverse

# Limit depth
gdc trace PlayerController --reverse --depth 2

# Bidirectional (dependencies + reverse dependencies)
gdc trace PlayerController --direction both

# Find path to specific node
gdc trace PlayerController --to DatabaseService

# JSON output
gdc trace PlayerController --format json
```

### gdc query
Query detailed information by node ID, qualified name, file path, or partial symbol.
Results now include match provenance such as canonical ID, spec path, implementation path,
and whether the match came from an exact ID, qualified name, or file lookup.

```bash
# Basic query
gdc query PlayerController

# Qualified name lookup
gdc query Game.Controllers.PlayerController

# File path lookup
gdc query src/Controllers/PlayerController.cs

# Partial discovery
gdc query Player

# JSON output
gdc query PlayerController --format json

# Machine-safe file lookup returning every mapped node
gdc query src/Controllers/PlayerController.cs --all --format json

# YAML output
gdc query PlayerController --format yaml

# Verbose (includes metadata, implementation list)
gdc query PlayerController --verbose
```

When a symbol is found in source files but is not yet in the graph, `gdc query` now points you
to the matching files and suggests a scoped `gdc sync --direction code --symbols <name>` follow-up.
Structured query formats write only the JSON or YAML document to stdout. In `--all` mode, no
matches produce an empty array, making the command suitable for file-oriented companion tools.

### gdc sync
Sync specs with the graph database or extract graph nodes from code. Scope-limited sync is supported
for local implementation loops.

```bash
# Full sync
gdc sync

# Preview only
gdc sync --dry-run

# Limit sync to specific files
gdc sync --direction code --files src/services/user_service.go

# Limit sync to a directory
gdc sync --direction code --dirs src/services

# Limit sync to specific symbols
gdc sync --direction code --symbols UserService,AuthService
```

### gdc extract (Extended Options)
Include code evidence as opt-in when generating AI prompts.

```bash
# Basic prompt (specs + dependency interfaces only)
gdc extract PlayerController

# Source-free implementation packet. Requires a complete schema 1.1
# implementation contract and validates the full dependency closure.
gdc extract PlayerController --for-implementation

# Include implementation code
gdc extract PlayerController --with-impl

# Include related tests
gdc extract PlayerController --with-tests

# Include caller/reference info
gdc extract PlayerController --with-callers

# Include all code evidence
gdc extract PlayerController --with-impl --with-tests --with-callers

# Copy to clipboard
gdc extract PlayerController --with-impl --clipboard

# Custom output file
gdc extract PlayerController --output prompt.md

# Use different template
gdc extract PlayerController --template review
```

`--for-implementation` is the only extract mode that claims the authored `.gdc`
contracts are sufficient to implement the target without repository source context.
It fails on unknown schema fields, unresolved placeholders, missing behavioral or
acceptance contracts, missing dependency members, and stale `contract_hash` values.
`contract_hash` is not a Git commit or source hash; it is GDC's fingerprint of the
dependency's authored contract, and readiness errors report the current value to use.
The mode cannot be combined with `--with-impl`, `--with-tests`, or `--with-callers`.
Ordinary extract remains the exploratory, backward-compatible mode.

### gdc graph
Export the dependency graph in various formats.

```bash
# Mermaid format (default)
gdc graph

# Graphviz DOT format
gdc graph --format dot --output graph.dot

# JSON format
gdc graph --format json > graph.json
```

### gdc stats
Display project statistics.

```bash
# Show statistics
gdc stats

# JSON output
gdc stats --format json
```

### gdc deps
List direct and transitive dependencies of a node (always JSON output).

```bash
gdc deps PlayerController
gdc deps PlayerController --depth 2
gdc deps PlayerController --transitive
```

### gdc refs
List all nodes that reference (depend on) a given node (always JSON output).

```bash
gdc refs IInputManager
gdc refs IInputManager --depth 2
```

### gdc context
Return full extraction context for a node — spec, dependencies, and optional evidence (always JSON output).

```bash
gdc context PlayerController
gdc context PlayerController --depth 2
gdc context PlayerController --with-impl --with-tests --with-callers
```

### gdc check
Validate graph integrity and check for issues.

Validation categories:
- `missing_ref` - References to non-existent nodes
- `hash_mismatch` - Contract hash mismatches
- `cycle` - Circular dependencies
- `orphan` - Nodes not referenced anywhere
- `impl_missing` - file_path missing or symbol not found in code
- `impl_mismatch` - Spec members do not match implementation
- `layer_violation` - Architecture layer violations
- `srp_violation` - Too many dependencies (SRP)

```bash
# Run all checks
gdc check

# Filter by category
gdc check --category hash_mismatch

# Filter by severity
gdc check --severity error

# Auto-fix issues
gdc check --fix
```

## 🔧 Parsers

GDC includes multi-language parsers to extract node information from source code.

### Supported Languages

| Language | Regex Parser | Tree-sitter Parser |
|----------|:------------:|:------------------:|
| Go | ✅ Default | - |
| C# | ✅ Default | ✅ (build tag) |
| TypeScript | ✅ Default | ✅ (build tag) |
| Rust | ✅ Default | - |
| Python | ✅ Default | - |

### Parser Features

- **Class/Interface Detection**: Extract type declarations, inheritance, implementation relationships
- **Method/Property Extraction**: Signatures, access modifiers, async/static modifiers
- **Automatic Dependency Detection**: Constructor injection, field injection patterns
- **Attributes/Decorators**: Extract C# attributes, TypeScript decorators
- **JSDoc/XML Documentation**: Extract descriptions from documentation comments

### Using Tree-sitter Parser

```bash
# Build with Tree-sitter based parser (more accurate parsing)
go build -tags treesitter -o gdc .
```

## 📁 Project Structure

```
.gdc/                            # GDC project configuration
├── config.yaml                  # Project settings
├── graph.db                     # SQLite index (gitignore)
├── nodes/                       # Node specification YAML
│   ├── IInputManager.yaml
│   └── PlayerController.yaml
└── templates/                   # Prompt templates
    └── implement.md.j2

cmd/gdc/                         # Alternate CLI entrypoint
internal/
├── cli/                         # CLI command definitions
│   ├── root.go                  # Root command and global flags
│   ├── output.go                # Shared JSON output helpers
│   ├── deps.go                  # deps command (dependency listing)
│   ├── refs.go                  # refs command (reverse dependency listing)
│   ├── context.go               # context command (full extraction context)
│   ├── extract.go               # extract command (AI prompt generation)
│   ├── search.go                # search command (pattern search)
│   ├── query.go                 # query command (symbol query)
│   └── trace.go                 # trace command (dependency/reverse dependency tracing)
├── extract/                     # Context assembly engine
│   ├── context_assembler.go     # Orchestrator (Hexagonal Architecture)
│   ├── impl_loader.go           # Implementation code loader
│   ├── test_matcher.go          # Test file matcher
│   ├── caller_resolver.go       # Caller resolver
│   └── output_formatter.go      # Output formatter
├── parser/                      # Source code parsers
│   ├── csharp_parser.go         # C# Regex parser
│   ├── csharp_parser_treesitter.go  # C# Tree-sitter parser
│   ├── python_parser.go         # Python heuristic parser
│   ├── rust_parser.go           # Rust Regex parser
│   ├── typescript_parser.go     # TypeScript Regex parser
│   └── typescript_parser_treesitter.go  # TypeScript Tree-sitter parser
├── search/                      # Search infrastructure
│   ├── interface.go             # Search interface definition
│   └── index_check.go           # Index status check (graceful degradation)
├── config/                      # Configuration management
└── node/                        # Node spec model

fixtures/                        # Parser test fixtures
scripts/                         # Utility scripts
└── benchmark_baseline.sh        # Performance benchmark baseline
tests/
└── integration/                 # Integration tests
```

## 📖 Documentation

- [📘 SPEC.md](docs/SPEC.md) - Full specification
- [🚀 QUICKSTART.md](docs/QUICKSTART.md) - Quick start guide
- [🧭 CODERLM_INTEGRATION.md](docs/CODERLM_INTEGRATION.md) - GDC extension strategy based on CodeRLM approach
- [📄 Node Schema](docs/schemas/node-schema.yaml) - Node schema
- [🗄️ DB Schema](docs/schemas/database-schema.sql) - Database schema

## 🛠 Development

```bash
# Install dependencies
go mod tidy

# Build
make build

# Run all tests
make test

# Phase-by-phase verification tests
make test-p1    # P1: Basic functionality verification
make test-p3    # P3: Parser enhancement (C#/TypeScript/Rust/Python)
make test-p4    # P4: Search/Query/Trace commands

# Clean
make clean
```

## 📄 License

MIT License
## Recent CLI Updates

The commands below reflect the current CLI behavior.

### sync

```bash
# Extract signatures from code into YAML
gdc sync --direction code --auto-status

# Run code sync and refresh the DB index in one step
gdc sync --direction both --strategy merge

# Review drift/conflicts discovered during code sync
gdc sync --direction both --conflict-log .gdc/conflicts.log

# Show timings and write a JSON profile report
gdc sync --timing --profile --profile-output .gdc/sync-profile.json
```

### check

```bash
# Verify file_path, symbol existence, and member drift
gdc check --verify-impl
gdc check --verify-impl --fail-on-missing

# CI-friendly output and thresholds
gdc check --ci-mode --max-warnings 5
gdc check --exit-on-warning

# Reduce orphan noise or enforce layer failures
gdc check --no-orphan-info
gdc check --orphan-filter "entry-point"
gdc check --layer-strict
```

### graph

```bash
# Highlight or isolate architecture layer violations
gdc graph --layer-violations
gdc graph --violations-only

# Export an interactive HTML viewer
gdc graph --interactive --output graph.html
```

### diff

```bash
# Compare the stored YAML spec for a node with the current implementation
gdc diff Agent
gdc diff Agent --format json
```

### extract

```bash
# Include implementation, tests, and caller/reference evidence
gdc extract PlayerController --with-impl --with-tests --with-callers
```

### Shell wrappers

For local development, `gdc.bat` and `gdc.sh` run the checked-out `cmd/gdc`
source by default, even when a local executable exists. Set `GDC_USE_PREBUILT=1`
only when you intentionally want the wrapper to use `gdc.exe`, `gdc`, or
`gdc-linux-amd64`.
