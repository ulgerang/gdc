// Package search provides search and query functionality for GDC codebase analysis.
//
// This package defines the interfaces for searching code patterns and querying
// node information from the code index. It supports graceful degradation when
// the index is unavailable, falling back to basic file-based operations.
//
// The search functionality follows the Repository pattern, allowing different
// implementations (indexed, filesystem-based, external service) while maintaining
// a consistent interface.
package search

import (
	"errors"
	"time"
)

// Sentinel errors for graceful degradation and error handling.
// These errors allow callers to distinguish between different failure modes
// and provide appropriate fallback behavior or user guidance.
//
// Note: ErrIndexNotBuilt, ErrProjectNotInit, and ErrNoNodesFound are defined
// in index_check.go to avoid circular dependencies.
var (
	// ErrNodeNotFound indicates that the requested node does not exist
	// in the codebase or index.
	ErrNodeNotFound = errors.New("node not found")

	// ErrSearchTimeout indicates that the search operation exceeded
	// the allotted time limit.
	ErrSearchTimeout = errors.New("search operation timed out")

	// ErrInvalidPattern indicates that the provided search pattern
	// is malformed or unsupported.
	ErrInvalidPattern = errors.New("invalid search pattern")

	// ErrIndexCorrupted indicates that the index exists but is corrupted
	// and needs to be rebuilt.
	ErrIndexCorrupted = errors.New("code index is corrupted")
)

// SearchResult represents a single search match found in the codebase.
// Each result captures the location and context of the match for display
// and navigation purposes.
type SearchResult struct {
	// FilePath is the absolute or relative path to the file containing the match.
	FilePath string `json:"file_path"`

	// LineNumber is the 1-based line number where the match occurs.
	LineNumber int `json:"line_number"`

	// Column is the 1-based column position where the match starts.
	Column int `json:"column"`

	// Content is the actual matched text.
	Content string `json:"content"`

	// Context provides surrounding lines for display purposes.
	// The number of lines is determined by SearchOptions.ContextLines.
	// Lines are separated by newlines.
	Context string `json:"context,omitempty"`

	// MatchStart is the byte offset where the match begins in the line.
	MatchStart int `json:"match_start,omitempty"`

	// MatchEnd is the byte offset where the match ends in the line.
	MatchEnd int `json:"match_end,omitempty"`
}

