// Package node handles node specification parsing and management
package node

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Spec represents a complete node specification
// Follows the Hybrid Specification Strategy:
// - Header (Universal): Node, Dependencies, Metadata - for graph traversal and integrity checks
// - Body (Language-Specific): LanguageSpec, Interface details - for LLM code generation
type Spec struct {
	SchemaVersion   string         `yaml:"schema_version"`
	Node            NodeInfo       `yaml:"node"`
	LanguageSpec    LanguageSpec   `yaml:"language_spec,omitempty"` // Language-specific configuration
	Responsibility  Responsibility `yaml:"responsibility"`
	Interface       Interface      `yaml:"interface,omitempty"`
	Dependencies    []Dependency   `yaml:"dependencies,omitempty"`
	Implementations []string       `yaml:"implementations,omitempty"`
	Logic           Logic          `yaml:"logic,omitempty"`
	Metadata        Metadata       `yaml:"metadata"`
	SourcePath      string         `yaml:"-"`
}

// NodeInfo contains basic node identification
type NodeInfo struct {
	ID        string `yaml:"id"`
	Type      string `yaml:"type"`            // class, interface, module, service, enum, function
	Layer     string `yaml:"layer,omitempty"` // domain, application, infrastructure, presentation
	Namespace string `yaml:"namespace,omitempty"`
	FilePath  string `yaml:"file_path,omitempty"`
}

// LanguageSpec contains language-specific metadata for hybrid specification strategy
// Header (Universal): Identity, Metadata, Relationships - used for graph traversal
// Body (Language-Specific): This struct - used for LLM code generation
type LanguageSpec struct {
	Language   string   `yaml:"language,omitempty"`   // csharp, go, typescript
	Package    string   `yaml:"package,omitempty"`    // Go: package name
	Module     string   `yaml:"module,omitempty"`     // TS: module path
	Attributes []string `yaml:"attributes,omitempty"` // C#: class-level attributes like [Serializable]
}

// Responsibility describes what the node does
type Responsibility struct {
	Summary    string   `yaml:"summary"`
	Details    string   `yaml:"details,omitempty"`
	Invariants []string `yaml:"invariants,omitempty"`
	Boundaries string   `yaml:"boundaries,omitempty"`
}

// Interface defines the public API of the node
type Interface struct {
	Constructors []Constructor `yaml:"constructors,omitempty"`
	Methods      []Method      `yaml:"methods,omitempty"`
	Properties   []Property    `yaml:"properties,omitempty"`
	Events       []Event       `yaml:"events,omitempty"`
}

// Constructor defines a constructor
type Constructor struct {
	Signature   string      `yaml:"signature"`
	Parameters  []Parameter `yaml:"parameters,omitempty"`
	Description string      `yaml:"description,omitempty"`
	// Language-specific fields (Body)
	Access     string   `yaml:"access,omitempty"`     // C#: public, private, internal, protected
	Attributes []string `yaml:"attributes,omitempty"` // C#: [Inject], [SerializeField], etc.
}

// Method defines a method
type Method struct {
	Name        string      `yaml:"name"`
	Signature   string      `yaml:"signature"`
	Description string      `yaml:"description,omitempty"`
	Parameters  []Parameter `yaml:"parameters,omitempty"`
	Returns     Returns     `yaml:"returns,omitempty"`
	Throws      []Throws    `yaml:"throws,omitempty"`
	// Language-specific fields (Body) - LLM uses these for accurate code generation
	Async      bool     `yaml:"async,omitempty"`      // TS: Promise return, C#: async/await
	Access     string   `yaml:"access,omitempty"`     // C#: public, private, internal, protected
	Exported   bool     `yaml:"exported,omitempty"`   // Go: starts with uppercase (exported)
	Static     bool     `yaml:"static,omitempty"`     // C#/TS: static method
	Virtual    bool     `yaml:"virtual,omitempty"`    // C#: virtual/override
	Abstract   bool     `yaml:"abstract,omitempty"`   // C#/TS: abstract method
	Attributes []string `yaml:"attributes,omitempty"` // C#: [SerializeField], [Inject], etc.
}

// Parameter defines a method parameter
type Parameter struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description,omitempty"`
	Optional    bool   `yaml:"optional,omitempty"`
	Default     string `yaml:"default,omitempty"`
	Constraint  string `yaml:"constraint,omitempty"`
}

// Returns defines method return info
type Returns struct {
	Type        string `yaml:"type,omitempty"`
	Description string `yaml:"description,omitempty"`
	Nullable    bool   `yaml:"nullable,omitempty"`
}

// Throws defines an exception that can be thrown
type Throws struct {
	Type      string `yaml:"type"`
	Condition string `yaml:"condition,omitempty"`
}

// Property defines a property
type Property struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Access      string `yaml:"access,omitempty"` // get, set, get/set
	Description string `yaml:"description,omitempty"`
	Default     string `yaml:"default,omitempty"`
	// Language-specific fields (Body)
	Readonly   bool     `yaml:"readonly,omitempty"`   // TS: readonly modifier
	Exported   bool     `yaml:"exported,omitempty"`   // Go: starts with uppercase
	Static     bool     `yaml:"static,omitempty"`     // C#/TS: static property
	Attributes []string `yaml:"attributes,omitempty"` // C#: [SerializeField], [JsonIgnore], etc.
}

