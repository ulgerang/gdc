// Package parser provides source code parsing functionality for extracting interface information
package parser

import (
	"fmt"
	"strings"

	"github.com/gdc-tools/gdc/internal/node"
)

// Parser defines the interface for language-specific parsers
type Parser interface {
	// ParseFile parses a source file and extracts interface information
	ParseFile(filePath string) (*ExtractedNode, error)

	// Language returns the language this parser handles
	Language() string
}

// MultiNodeParser is implemented by parsers that can extract multiple named
// types from a single source file.
type MultiNodeParser interface {
	ParseFileNodes(filePath string) ([]*ExtractedNode, error)
}

// NamedNodeParser is implemented by parsers that can select one named type
// from a source file containing multiple public declarations. Verification uses
// this before the legacy single-node ParseFile fallback so generic types and
// module-owned symbols are matched against their actual declaration.
type NamedNodeParser interface {
	ParseFileNode(filePath, nodeID string) (*ExtractedNode, error)
}

// ExtractedNode contains information extracted from source code
type ExtractedNode struct {
	ID        string
	Type      string // "class", "interface", "struct"
	Namespace string

	// Language-specific fields (for Hybrid Specification Strategy)
	Language   string   // "go", "csharp", "typescript", "rust", "python"
	Package    string   // Go: package name
	Module     string   // TS: module path
	Attributes []string // C#: class-level attributes

	Description string

	Constructors []ExtractedConstructor
	Methods      []ExtractedMethod
	Properties   []ExtractedProperty
	Events       []ExtractedEvent
	Dependencies []ExtractedDependency

	FilePath string
}

// ExtractedConstructor represents a constructor found in source
type ExtractedConstructor struct {
	Signature   string
	Parameters  []ExtractedParameter
	Description string // from doc comments
}

// ExtractedMethod represents a method found in source
type ExtractedMethod struct {
	Name        string
	Signature   string
	Parameters  []ExtractedParameter
	Returns     string
	Description string // from doc comments
	IsPublic    bool
	// Language-specific fields
	Exported   bool     // Go: starts with uppercase (exported)
	Async      bool     // TS/C#: async method
	Static     bool     // C#/TS: static method
	Access     string   // C#: public, private, internal, protected
	Attributes []string // C#: method-level attributes
}

// ExtractedProperty represents a property found in source
type ExtractedProperty struct {
	Name        string
	Type        string
	Access      string // "get", "set", "get; set"
	Description string
	IsPublic    bool
}

// ExtractedEvent represents an event found in source
type ExtractedEvent struct {
	Name        string
	Signature   string
	Description string
	IsPublic    bool
}

// ExtractedParameter represents a method parameter
type ExtractedParameter struct {
	Name string
	Type string
}

// ExtractedDependency represents a dependency found in source (constructor injection, field injection)
type ExtractedDependency struct {
	Target    string // type name
	Type      string // class, interface, module
	Namespace string // package/namespace hint for collision resolution
	FieldName string // injected field name
	Injection string // "constructor", "field", "property"
}