// SearchOptions configures the behavior of search operations.
// Zero values use sensible defaults: case-insensitive, no regex,
// no file filtering, unlimited results, no context.
type SearchOptions struct {
	// CaseSensitive determines whether the search respects case.
	// Default is false (case-insensitive).
	CaseSensitive bool `json:"case_sensitive"`

	// WholeWord requires matches to be bounded by word boundaries.
	// Useful for finding exact symbol names.
	WholeWord bool `json:"whole_word"`

	// Regex treats the pattern as a regular expression.
	// When false, the pattern is treated as a literal string.
	Regex bool `json:"regex"`

	// FilePattern filters results to files matching this glob pattern.
	// Examples: "*.go", "**/*.ts", "src/**/*.java"
	// Empty string matches all files.
	FilePattern string `json:"file_pattern,omitempty"`

	// MaxResults limits the number of results returned.
	// A value of 0 or negative means unlimited.
	MaxResults int `json:"max_results,omitempty"`

	// ContextLines specifies the number of lines before and after
	// the match to include in SearchResult.Context.
	// A value of 0 omits context.
	ContextLines int `json:"context_lines,omitempty"`

	// ExcludePatterns specifies glob patterns for files/directories to exclude.
	// Common patterns: "vendor/*", "node_modules/*", "*.generated.*"
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`

	// IncludeHidden includes hidden files (starting with .) in search.
	// Default is false.
	IncludeHidden bool `json:"include_hidden"`
}

// NodeReference represents a reference to a node in the codebase.
// References capture how one node depends on or uses another.
type NodeReference struct {
	// NodeID is the unique identifier of the referenced node.
	NodeID string `json:"node_id"`

	// NodeType is the type of the referenced node (class, interface, etc.).
	NodeType string `json:"node_type"`

	// FilePath is the file where the reference occurs.
	FilePath string `json:"file_path"`

	// LineNumber is the line where the reference occurs.
	LineNumber int `json:"line_number"`

	// Reference describes how the node is referenced.
	// Examples: "import", "implements", "calls", "extends", "uses"
	Reference string `json:"reference"`

	// CodeSnippet shows the actual code making the reference.
	CodeSnippet string `json:"code_snippet,omitempty"`
}

// SymbolInfo contains detailed information about a symbol found via query.
type SymbolInfo struct {
	// Name is the symbol's identifier.
	Name string `json:"name"`

	// Kind is the symbol's kind (function, method, class, interface, etc.).
	Kind string `json:"kind"`

	// FilePath is the file containing the symbol definition.
	FilePath string `json:"file_path"`

	// LineNumber is the line where the symbol is defined.
	LineNumber int `json:"line_number"`

	// Signature is the full signature for functions/methods.
	Signature string `json:"signature,omitempty"`

	// Documentation contains any doc comments for the symbol.
	Documentation string `json:"documentation,omitempty"`

	// Parent is the containing scope (class for methods, package for functions).
	Parent string `json:"parent,omitempty"`

	// Exported indicates whether the symbol is publicly accessible.
	Exported bool `json:"exported"`
}

// SearchService provides pattern-based search functionality.
// Implementations may use different backends (filesystem, index, external service)
// but all provide the same search semantics.
//
// Graceful Degradation:
// When the index is unavailable, implementations should fall back to
// file-based search using standard tools (grep-like functionality).
// Use IsIndexAvailable() to check index status before operations.
type SearchService interface {
	// Search searches for a pattern in the codebase.
	// The pattern interpretation depends on SearchOptions.Regex:
	// - When Regex is false: literal string match
	// - When Regex is true: regular expression match
	//
	// Returns ErrInvalidPattern if the pattern is malformed.
	// Returns ErrSearchTimeout if the operation exceeds internal limits.
	// Returns ErrIndexNotBuilt if index is required but not available.
	Search(pattern string, opts SearchOptions) ([]SearchResult, error)

	// SearchInFile searches for a pattern within a specific file.
	// This is more efficient than filtering Search results by file path.
	SearchInFile(filePath, pattern string, opts SearchOptions) ([]SearchResult, error)

	// IsIndexAvailable returns true if the code index is built and ready.
	// When false, search operations may still work using filesystem fallback.
	IsIndexAvailable() bool

	// IndexStatus returns a human-readable status message about the index.
	// Useful for CLI output and debugging.
	IndexStatus() string
}

// QueryService provides symbol-based query functionality.
// Unlike SearchService which finds text patterns, QueryService understands
// code structure and can find symbols by name and trace relationships.
type QueryService interface {
	// QuerySymbol looks up a symbol by name.
	// Name can be:
	// - Simple name: "UserService"
	// - Qualified name: "auth.UserService"
	// - Fully qualified: "github.com/myorg/myapp/auth.UserService"
	//
	// Returns ErrNodeNotFound if no symbol matches.
	// Returns ErrIndexNotBuilt if index is required but not available.
	QuerySymbol(name string) (*SymbolInfo, error)

	// QuerySymbolByPrefix finds all symbols starting with the given prefix.
	// Useful for autocomplete and symbol discovery.
	QuerySymbolByPrefix(prefix string, limit int) ([]SymbolInfo, error)

	// FindReferences finds all references to a node in the codebase.
	// This includes imports, usages, implementations, and extensions.
	//
	// Returns ErrNodeNotFound if the node doesn't exist.
	FindReferences(nodeID string) ([]NodeReference, error)

	// FindImplementations finds all types that implement the given interface.
	// For non-interface nodes, this returns types that extend or inherit.
	FindImplementations(interfaceID string) ([]SymbolInfo, error)

	// FindCallers finds all locations where a function/method is called.
	FindCallers(functionName string) ([]NodeReference, error)

	// FindCallees finds all functions/methods called by the given function.
	FindCallees(functionName string) ([]SymbolInfo, error)

	// IsIndexAvailable returns true if the code index is built and ready.
	IsIndexAvailable() bool

	// IndexStatus returns a human-readable status message about the index.
	IndexStatus() string
}

// IndexCheck provides utilities for graceful degradation.
// It allows callers to verify index availability and provide
// helpful guidance to users when the index is missing or outdated.
//
// Note: IndexChecker in index_check.go provides a concrete implementation
// of most of this functionality. This interface exists for abstraction
// and testing purposes.
type IndexCheck interface {
	// Check verifies the index status and returns an error if unavailable.
	// The error message should be user-friendly and actionable.
	Check() error

	// SuggestCommand returns a command the user can run to fix index issues.
	// Example: "gdc sync --rebuild"
	SuggestCommand() string

	// IsStale returns true if the index exists but may be outdated
	// based on file modification times.
	IsStale() bool

	// LastSyncTime returns when the index was last synchronized.
	// Returns zero time if never synced.
	LastSyncTime() time.Time
}

// SearchResultFormatter formats search results for output.
// Implementations can customize output for different contexts
// (terminal, JSON, IDE integration).
type SearchResultFormatter interface {
	// Format formats a single search result.
	Format(result SearchResult) string

	// FormatAll formats multiple results, potentially with grouping.
	FormatAll(results []SearchResult) string

	// SetColorEnabled enables or disables colored output.
	SetColorEnabled(enabled bool)
}

// DefaultSearchOptions returns SearchOptions with sensible defaults.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		CaseSensitive: false,
		WholeWord:     false,
		Regex:         false,
		MaxResults:    100,
		ContextLines:  2,
		ExcludePatterns: []string{
			"vendor/*",
			"node_modules/*",
			"*.min.js",
			"*.generated.*",
		},
		IncludeHidden: false,
	}
}

// NewSearchResult creates a SearchResult with computed match positions.
func NewSearchResult(filePath string, lineNum, column int, content, context string) SearchResult {
	return SearchResult{
		FilePath:   filePath,
		LineNumber: lineNum,
		Column:     column,
		Content:    content,
		Context:    context,
		MatchStart: column - 1,
		MatchEnd:   column - 1 + len(content),
	}
}
