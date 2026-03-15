package extract

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
)

// PromptFormatter formats extracted context as an AI-optimized prompt.
type PromptFormatter struct {
	templateStr string
}

// NewPromptFormatter creates a new prompt formatter with default template.
func NewPromptFormatter() *PromptFormatter {
	return &PromptFormatter{
		templateStr: defaultPromptTemplate,
	}
}

// NewPromptFormatterWithTemplate creates a formatter with a custom template.
func NewPromptFormatterWithTemplate(template string) *PromptFormatter {
	return &PromptFormatter{
		templateStr: template,
	}
}

// FormatName returns the formatter name.
func (f *PromptFormatter) FormatName() string {
	return "prompt"
}

// ContentType returns the MIME type.
func (f *PromptFormatter) ContentType() string {
	return "text/markdown"
}

// Format converts the extracted context to a prompt.
func (f *PromptFormatter) Format(ctx context.Context, data *ExtractedContext, opts FormatOptions) (string, error) {
	type TemplateData struct {
		Node         *NodeSpec
		Dependencies []*DependencyInfo
		Config       struct{ Language string }
		IncludeLogic bool
		// Code evidence sections
		HasImplementation bool
		Implementation    *CodeLoadResult
		HasTests          bool
		Tests             []*TestFileContent
		HasCallers        bool
		Callers           []*CallerInfo
		// Warnings
		Warnings []string
	}

	templateData := TemplateData{
		Node:              data.Node,
		Dependencies:      data.Dependencies,
		Config:            struct{ Language string }{Language: opts.Language},
		IncludeLogic:      data.Options.IncludeLogic,
		HasImplementation: data.Implementation != nil,
		Implementation:    data.Implementation,
		HasTests:          len(data.Tests) > 0,
		Tests:             data.Tests,
		HasCallers:        len(data.Callers) > 0,
		Callers:           data.Callers,
		Warnings:          data.Warnings,
	}

	tmpl, err := template.New("prompt").Parse(f.templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// defaultPromptTemplate is the default template for prompt generation.
const defaultPromptTemplate = `# Implementation Request: {{.Node.ID}}

## Context
This is a structured implementation request. Implement the following {{.Node.Type}} according to the specification.
Use ONLY the provided interfaces - do not assume or access any external systems not listed here.

---

## Target Node Specification

### Basic Information
- **Name**: ` + "`{{.Node.ID}}`" + `
- **Type**: {{.Node.Type}}
- **Layer**: {{.Node.Layer}}
{{if .Node.Namespace}}
- **Namespace**: ` + "`{{.Node.Namespace}}`" + `
{{end}}

### Responsibility
{{.Node.Responsibility.Summary}}
{{if .Node.Responsibility.Details}}
**Details:**
{{.Node.Responsibility.Details}}
{{end}}

{{if .Node.Responsibility.Invariants}}
### Invariants (Must Always Hold)
{{range .Node.Responsibility.Invariants}}
- {{.}}
{{end}}
{{end}}

---

## Public Interface to Implement

` + "```{{.Config.Language}}" + `
{{generateInterfaceCode .Node}}
` + "```" + `

{{if .Node.Interface.Methods}}
### Method Specifications
{{range .Node.Interface.Methods}}
#### ` + "`{{.Name}}`" + `
{{if .Description}}
{{.Description}}
{{end}}
{{if .Parameters}}
**Parameters:**
{{range .Parameters}}
- ` + "`{{.Name}}`" + ` ({{.Type}}): {{.Description}}
{{end}}
{{end}}
{{if .Returns.Type}}
**Returns:** ` + "`{{.Returns.Type}}`" + `{{if .Returns.Description}} - {{.Returns.Description}}{{end}}
{{end}}
{{end}}
{{end}}

---

{{if .Dependencies}}
## Available Dependencies (Contracts)

Use ONLY these interfaces. Do not access any other systems.

{{range .Dependencies}}
### {{.Target}}{{if .Optional}} (Optional){{end}}

**Injection**: {{.Injection}}

` + "```{{$.Config.Language}}" + `
{{.InterfaceCode}}
` + "```" + `
{{if .Usage}}
**Usage Notes:**
{{.Usage}}
{{end}}

---
{{end}}
{{end}}

{{if .HasImplementation}}
## Implementation Code Evidence

{{if .Implementation.PrimaryFile}}
### Primary Implementation File
**File**: ` + "`{{.Implementation.PrimaryFile.Path}}`" + `

` + "```{{.Implementation.Language}}" + `
{{.Implementation.PrimaryFile.Content}}
` + "```" + `
{{end}}

{{range .Implementation.AdditionalFiles}}
### Additional File
**File**: ` + "`{{.Path}}`" + `

` + "```{{.Language}}" + `
{{.Content}}
` + "```" + `
{{end}}

---
{{end}}

{{if .HasTests}}
## Test Code Evidence

The following tests exist for this component. Consider these when implementing:

{{range .Tests}}
### {{.Name}}
**Framework**: {{.Framework}} | **Lines**: {{.Lines}}
**Path**: ` + "`{{.Path}}`" + `

` + "```{{$.Config.Language}}" + `
{{.Content}}
` + "```" + `

---
{{end}}
{{end}}

{{if .HasCallers}}
## Usage Evidence (Callers)

This component is called from the following locations:

{{range .Callers}}
### {{.Function}} ({{.File}}:{{.Line}})
**Package**: {{.Package}}
**Relevance Score**: {{.Relevance}}

` + "```{{$.Config.Language}}" + `
{{.CallSnippet}}
` + "```" + `

{{if .ContextLines}}
**Context:**
{{range .ContextLines}}
{{.}}
{{end}}
{{end}}

---
{{end}}
{{end}}

{{if .Warnings}}
## ⚠️ Warnings

{{range .Warnings}}
- {{.}}
{{end}}

{{end}}

## Implementation Guidelines

1. **Strict Interface Compliance**: Implement EXACTLY the interface specified above.
2. **Dependency Injection Only**: Use only the provided dependencies. No direct instantiation.
3. **Maintain Invariants**: Ensure all listed invariants hold after every public method.
4. **Error Handling**: Handle edge cases. Throw specified exceptions with clear messages.
5. **Code Style**: Follow {{.Config.Language}} idiomatic conventions.

{{if .Node.Metadata.Notes}}
## Additional Notes
{{.Node.Metadata.Notes}}
{{end}}
`

// generateInterfaceCode generates interface code for a node spec.
// This is a template function.
func generateInterfaceCode(node *NodeSpec) string {
	var sb strings.Builder

	// Header
	if node.Type == "interface" {
		sb.WriteString(fmt.Sprintf("interface %s {\n", node.ID))
	} else {
		sb.WriteString(fmt.Sprintf("class %s {\n", node.ID))
	}

	// Constructors
	for _, ctor := range node.Interface.Constructors {
		if ctor.Description != "" {
			sb.WriteString(fmt.Sprintf("    // %s\n", ctor.Description))
		} else {
			sb.WriteString("    // ⚠️ [NEEDS DESCRIPTION]\n")
		}
		sb.WriteString(fmt.Sprintf("    %s;\n", ctor.Signature))
	}

	if len(node.Interface.Constructors) > 0 && len(node.Interface.Methods) > 0 {
		sb.WriteString("\n")
	}

	// Methods
	for _, method := range node.Interface.Methods {
		if method.Description != "" {
			sb.WriteString(fmt.Sprintf("    // %s\n", method.Description))
		} else {
			sb.WriteString("    // ⚠️ [NEEDS DESCRIPTION]\n")
		}
		sb.WriteString(fmt.Sprintf("    %s;\n", method.Signature))
	}

	// Properties
	if len(node.Interface.Properties) > 0 {
		sb.WriteString("\n")
		for _, prop := range node.Interface.Properties {
			if prop.Description != "" {
				sb.WriteString(fmt.Sprintf("    // %s\n", prop.Description))
			} else {
				sb.WriteString("    // ⚠️ [NEEDS DESCRIPTION]\n")
			}
			sb.WriteString(fmt.Sprintf("    %s %s { %s; }\n", prop.Type, prop.Name, prop.Access))
		}
	}

	sb.WriteString("}")
	return sb.String()
}
