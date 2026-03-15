// Package codegen provides language-specific code generation utilities
package codegen

import (
	"fmt"
	"strings"

	"github.com/gdc-tools/gdc/internal/node"
)

// Language represents a supported programming language
type Language string

const (
	LangGo         Language = "go"
	LangCSharp     Language = "csharp"
	LangTypeScript Language = "typescript"
)

// Generator generates language-specific code representations
type Generator struct {
	language Language
}

// NewGenerator creates a new code generator for the specified language
func NewGenerator(lang string) (*Generator, error) {
	switch strings.ToLower(lang) {
	case "go", "golang":
		return &Generator{language: LangGo}, nil
	case "csharp", "cs", "c#":
		return &Generator{language: LangCSharp}, nil
	case "typescript", "ts":
		return &Generator{language: LangTypeScript}, nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}

// Language returns the language name as a string
func (g *Generator) Language() string {
	return string(g.language)
}

// MemberInfo contains information about a member with documentation status
type MemberInfo struct {
	Name             string
	Signature        string
	Description      string
	NeedsDescription bool   // true if description is empty/missing
	MemberType       string // "method", "property", "event", "constructor"
}

// InterfaceInfo contains extracted interface information with documentation status
type InterfaceInfo struct {
	NodeID       string
	NodeType     string
	Members      []MemberInfo
	HasMissing   bool // true if any member needs description
	MissingCount int
}

// AnalyzeSpec analyzes a node spec and returns documentation status
func AnalyzeSpec(spec *node.Spec) *InterfaceInfo {
	info := &InterfaceInfo{
		NodeID:   spec.Node.ID,
		NodeType: spec.Node.Type,
		Members:  make([]MemberInfo, 0),
	}

	// Analyze constructors
	for _, ctor := range spec.Interface.Constructors {
		needsDesc := strings.TrimSpace(ctor.Description) == ""
		info.Members = append(info.Members, MemberInfo{
			Name:             "constructor",
			Signature:        ctor.Signature,
			Description:      ctor.Description,
			NeedsDescription: needsDesc,
			MemberType:       "constructor",
		})
		if needsDesc {
			info.HasMissing = true
			info.MissingCount++
		}
	}

	// Analyze methods
	for _, method := range spec.Interface.Methods {
		needsDesc := strings.TrimSpace(method.Description) == ""
		info.Members = append(info.Members, MemberInfo{
			Name:             method.Name,
			Signature:        method.Signature,
			Description:      method.Description,
			NeedsDescription: needsDesc,
			MemberType:       "method",
		})
		if needsDesc {
			info.HasMissing = true
			info.MissingCount++
		}
	}

	// Analyze properties
	for _, prop := range spec.Interface.Properties {
		needsDesc := strings.TrimSpace(prop.Description) == ""
		info.Members = append(info.Members, MemberInfo{
			Name:             prop.Name,
			Signature:        fmt.Sprintf("%s %s { %s; }", prop.Type, prop.Name, prop.Access),
			Description:      prop.Description,
			NeedsDescription: needsDesc,
			MemberType:       "property",
		})
		if needsDesc {
			info.HasMissing = true
			info.MissingCount++
		}
	}

	// Analyze events
	for _, event := range spec.Interface.Events {
		needsDesc := strings.TrimSpace(event.Description) == ""
		info.Members = append(info.Members, MemberInfo{
			Name:             event.Name,
			Signature:        event.Signature,
			Description:      event.Description,
			NeedsDescription: needsDesc,
			MemberType:       "event",
		})
		if needsDesc {
			info.HasMissing = true
			info.MissingCount++
		}
	}

	return info
}

// GenerateInterface generates language-specific interface code
func (g *Generator) GenerateInterface(spec *node.Spec) string {
	switch g.language {
	case LangGo:
		return g.generateGoInterface(spec)
	case LangCSharp:
		return g.generateCSharpInterface(spec)
	case LangTypeScript:
		return g.generateTypeScriptInterface(spec)
	default:
		return g.generateCSharpInterface(spec)
	}
}

// Go interface generation
func (g *Generator) generateGoInterface(spec *node.Spec) string {
	var sb strings.Builder
	info := AnalyzeSpec(spec)

	if spec.Node.Type == "interface" {
		sb.WriteString(fmt.Sprintf("type %s interface {\n", spec.Node.ID))
	} else {
		sb.WriteString(fmt.Sprintf("type %s struct {\n", spec.Node.ID))
		// For structs, show fields from properties
		for _, prop := range spec.Interface.Properties {
			sb.WriteString(fmt.Sprintf("\t%s %s\n", prop.Name, convertToGoType(prop.Type)))
		}
		sb.WriteString("}\n\n")
		// Methods as receivers
		for _, member := range info.Members {
			if member.MemberType == "method" {
				if member.NeedsDescription {
					sb.WriteString("// ⚠️ [NEEDS DESCRIPTION]\n")
				} else if member.Description != "" {
					sb.WriteString(fmt.Sprintf("// %s\n", member.Description))
				}
				sb.WriteString(fmt.Sprintf("func (s *%s) %s\n\n", spec.Node.ID, convertSignatureToGo(member.Signature)))
			}
		}
		return sb.String()
	}

	// Interface methods
	for _, member := range info.Members {
		if member.MemberType == "method" {
			if member.NeedsDescription {
				sb.WriteString("\t// ⚠️ [NEEDS DESCRIPTION]\n")
			} else if member.Description != "" {
				sb.WriteString(fmt.Sprintf("\t// %s\n", member.Description))
			}
			sb.WriteString(fmt.Sprintf("\t%s\n", convertSignatureToGo(member.Signature)))
		}
	}

	sb.WriteString("}")
	return sb.String()
}

// C# interface generation
func (g *Generator) generateCSharpInterface(spec *node.Spec) string {
	var sb strings.Builder
	info := AnalyzeSpec(spec)

	keyword := "class"
	if spec.Node.Type == "interface" {
		keyword = "interface"
	}

	sb.WriteString(fmt.Sprintf("public %s %s\n{\n", keyword, spec.Node.ID))

	// Constructors
	for _, member := range info.Members {
		if member.MemberType == "constructor" {
			if member.NeedsDescription {
				sb.WriteString("    /// <summary>⚠️ [NEEDS DESCRIPTION]</summary>\n")
			} else if member.Description != "" {
				sb.WriteString(fmt.Sprintf("    /// <summary>%s</summary>\n", member.Description))
			}
			sb.WriteString(fmt.Sprintf("    public %s;\n\n", member.Signature))
		}
	}

	// Properties
	for _, member := range info.Members {
		if member.MemberType == "property" {
			if member.NeedsDescription {
				sb.WriteString("    /// <summary>⚠️ [NEEDS DESCRIPTION]</summary>\n")
			} else if member.Description != "" {
				sb.WriteString(fmt.Sprintf("    /// <summary>%s</summary>\n", member.Description))
			}
			sb.WriteString(fmt.Sprintf("    public %s\n\n", member.Signature))
		}
	}

	// Methods
	for _, member := range info.Members {
		if member.MemberType == "method" {
			if member.NeedsDescription {
				sb.WriteString("    /// <summary>⚠️ [NEEDS DESCRIPTION]</summary>\n")
			} else if member.Description != "" {
				sb.WriteString(fmt.Sprintf("    /// <summary>%s</summary>\n", member.Description))
			}
			sb.WriteString(fmt.Sprintf("    public %s;\n\n", member.Signature))
		}
	}

	// Events
	for _, member := range info.Members {
		if member.MemberType == "event" {
			if member.NeedsDescription {
				sb.WriteString("    /// <summary>⚠️ [NEEDS DESCRIPTION]</summary>\n")
			} else if member.Description != "" {
				sb.WriteString(fmt.Sprintf("    /// <summary>%s</summary>\n", member.Description))
			}
			sb.WriteString(fmt.Sprintf("    public %s;\n\n", member.Signature))
		}
	}

	sb.WriteString("}")
	return sb.String()
}

// TypeScript interface generation
func (g *Generator) generateTypeScriptInterface(spec *node.Spec) string {
	var sb strings.Builder
	info := AnalyzeSpec(spec)

	keyword := "class"
	if spec.Node.Type == "interface" {
		keyword = "interface"
	}

	sb.WriteString(fmt.Sprintf("export %s %s {\n", keyword, spec.Node.ID))

	// Constructor (only for class)
	if keyword == "class" {
		for _, member := range info.Members {
			if member.MemberType == "constructor" {
				if member.NeedsDescription {
					sb.WriteString("    /** ⚠️ [NEEDS DESCRIPTION] */\n")
				} else if member.Description != "" {
					sb.WriteString(fmt.Sprintf("    /** %s */\n", member.Description))
				}
				sb.WriteString(fmt.Sprintf("    %s;\n\n", convertSignatureToTS(member.Signature)))
			}
		}
	}

	// Properties
	for _, member := range info.Members {
		if member.MemberType == "property" {
			if member.NeedsDescription {
				sb.WriteString("    /** ⚠️ [NEEDS DESCRIPTION] */\n")
			} else if member.Description != "" {
				sb.WriteString(fmt.Sprintf("    /** %s */\n", member.Description))
			}
			sb.WriteString(fmt.Sprintf("    %s;\n\n", convertPropertyToTS(member.Name, member.Signature)))
		}
	}

	// Methods
	for _, member := range info.Members {
		if member.MemberType == "method" {
			if member.NeedsDescription {
				sb.WriteString("    /** ⚠️ [NEEDS DESCRIPTION] */\n")
			} else if member.Description != "" {
				sb.WriteString(fmt.Sprintf("    /** %s */\n", member.Description))
			}
			sb.WriteString(fmt.Sprintf("    %s;\n\n", convertSignatureToTS(member.Signature)))
		}
	}

	sb.WriteString("}")
	return sb.String()
}

// Helper functions for type conversion

func convertToGoType(csType string) string {
	typeMap := map[string]string{
		"int":     "int",
		"string":  "string",
		"bool":    "bool",
		"float":   "float32",
		"double":  "float64",
		"void":    "",
		"Vector2": "Vector2",
		"Vector3": "Vector3",
	}
	if goType, ok := typeMap[csType]; ok {
		return goType
	}
	return csType
}

func convertSignatureToGo(sig string) string {
	// Basic conversion: "void Move(Vector2 direction)" -> "Move(direction Vector2)"
	// This is a simplified conversion - real implementation would need proper parsing
	return sig // For now, return as-is with a note
}

func convertSignatureToTS(sig string) string {
	// Basic conversion: "void Move(Vector2 direction)" -> "move(direction: Vector2): void"
	// This is a simplified conversion
	return sig // For now, return as-is
}

func convertPropertyToTS(name, sig string) string {
	// Extract type from signature like "bool IsActive { get; }"
	// Add readonly if signature contains "get" but not "set"
	isReadonly := strings.Contains(sig, "get") && !strings.Contains(sig, "set")
	prefix := ""
	if isReadonly {
		prefix = "readonly "
	}

	// Try to extract type from signature
	typeName := "any"
	parts := strings.Fields(sig)
	if len(parts) > 0 {
		typeName = convertCSharpTypeToTS(parts[0])
	}

	propName := strings.ToLower(name[:1]) + name[1:]
	return fmt.Sprintf("%s%s: %s", prefix, propName, typeName)
}

func convertCSharpTypeToTS(csType string) string {
	typeMap := map[string]string{
		"int":     "number",
		"float":   "number",
		"double":  "number",
		"string":  "string",
		"bool":    "boolean",
		"void":    "void",
		"boolean": "boolean",
	}
	if tsType, ok := typeMap[csType]; ok {
		return tsType
	}
	return csType
}
