// Package extract provides context assembly capabilities for the GDC extract command.
//
// Architecture Overview:
//
// The extract package follows the Ports and Adapters pattern (Hexagonal Architecture),
// separating the core domain logic from external concerns like file system access,
// database queries, and output formatting.
//
// Core Components:
//
// 1. ContextAssembler (Orchestrator)
//   - Coordinates the assembly of extraction context
//   - Delegates to specialized components
//   - Maintains separation between spec and code evidence
//
// 2. SpecLoader (Port)
//   - Loads node specifications from YAML files
//   - Independent of code analysis
//
// 3. CodeLoader (Port)
//   - Loads implementation code from source files
//   - Gracefully degrades when code index unavailable
//
// 4. TestMatcher (Port)
//   - Finds test files related to a node
//   - Language-aware test discovery
//
// 5. CallerResolver (Port)
//   - Resolves callers and references to functions
//   - Works with code index or direct analysis
//
// 6. OutputFormatter (Port)
//   - Formats the final assembled context
//   - Multiple output formats supported
//
// Error Handling Strategy:
//
// All operations return errors that can be classified as:
//   - Critical: Operation cannot continue (file not found, parse error)
//   - Recoverable: Partial success possible (missing code index, test not found)
//   - Informational: Warnings that don't affect output quality
//
// The ContextAssembler aggregates all errors but continues when possible,
// providing the best available context even with partial failures.
package extract

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// CONFIGURATION
// ============================================================================

// ExtractOptions configures the behavior of context extraction.
// This struct uses the Options pattern for extensibility - new options
// can be added without changing method signatures.
type ExtractOptions struct {
	// Node identification
	NodeName string
	NodePath string // Optional: explicit path to node spec

	// Context inclusion flags
	IncludeImpl    bool // Load implementation code
	IncludeTests   bool // Load related test files
	IncludeCallers bool // Load callers/references

	// Scope control
	DependencyDepth int  // How many levels of dependencies to include
	IncludeLogic    bool // Include internal logic specification

	// Output configuration
	OutputFormat OutputFormat
	TemplateName string // For custom prompt templates

	// Language and project context
	Language    string
	ProjectRoot string

	// Advanced options
	MaxCodeLines  int  // Limit code output length
	MaxCallers    int  // Limit number of callers shown
	StrictMode    bool // Fail on missing dependencies
	IndexRequired bool // Require code index to be available
}

// OutputFormat defines the format of the assembled output.
type OutputFormat string

const (
	FormatPrompt   OutputFormat = "prompt"   // AI-optimized prompt (default)
	FormatJSON     OutputFormat = "json"     // Structured JSON output
	FormatMarkdown OutputFormat = "markdown" // Human-readable markdown
	FormatPlain    OutputFormat = "plain"    // Plain text
)

// DefaultExtractOptions returns sensible defaults for extraction.
func DefaultExtractOptions(nodeName string) ExtractOptions {
	return ExtractOptions{
		NodeName:        nodeName,
		DependencyDepth: 1,
		IncludeLogic:    false,
		OutputFormat:    FormatPrompt,
		MaxCodeLines:    500,
		MaxCallers:      10,
		StrictMode:      false,
		IndexRequired:   false,
	}
}

// ============================================================================
// CONTEXT ASSEMBLER (Orchestrator)
// ============================================================================

// ContextAssembler is the primary orchestrator that coordinates the assembly
// of extraction context. It maintains separation of concerns by delegating
// to specialized components while providing a unified interface.
//
// The assembler follows the Facade pattern - it provides a simplified interface
// to a complex subsystem of loaders, resolvers, and formatters.
//
// Thread Safety: Implementations must be safe for concurrent use.
type ContextAssembler interface {
	// Assemble collects all requested context for a node and formats it.
	//
	// The assembly process:
	//   1. Load node specification (always required)
	//   2. Load implementation code (if IncludeImpl)
	//   3. Load test files (if IncludeTests)
	//   4. Resolve callers (if IncludeCallers)
	//   5. Load dependencies (up to DependencyDepth)
	//   6. Format and return output
	//
	// Errors are aggregated in AssemblyResult.Errors. Critical errors
	// that prevent assembly will be returned as the error value.
	Assemble(ctx context.Context, opts ExtractOptions) (*AssemblyResult, error)

	// ValidateOptions checks if the provided options are valid and
	// can be satisfied with available resources.
	ValidateOptions(opts ExtractOptions) error
}

// AssemblyResult contains the assembled context and metadata about the
// assembly process.
type AssemblyResult struct {
	// The formatted output
	Output string

	// Structured data for programmatic access
	Data *ExtractedContext

	// Assembly metadata
	AssemblyTime time.Duration
	NodeFound    bool
	ImplFound    bool
	TestsFound   int
	CallersFound int
	DepsResolved int

	// Errors encountered during assembly (non-critical)
	Errors []AssemblyError

	// Warnings about data quality or completeness
	Warnings []string
}