// ToNodeSpec converts extracted information to a node.Spec. Code-owned shape
// (signatures, current parameters, types, and access) comes from extraction;
// authored semantic and behavioral contract fields are preserved from oldSpec.
func (e *ExtractedNode) ToNodeSpec(oldSpec *node.Spec) *node.Spec {
	schemaVersion := "1.0"
	if oldSpec != nil && strings.TrimSpace(oldSpec.SchemaVersion) != "" {
		schemaVersion = oldSpec.SchemaVersion
	}
	spec := &node.Spec{
		SchemaVersion: schemaVersion,
		Node: node.NodeInfo{
			ID:        e.ID,
			Type:      e.Type,
			Namespace: e.Namespace,
		},
		// Language-specific section (Body of Hybrid Specification)
		LanguageSpec: node.LanguageSpec{
			Language:   e.Language,
			Package:    e.Package,    // Go-specific
			Module:     e.Module,     // TypeScript-specific
			Attributes: e.Attributes, // C#-specific
		},
	}

	// Preserve responsibility from old spec if exists
	if oldSpec != nil {
		spec.Responsibility = oldSpec.Responsibility
		spec.Node.Layer = oldSpec.Node.Layer
		spec.Metadata = oldSpec.Metadata
		spec.Logic = oldSpec.Logic
		spec.Interface.Types = append([]node.TypeContract(nil), oldSpec.Interface.Types...)
		spec.Implementations = append([]string(nil), oldSpec.Implementations...)
		spec.ImplementationContract = oldSpec.ImplementationContract
		spec.SyncPolicy = cloneSyncPolicy(oldSpec.SyncPolicy)
	}

	// Convert constructors
	for _, ctor := range e.Constructors {
		newCtor := node.Constructor{
			Signature:   ctor.Signature,
			Description: ctor.Description,
		}
		var oldCtor *node.Constructor
		if oldSpec != nil {
			oldCtor = findOldConstructor(oldSpec.Interface.Constructors, ctor.Signature)
		}
		if oldCtor != nil {
			if newCtor.Description == "" {
				newCtor.Description = oldCtor.Description
			}
			newCtor.Name = oldCtor.Name
			newCtor.Access = oldCtor.Access
			newCtor.Attributes = append([]string(nil), oldCtor.Attributes...)
		}
		newCtor.Parameters = mergeExtractedParameters(ctor.Parameters, parametersFromConstructor(oldCtor))
		spec.Interface.Constructors = append(spec.Interface.Constructors, newCtor)
	}

	// Convert methods (preserve old descriptions)
	for _, method := range e.Methods {
		if !method.IsPublic && e.Type != "function" {
			continue
		}
		newMethod := node.Method{
			Name:        method.Name,
			Signature:   method.Signature,
			Description: method.Description,
			// Language-specific fields (Hybrid Specification Body)
			Exported:   method.Exported,   // Go: uppercase function name
			Async:      method.Async,      // TS/C#: async method
			Static:     method.Static,     // C#/TS: static method
			Access:     method.Access,     // C#: access modifier
			Attributes: method.Attributes, // C#: method attributes
		}
		var oldMethod *node.Method
		if oldSpec != nil {
			oldMethod = findOldMethod(oldSpec.Interface.Methods, method.Name, method.Signature)
		}
		if oldMethod != nil {
			if newMethod.Description == "" {
				newMethod.Description = oldMethod.Description
			}
			newMethod.Throws = append([]node.Throws(nil), oldMethod.Throws...)
			newMethod.Preconditions = append([]string(nil), oldMethod.Preconditions...)
			newMethod.Postconditions = append([]string(nil), oldMethod.Postconditions...)
			newMethod.SideEffects = append([]string(nil), oldMethod.SideEffects...)
		}
		newMethod.Parameters = mergeExtractedParameters(method.Parameters, parametersFromMethod(oldMethod))
		newMethod.Returns = mergeExtractedReturns(method.Returns, returnsFromMethod(oldMethod))
		if oldMethod != nil && method.Returns == "" {
			newMethod.Returns.Type = oldMethod.Returns.Type
		}
		spec.Interface.Methods = append(spec.Interface.Methods, newMethod)
	}

	// Convert properties (preserve old descriptions)
	for _, prop := range e.Properties {
		if !prop.IsPublic {
			continue
		}
		newProp := node.Property{
			Name:        prop.Name,
			Type:        prop.Type,
			Access:      prop.Access,
			Description: prop.Description,
		}
		if oldSpec != nil {
			if oldProp := findOldProperty(oldSpec.Interface.Properties, prop.Name); oldProp != nil {
				if newProp.Description == "" {
					newProp.Description = oldProp.Description
				}
				if newProp.Type == "" {
					newProp.Type = oldProp.Type
				}
				if newProp.Access == "" {
					newProp.Access = oldProp.Access
				}
				newProp.Default = oldProp.Default
				newProp.Readonly = oldProp.Readonly
				newProp.Exported = oldProp.Exported
				newProp.Static = oldProp.Static
				newProp.Attributes = append([]string(nil), oldProp.Attributes...)
			}
		}
		spec.Interface.Properties = append(spec.Interface.Properties, newProp)
	}

	// Convert events (preserve old descriptions)
	for _, event := range e.Events {
		if !event.IsPublic {
			continue
		}
		newEvent := node.Event{
			Name:        event.Name,
			Signature:   event.Signature,
			Description: event.Description,
		}
		if oldSpec != nil {
			if oldEvent := findOldEvent(oldSpec.Interface.Events, event.Name); oldEvent != nil {
				if newEvent.Description == "" {
					newEvent.Description = oldEvent.Description
				}
				newEvent.Payload = oldEvent.Payload
			}
		}
		spec.Interface.Events = append(spec.Interface.Events, newEvent)
	}

	// Convert dependencies
	for _, dep := range e.Dependencies {
		depType := dep.Type
		if depType == "" {
			depType = "interface"
		}
		newDep := node.Dependency{
			Target:    dep.Target,
			Type:      depType,
			Injection: dep.Injection,
		}
		// Preserve old contract hash and usage
		if oldSpec != nil {
			if oldDep := findOldDependency(oldSpec.Dependencies, dep.Target); oldDep != nil {
				newDep.ContractHash = oldDep.ContractHash
				newDep.Usage = oldDep.Usage
				newDep.Optional = oldDep.Optional
				newDep.Type = oldDep.Type
				newDep.Requires = append([]string(nil), oldDep.Requires...)
			}
		}
		spec.Dependencies = append(spec.Dependencies, newDep)
	}

	return spec
}

func cloneSyncPolicy(policy *node.SyncPolicy) *node.SyncPolicy {
	if policy == nil {
		return nil
	}
	cloned := &node.SyncPolicy{Default: policy.Default}
	if len(policy.Ownership) > 0 {
		cloned.Ownership = make(map[string]string, len(policy.Ownership))
		for path, owner := range policy.Ownership {
			cloned.Ownership[path] = owner
		}
	}
	return cloned
}

