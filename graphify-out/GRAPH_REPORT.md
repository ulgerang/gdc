# Graph Report - D:\git\26\gdc  (2026-04-25)

## Corpus Check
- 2 files · ~2,354 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 30 nodes · 63 edges · 6 communities detected
- Extraction: 44% EXTRACTED · 56% INFERRED · 0% AMBIGUOUS · INFERRED: 35 edges (avg confidence: 0.5)
- Token cost: 0 input · 0 output

## God Nodes (most connected - your core abstractions)

## Surprising Connections (you probably didn't know these)
- None detected - all connections are within the same source files.

## Communities

### Community 0 - "Rust parser receiver parameter parsing"
Cohesion: 0.47
Nodes (4): collectRustSignature(), parseRustDocLine(), RustParser, stripRustLineComment()

### Community 1 - "Rust parser receiver parameter parsing"
Cohesion: 0.4
Nodes (3): captureRustBlock(), findMatchingParen(), isRustConstructor()

### Community 2 - "Rust parser receiver parameter parsing"
Cohesion: 0.4
Nodes (0): 

### Community 3 - "Rust parser receiver parameter parsing"
Cohesion: 0.5
Nodes (4): extractRustDependencyTypes(), rustDependenciesFromParameters(), shouldSkipRustDependency(), splitRustTypeIdentifiers()

### Community 4 - "Rust parser receiver parameter parsing"
Cohesion: 0.67
Nodes (3): isRustReceiverParameter(), parseRustParameters(), splitRustCommaList()

### Community 5 - "Rust parser receiver parameter parsing"
Cohesion: 1.0
Nodes (3): buildRustSignature(), collapseWhitespace(), parseRustMethodSignature()