// Event defines an event
type Event struct {
	Name        string `yaml:"name"`
	Signature   string `yaml:"signature"`
	Description string `yaml:"description,omitempty"`
	Payload     string `yaml:"payload,omitempty"`
}

// Dependency defines a dependency on another node
type Dependency struct {
	Target       string `yaml:"target"`
	Type         string `yaml:"type,omitempty"`      // interface, class, module
	Injection    string `yaml:"injection,omitempty"` // constructor, property, method
	Optional     bool   `yaml:"optional,omitempty"`
	Usage        string `yaml:"usage,omitempty"`
	ContractHash string `yaml:"contract_hash,omitempty"`
}

// Logic contains internal implementation details
type Logic struct {
	StateMachine *StateMachine `yaml:"state_machine,omitempty"`
	Algorithms   []Algorithm   `yaml:"algorithms,omitempty"`
	DataFlow     string        `yaml:"data_flow,omitempty"`
	Pseudocode   string        `yaml:"pseudocode,omitempty"`
}

// StateMachine defines state machine behavior
type StateMachine struct {
	Initial string  `yaml:"initial"`
	States  []State `yaml:"states"`
}

// State defines a state
type State struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description,omitempty"`
	OnEnter     string       `yaml:"on_enter,omitempty"`
	OnExit      string       `yaml:"on_exit,omitempty"`
	Transitions []Transition `yaml:"transitions,omitempty"`
}

// Transition defines a state transition
type Transition struct {
	To      string `yaml:"to"`
	Trigger string `yaml:"trigger"`
	Guard   string `yaml:"guard,omitempty"`
	Action  string `yaml:"action,omitempty"`
}

// Algorithm describes an algorithm
type Algorithm struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Complexity  string `yaml:"complexity,omitempty"`
	Steps       string `yaml:"steps,omitempty"`
}

// Metadata contains additional node information
type Metadata struct {
	Status       string   `yaml:"status"` // draft, specified, implemented, tested, deprecated
	Origin       string   `yaml:"origin,omitempty"`
	ExtractedAt  string   `yaml:"extracted_at,omitempty"`
	Created      string   `yaml:"created,omitempty"`
	Updated      string   `yaml:"updated,omitempty"`
	Author       string   `yaml:"author,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
	SpecHash     string   `yaml:"spec_hash,omitempty"`
	ImplHash     string   `yaml:"impl_hash,omitempty"`
	Notes        string   `yaml:"notes,omitempty"`
	SRPThreshold int      `yaml:"srp_threshold,omitempty"` // Per-node SRP threshold override (0 = use global)
	Layer        string   `yaml:"layer,omitempty"`         // Optional layer override
}

// Load reads a node specification from a YAML file
func Load(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	// Validate required fields
	if spec.Node.ID == "" {
		return nil, fmt.Errorf("node.id is required in %s", path)
	}
	if spec.Node.Type == "" {
		spec.Node.Type = "class"
	}
	if spec.Metadata.Status == "" {
		spec.Metadata.Status = "draft"
	}
	if spec.SchemaVersion == "" {
		spec.SchemaVersion = "1.0"
	}
	spec.SourcePath = path

	return &spec, nil
}

// Save writes a node specification to a YAML file
func Save(path string, spec *Spec) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Validate checks if the specification is valid
func (s *Spec) Validate() []string {
	var errors []string

	if s.Node.ID == "" {
		errors = append(errors, "node.id is required")
	}

	validTypes := map[string]bool{
		"class": true, "interface": true, "module": true,
		"service": true, "enum": true, "function": true,
	}
	if !validTypes[s.Node.Type] {
		errors = append(errors, fmt.Sprintf("invalid node.type: %s", s.Node.Type))
	}

	validStatuses := map[string]bool{
		"draft": true, "specified": true, "implemented": true,
		"tested": true, "deprecated": true,
	}
	if !validStatuses[s.Metadata.Status] {
		errors = append(errors, fmt.Sprintf("invalid metadata.status: %s", s.Metadata.Status))
	}

	if s.Responsibility.Summary == "" {
		errors = append(errors, "responsibility.summary is required")
	}

	return errors
}

// GetDependencyTargets returns a list of all dependency targets
func (s *Spec) GetDependencyTargets() []string {
	targets := make([]string, len(s.Dependencies))
	for i, dep := range s.Dependencies {
		targets[i] = dep.Target
	}
	return targets
}

// HasDependency checks if the node depends on the given target
func (s *Spec) HasDependency(target string) bool {
	for _, dep := range s.Dependencies {
		if dep.Target == target {
			return true
		}
	}
	return false
}

func (s *Spec) QualifiedID() string {
	if s == nil {
		return ""
	}
	return s.Node.QualifiedID()
}

func (n NodeInfo) QualifiedID() string {
	if n.Namespace == "" {
		return n.ID
	}
	return n.Namespace + "." + n.ID
}