// AssemblyError represents a non-fatal error during assembly.
type AssemblyError struct {
	Component   string // Which component reported the error
	Operation   string // What operation was being performed
	Message     string
	Recoverable bool
}

func (e AssemblyError) Error() string {
	return fmt.Sprintf("[%s:%s] %s", e.Component, e.Operation, e.Message)
}

// ============================================================================
// SPEC LOADER (Port)
// ============================================================================

// SpecLoader handles loading and parsing of node specifications.
// This is the foundation of extraction - specs are always loaded first.
//
// The loader is independent of code analysis and works purely with
// YAML specification files.
type SpecLoader interface {
	// Load retrieves a node specification by name or path.
	// Returns ErrSpecNotFound if the spec cannot be located.
	Load(ctx context.Context, nodeName string, nodePath string) (*SpecLoadResult, error)

	// LoadAll loads all available node specifications.
	// Useful for dependency resolution.
	LoadAll(ctx context.Context, nodesDir string) ([]*SpecLoadResult, error)

	// ResolveDependencies finds and loads dependencies for a spec.
	// Returns specs up to the specified depth.
	ResolveDependencies(ctx context.Context, spec *NodeSpec, depth int, allSpecs map[string]*NodeSpec) ([]*DependencyInfo, error)
}

// SpecLoadResult contains a loaded specification and metadata.
type SpecLoadResult struct {
	Spec     *NodeSpec
	Path     string
	LoadTime time.Duration
	Errors   []error // Parse warnings, validation issues, etc.
}

// NodeSpec wraps the node.Spec type for the extract package.
// This provides a clean boundary between the node package and extract package.
type NodeSpec struct {
	ID              string
	Type            string
	Layer           string
	Namespace       string
	Responsibility  ResponsibilityInfo
	Interface       InterfaceInfo
	Dependencies    []DependencyRef
	Logic           LogicInfo
	Metadata        MetadataInfo
	SourcePath      string
	Implementations []string // Paths to implementation files
}

// ResponsibilityInfo describes what the node does.
type ResponsibilityInfo struct {
	Summary    string
	Details    string
	Invariants []string
	Boundaries string
}

// InterfaceInfo defines the public API.
type InterfaceInfo struct {
	Constructors []ConstructorInfo
	Methods      []MethodInfo
	Properties   []PropertyInfo
	Events       []EventInfo
}

// ConstructorInfo represents a constructor.
type ConstructorInfo struct {
	Signature   string
	Description string
	Parameters  []ParameterInfo
}

// MethodInfo represents a method.
type MethodInfo struct {
	Name        string
	Signature   string
	Description string
	Parameters  []ParameterInfo
	Returns     ReturnInfo
}

// ParameterInfo represents a method parameter.
type ParameterInfo struct {
	Name        string
	Type        string
	Description string
}

// ReturnInfo represents return type information.
type ReturnInfo struct {
	Type        string
	Description string
}

// PropertyInfo represents a property.
type PropertyInfo struct {
	Name        string
	Type        string
	Access      string
	Description string
}

// EventInfo represents an event.
type EventInfo struct {
	Name        string
	Signature   string
	Description string
}

// DependencyRef represents a dependency reference.
type DependencyRef struct {
	Target    string
	Type      string
	Injection string
	Optional  bool
	Usage     string
}

// LogicInfo contains implementation logic details.
type LogicInfo struct {
	StateMachine *StateMachineInfo
	Algorithms   []AlgorithmInfo
}

// StateMachineInfo represents state machine behavior.
type StateMachineInfo struct {
	Initial string
	States  []StateInfo
}

// StateInfo represents a state.
type StateInfo struct {
	Name        string
	Description string
	Transitions []TransitionInfo
}

// TransitionInfo represents a state transition.
type TransitionInfo struct {
	To      string
	Trigger string
}

// AlgorithmInfo describes an algorithm.
type AlgorithmInfo struct {
	Name        string
	Description string
}

// MetadataInfo contains node metadata.
type MetadataInfo struct {
	Status string
	Author string
	Tags   []string
	Notes  string
}

// DependencyInfo represents a resolved dependency with its spec.
type DependencyInfo struct {
	Target              string
	Type                string
	Injection           string
	Optional            bool
	Usage               string
	Spec                *NodeSpec
	InterfaceCode       string
	MissingDescriptions []string
}

// ============================================================================
// CODE LOADER (Port)
// ============================================================================

