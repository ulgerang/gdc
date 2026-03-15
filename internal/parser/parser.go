// Package parser provides source code parsing functionality for extracting interface information
package parser

import (
	"fmt"

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

// ExtractedNode contains information extracted from source code
type ExtractedNode struct {
	ID        string
	Type      string // "class", "interface", "struct"
	Namespace string

	// Language-specific fields (for Hybrid Specification Strategy)
	Language   string   // "go", "csharp", "typescript"
	Package    string   // Go: package name
	Module     string   // TS: module path
	Attributes []string // C#: class-level attributes

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

// ToNodeSpec converts extracted information to a node.Spec
// Preserves existing descriptions from the old spec
func (e *ExtractedNode) ToNodeSpec(oldSpec *node.Spec) *node.Spec {
	spec := &node.Spec{
		SchemaVersion: "1.0",
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
	}

	// Convert constructors
	for _, ctor := range e.Constructors {
		newCtor := node.Constructor{
			Signature:   ctor.Signature,
			Description: ctor.Description,
		}
		// Preserve old description if new one is empty
		if newCtor.Description == "" && oldSpec != nil {
			for _, oldCtor := range oldSpec.Interface.Constructors {
				if oldCtor.Signature == ctor.Signature {
					newCtor.Description = oldCtor.Description
					break
				}
			}
		}
		for _, p := range ctor.Parameters {
			newCtor.Parameters = append(newCtor.Parameters, node.Parameter{
				Name: p.Name,
				Type: p.Type,
			})
		}
		spec.Interface.Constructors = append(spec.Interface.Constructors, newCtor)
	}

	// Convert methods (preserve old descriptions)
	for _, method := range e.Methods {
		if !method.IsPublic {
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
		// Preserve old description if new one is empty
		if newMethod.Description == "" && oldSpec != nil {
			for _, oldMethod := range oldSpec.Interface.Methods {
				if oldMethod.Name == method.Name {
					newMethod.Description = oldMethod.Description
					newMethod.Parameters = oldMethod.Parameters
					newMethod.Returns = oldMethod.Returns
					newMethod.Throws = oldMethod.Throws
					break
				}
			}
		}
		// Add parameters from extracted
		if len(newMethod.Parameters) == 0 {
			for _, p := range method.Parameters {
				newMethod.Parameters = append(newMethod.Parameters, node.Parameter{
					Name: p.Name,
					Type: p.Type,
				})
			}
		}
		if newMethod.Returns.Type == "" && method.Returns != "" {
			newMethod.Returns = node.Returns{Type: method.Returns}
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
		// Preserve old description
		if newProp.Description == "" && oldSpec != nil {
			for _, oldProp := range oldSpec.Interface.Properties {
				if oldProp.Name == prop.Name {
					newProp.Description = oldProp.Description
					break
				}
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
		// Preserve old description
		if newEvent.Description == "" && oldSpec != nil {
			for _, oldEvent := range oldSpec.Interface.Events {
				if oldEvent.Name == event.Name {
					newEvent.Description = oldEvent.Description
					break
				}
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
			for _, oldDep := range oldSpec.Dependencies {
				if oldDep.Target == dep.Target {
					newDep.ContractHash = oldDep.ContractHash
					newDep.Usage = oldDep.Usage
					newDep.Optional = oldDep.Optional
					newDep.Type = oldDep.Type
					break
				}
			}
		}
		spec.Dependencies = append(spec.Dependencies, newDep)
	}

	return spec
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
