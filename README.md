# GDC - Graph-Driven Codebase

**GDC** is a specification-driven development tool for AI-assisted software development.  
It models software systems as graphs (nodes & edges) to maximize the accuracy of AI code generation.

## ✨ Core Principles

- **Single Source of Truth**: YAML specs are the single source of truth for all design
- **Context Isolation**: Provide only minimal, accurate context to AI
- **Graph-First Design**: Express systems with classes (nodes) and dependencies (edges)
- **Opt-in Evidence**: Code evidence (implementation/tests/callers) is introduced gradually as opt-in

## 🚀 Quick Start

### Build

```bash
# Requires Go 1.22+
go build -o gdc ./cmd/gdc

# Windows
go build -o gdc.exe ./cmd/gdc

# Using Makefile
make build
```

### Usage

```bash
# 1. Initialize project
gdc init
gdc init --language typescript
gdc init --language go --storage distributed

# 2. Create nodes
gdc node create PlayerController
gdc node create IInputManager --type interface
gdc node create GameService --type service --layer application

# 3. Manage nodes
gdc node delete OldController
gdc node rename PlayerController CharacterController

# 4. Write YAML specs (edit .gdc/nodes/*.yaml)

# 5. Sync and verify
gdc sync                              # Sync YAML to DB
gdc sync --dry-run                    # Preview changes
gdc sync --force                      # Force full resync
gdc sync --direction code             # Extract from code → YAML
gdc sync --direction code --source src/
gdc sync --direction code --files src/services/user_service.go
gdc sync --direction code --dirs src/services --symbols UserService

gdc check                             # Consistency check
gdc check --category hash_mismatch    # Filter by category
gdc check --severity error            # Filter by severity

# 6. List and show nodes
gdc list
gdc list --filter "layer=domain"
gdc list --filter "type=interface"
gdc list --format json
gdc show PlayerController
gdc show PlayerController --deps --refs
gdc show IInputManager --full
gdc show IInputManager --interface-only

# 7. Generate AI prompt
gdc extract PlayerController --clipboard
gdc extract PlayerController --output prompt.md
gdc extract PlayerController --template implement

# 8. Include code evidence in prompt (opt-in)
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
| `gdc stats` | Project statistics |
| `gdc search <pattern>` | Search patterns in codebase |
| `gdc query <symbol>` | Query node info by symbol name |

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

# YAML output
gdc query PlayerController --format yaml

# Verbose (includes metadata, implementation list)
gdc query PlayerController --verbose
```

When a symbol is found in source files but is not yet in the graph, `gdc query` now points you
to the matching files and suggests a scoped `gdc sync --direction code --symbols <name>` follow-up.

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
```

### gdc check
Validate graph integrity and check for issues.

Validation categories:
- `missing_ref` - References to non-existent nodes
- `hash_mismatch` - Contract hash mismatches
- `cycle` - Circular dependencies
- `orphan` - Nodes not referenced anywhere
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

### Parser Features

- **Class/Interface Detection**: Extract type declarations, inheritance, implementation relationships
- **Method/Property Extraction**: Signatures, access modifiers, async/static modifiers
- **Automatic Dependency Detection**: Constructor injection, field injection patterns
- **Attributes/Decorators**: Extract C# attributes, TypeScript decorators
- **JSDoc/XML Documentation**: Extract descriptions from documentation comments

### Using Tree-sitter Parser

```bash
# Build with Tree-sitter based parser (more accurate parsing)
go build -tags treesitter -o gdc ./cmd/gdc
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

cmd/gdc/                         # CLI entrypoint
internal/
├── cli/                         # CLI command definitions
│   ├── root.go                  # Root command and global flags
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
make test-p3    # P3: Parser enhancement (C#/TypeScript)
make test-p4    # P4: Search/Query/Trace commands

# Clean
make clean
```

## 📄 License

MIT License