// CodeLoader retrieves implementation code from source files.
// This port abstracts the mechanism of code retrieval, allowing for
// different implementations: file system, code index, or language server.
//
// The loader follows the Strategy pattern - different strategies can be
// used depending on available resources and configuration.
type CodeLoader interface {
	// LoadImplementation retrieves the implementation code for a node.
	// Returns ErrCodeNotFound if implementation cannot be located.
	// Returns ErrCodeIndexUnavailable if index is required but not available.
	LoadImplementation(ctx context.Context, spec *NodeSpec) (*CodeLoadResult, error)

	// LoadFunction retrieves a specific function/method implementation.
	// Useful for loading individual methods when full implementation
	// is not needed or available.
	LoadFunction(ctx context.Context, spec *NodeSpec, functionName string) (*FunctionCode, error)

	// IsAvailable checks if the code loader has access to implementation code.
	// Returns true if LoadImplementation is expected to succeed.
	IsAvailable() bool
}

// CodeLoadResult contains loaded implementation code.
type CodeLoadResult struct {
	// Primary implementation file
	PrimaryFile *SourceFile

	// Additional implementation files (partial classes, extensions, etc.)
	AdditionalFiles []*SourceFile

	// Metadata
	TotalLines int
	Language   string
	FoundBy    string // "index", "filesystem", "heuristic"
	LoadTime   time.Duration
}

// SourceFile represents a source code file.
type SourceFile struct {
	Path         string
	Content      string
	Lines        int
	Language     string
	Checksum     string
	LastModified time.Time
}

// FunctionCode represents a single function's implementation.
type FunctionCode struct {
	Name      string
	Signature string
	Body      string
	StartLine int
	EndLine   int
	File      string
	Comments  string // Leading comments/documentation
}

// ============================================================================
// TEST MATCHER (Port)
// ============================================================================

// TestMatcher finds and loads test files related to a node.
// The matcher uses language-aware conventions to locate tests.
//
// Test discovery strategies:
//   - Naming convention: NodeName_test.go, NodeNameTests.cs, etc.
//   - Directory convention: tests/, __tests__/, spec/ folders
//   - Import analysis: Files that import the node
//   - Code index: Database of test relationships
type TestMatcher interface {
	// FindTests locates test files for a given node.
	// Returns empty slice if no tests found (not an error).
	FindTests(ctx context.Context, spec *NodeSpec) ([]*TestFile, error)

	// LoadTestContent loads the content of test files.
	LoadTestContent(ctx context.Context, tests []*TestFile) ([]*TestFileContent, error)

	// GetTestCoverage returns coverage information if available.
	// Returns nil if coverage data is not available.
	GetTestCoverage(ctx context.Context, spec *NodeSpec) (*TestCoverage, error)
}

// TestFile represents a discovered test file.
type TestFile struct {
	Path        string
	Name        string
	Framework   string  // "go test", "xUnit", "jest", etc.
	MatchScore  float64 // 0.0-1.0 confidence of relevance
	MatchReason string  // Why this file was matched
}

// TestFileContent contains loaded test file content.
type TestFileContent struct {
	*TestFile
	Content string
	Lines   int
}

// TestCoverage represents test coverage information.
type TestCoverage struct {
	OverallPercent float64
	ByFunction     map[string]float64
	LastUpdated    time.Time
}

// ============================================================================
// CALLER RESOLVER (Port)
// ============================================================================

// CallerResolver finds callers and references to functions.
// This enables "find usages" functionality without requiring
// a running language server.
//
// Resolution strategies:
//   - Static analysis: Parse source files to find call sites
//   - Code index: Query pre-built index of references
//   - Import analysis: Find files that import the target
type CallerResolver interface {
	// FindCallers finds all call sites for a node's methods.
	// Returns up to maxCallers results, ordered by relevance.
	FindCallers(ctx context.Context, spec *NodeSpec, maxCallers int) ([]*CallerInfo, error)

	// FindReferences finds all references to the node (imports, type usage, etc.).
	FindReferences(ctx context.Context, spec *NodeSpec, maxRefs int) ([]*ReferenceInfo, error)

	// FindFunctionCallers finds callers of a specific function.
	FindFunctionCallers(ctx context.Context, spec *NodeSpec, functionName string, maxCallers int) ([]*CallerInfo, error)

	// IsAvailable checks if caller resolution is available.
	IsAvailable() bool
}

// CallerInfo represents a function call site.
type CallerInfo struct {
	// Location
	File   string
	Line   int
	Column int

	// Context
	Function string // Function containing the call
	Class    string // Class/type containing the call
	Package  string // Package/module

	// Call details
	CallSnippet  string   // The actual call code
	ContextLines []string // Surrounding lines for context

	// Metadata
	Relevance float64 // 0.0-1.0 importance score
}