func findOldConstructor(constructors []node.Constructor, signature string) *node.Constructor {
	for i := range constructors {
		if constructors[i].Signature == signature {
			return &constructors[i]
		}
	}
	if len(constructors) == 1 {
		return &constructors[0]
	}
	return nil
}

func findOldMethod(methods []node.Method, name, signature string) *node.Method {
	for i := range methods {
		if methods[i].Name == name && methods[i].Signature == signature {
			return &methods[i]
		}
	}
	var match *node.Method
	for i := range methods {
		if methods[i].Name != name {
			continue
		}
		if match != nil {
			return nil
		}
		match = &methods[i]
	}
	return match
}

func findOldProperty(properties []node.Property, name string) *node.Property {
	for i := range properties {
		if properties[i].Name == name {
			return &properties[i]
		}
	}
	return nil
}

func findOldEvent(events []node.Event, name string) *node.Event {
	for i := range events {
		if events[i].Name == name {
			return &events[i]
		}
	}
	return nil
}

func findOldDependency(dependencies []node.Dependency, target string) *node.Dependency {
	trimmedTarget := strings.TrimSpace(target)
	for i := range dependencies {
		if strings.TrimSpace(dependencies[i].Target) == trimmedTarget {
			return &dependencies[i]
		}
	}

	normalizedTarget := NormalizeTypeReference(trimmedTarget)
	for i := range dependencies {
		if NormalizeTypeReference(dependencies[i].Target) == normalizedTarget {
			return &dependencies[i]
		}
	}

	baseTarget := unqualifiedTypeReference(normalizedTarget)
	var match *node.Dependency
	for i := range dependencies {
		if unqualifiedTypeReference(NormalizeTypeReference(dependencies[i].Target)) != baseTarget {
			continue
		}
		if match != nil {
			return nil
		}
		match = &dependencies[i]
	}
	return match
}

func unqualifiedTypeReference(typeRef string) string {
	typeRef = strings.TrimSpace(typeRef)
	if index := strings.LastIndex(typeRef, "::"); index >= 0 {
		typeRef = typeRef[index+2:]
	}
	if index := strings.LastIndex(typeRef, "."); index >= 0 {
		typeRef = typeRef[index+1:]
	}
	return strings.TrimSpace(typeRef)
}

func parametersFromConstructor(constructor *node.Constructor) []node.Parameter {
	if constructor == nil {
		return nil
	}
	return constructor.Parameters
}

func parametersFromMethod(method *node.Method) []node.Parameter {
	if method == nil {
		return nil
	}
	return method.Parameters
}

func returnsFromMethod(method *node.Method) node.Returns {
	if method == nil {
		return node.Returns{}
	}
	return method.Returns
}

func mergeExtractedParameters(extracted []ExtractedParameter, authored []node.Parameter) []node.Parameter {
	merged := make([]node.Parameter, 0, len(extracted))
	for _, parameter := range extracted {
		result := node.Parameter{Name: parameter.Name, Type: parameter.Type}
		for _, oldParameter := range authored {
			if oldParameter.Name != parameter.Name {
				continue
			}
			result.Description = oldParameter.Description
			result.Optional = oldParameter.Optional
			result.Default = oldParameter.Default
			result.Constraint = oldParameter.Constraint
			result.Examples = append([]string(nil), oldParameter.Examples...)
			result.Enum = append([]string(nil), oldParameter.Enum...)
			break
		}
		merged = append(merged, result)
	}
	return merged
}

func mergeExtractedReturns(extractedType string, authored node.Returns) node.Returns {
	result := authored
	if strings.TrimSpace(extractedType) != "" {
		result.Type = extractedType
	}
	return result
}

// GetParser returns the appropriate parser for the given language
func GetParser(language string) (Parser, error) {
	switch language {
	case "go", "golang":
		return NewGoParser(), nil
	case "csharp", "cs", "c#":
		return NewCSharpParser(), nil
	case "typescript", "ts":
		return NewTypeScriptParser(), nil
	case "rust", "rs":
		return NewRustParser(), nil
	case "python", "py":
		return NewPythonParser(), nil
	case "gdscript", "gd":
		return NewGDScriptParser(), nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}

// LSPClient defines the interface for LSP server communication
type LSPClient interface {
	// Connect attempts to connect to an already running LSP server
	Connect(host string, port int) error

	// IsConnected returns true if connected to LSP server
	IsConnected() bool

	// Close closes the connection
	Close() error

	// GetSymbols retrieves symbols from a file
	GetSymbols(filePath string) ([]Symbol, error)
}

// Symbol represents an LSP symbol
type Symbol struct {
	Name     string
	Kind     SymbolKind
	Range    Range
	Children []Symbol
	Detail   string
}

// SymbolKind represents the type of symbol
type SymbolKind int

const (
	SymbolClass SymbolKind = iota
	SymbolInterface
	SymbolMethod
	SymbolProperty
	SymbolField
	SymbolConstructor
	SymbolEvent
)

// Range represents a position range in source
type Range struct {
	StartLine   int
	StartColumn int
	EndLine     int
	EndColumn   int
}