// ReferenceInfo represents a reference to a node.
type ReferenceInfo struct {
	File    string
	Line    int
	Column  int
	Type    string // "import", "type_reference", "method_call", etc.
	Snippet string
}

// ============================================================================
// OUTPUT FORMATTER (Port)
// ============================================================================

// OutputFormatter formats the assembled context into the desired output.
// Different formatters produce different output types: prompts, JSON, etc.
//
// The formatter receives the complete ExtractedContext and produces
// the final output string. This separation allows for:
//   - Multiple output formats from the same assembled data
//   - Template-based formatting
//   - Consistent structure across formats
type OutputFormatter interface {
	// Format converts the extracted context to the output format.
	Format(ctx context.Context, data *ExtractedContext, opts FormatOptions) (string, error)

	// FormatName returns the name of this formatter.
	FormatName() string

	// ContentType returns the MIME type of the output.
	ContentType() string
}

// FormatOptions configures formatting behavior.
type FormatOptions struct {
	Template        string            // Custom template name/path
	TemplateVars    map[string]string // Variables for template substitution
	MaxLength       int               // Maximum output length
	IncludeLineNums bool              // Include line numbers in code blocks
	Language        string            // Target language for syntax
}

// ExtractedContext contains all assembled data ready for formatting.
// This is the unified data structure passed to formatters.
type ExtractedContext struct {
	// Core specification
	Node         *NodeSpec
	Dependencies []*DependencyInfo

	// Optional code evidence
	Implementation *CodeLoadResult
	Tests          []*TestFileContent
	Callers        []*CallerInfo
	References     []*ReferenceInfo

	// Metadata
	Options     ExtractOptions
	AssembledAt time.Time
	Warnings    []string
}

// ============================================================================
// ERRORS
// ============================================================================

var (
	// ErrSpecNotFound is returned when a node specification cannot be found.
	ErrSpecNotFound = fmt.Errorf("specification not found")

	// ErrCodeNotFound is returned when implementation code cannot be found.
	ErrCodeNotFound = fmt.Errorf("implementation not found")

	// ErrCodeIndexUnavailable is returned when code index is required but unavailable.
	ErrCodeIndexUnavailable = fmt.Errorf("code index unavailable")

	// ErrInvalidOptions is returned when extract options are invalid.
	ErrInvalidOptions = fmt.Errorf("invalid extract options")

	// ErrAssemblyFailed is returned when assembly cannot be completed.
	ErrAssemblyFailed = fmt.Errorf("assembly failed")
)

// RecoverableError wraps an error to indicate it doesn't prevent assembly.
type RecoverableError struct {
	Component string
	Err       error
}

func (e *RecoverableError) Error() string {
	return fmt.Sprintf("%s: %v", e.Component, e.Err)
}

func (e *RecoverableError) Unwrap() error {
	return e.Err
}

// IsRecoverable checks if an error is recoverable.
func IsRecoverable(err error) bool {
	var rec *RecoverableError
	return err != nil && fmt.Errorf("%w", err) != nil && fmt.Sprintf("%T", rec) != ""
}

// ============================================================================
// FACTORY FUNCTIONS
// ============================================================================

// NewContextAssembler creates a default ContextAssembler with all components.
// This is the primary entry point for creating an assembler.
//
// The assembler uses file-based adapters by default, with optional
// database-backed code loading when available.
func NewContextAssembler(cfg *AssemblerConfig) (ContextAssembler, error) {
	// This will be implemented in context_assembler_impl.go
	return nil, fmt.Errorf("not implemented: use NewDefaultAssembler")
}

// AssemblerConfig configures the assembler and its components.
type AssemblerConfig struct {
	// Required
	ProjectRoot string
	NodesDir    string
	Language    string

	// Optional - if provided, enables database-backed code loading
	DatabasePath string

	// Optional - if provided, enables source directory scanning
	SourceDir string

	// Component overrides (for testing or customization)
	SpecLoader     SpecLoader
	CodeLoader     CodeLoader
	TestMatcher    TestMatcher
	CallerResolver CallerResolver
	Formatter      OutputFormatter
}

// NewDefaultAssembler creates a ContextAssembler with default file-based adapters.
// This works without any database or server process.
func NewDefaultAssembler(projectRoot, nodesDir string) (ContextAssembler, error) {
	// This will be implemented in context_assembler_impl.go
	return nil, fmt.Errorf("not implemented")
}

// NewIndexedAssembler creates a ContextAssembler with database-backed code resolution.
// Requires the GDC database to be present and initialized.
func NewIndexedAssembler(projectRoot, nodesDir, dbPath string) (ContextAssembler, error) {
	// This will be implemented in context_assembler_impl.go
	return nil, fmt.Errorf("not implemented")
}
