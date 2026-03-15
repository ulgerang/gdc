package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/atotto/clipboard"
	"github.com/gdc-tools/gdc/internal/config"
	extractctx "github.com/gdc-tools/gdc/internal/extract"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
)

var (
	extractTemplate     string
	extractOutput       string
	extractDepth        int
	extractIncludeLogic bool
	extractClipboard    bool
	// P2: New context extension flags
	extractWithImpl    bool
	extractWithTests   bool
	extractWithCallers bool
)

var extractCmd = &cobra.Command{
	Use:   "extract <node>",
	Short: "Generate AI implementation prompt",
	Long: `Generate an optimized prompt for AI implementation of a node.

This command extracts the node specification and its dependencies,
formatting them into a structured prompt optimized for AI code generation.

The generated prompt follows the Context Isolation principle:
  • Only includes the target node's specification
  • Only includes dependency interfaces (not their implementations)
  • Provides clear implementation guidelines

Optional Code Evidence (opt-in):
  • --with-impl: Include implementation code from source files
  • --with-tests: Include related test files
  • --with-callers: Include caller/references information

Examples:
  $ gdc extract PlayerController
  $ gdc extract PlayerController --clipboard
  $ gdc extract PlayerController --output prompt.md
  $ gdc extract PlayerController --template review
  $ gdc extract PlayerController --with-impl
  $ gdc extract PlayerController --with-impl --with-tests`,
	Args: cobra.ExactArgs(1),
	RunE: runExtract,
}

func init() {
	extractCmd.Flags().StringVarP(&extractTemplate, "template", "t", "implement",
		"prompt template (implement, review, test)")
	extractCmd.Flags().StringVarP(&extractOutput, "output", "o", "",
		"output file (default: stdout)")
	extractCmd.Flags().IntVarP(&extractDepth, "depth", "d", 1,
		"dependency inclusion depth")
	extractCmd.Flags().BoolVar(&extractIncludeLogic, "include-logic", false,
		"include internal logic specification")
	extractCmd.Flags().BoolVar(&extractClipboard, "clipboard", false,
		"copy to clipboard")

	// P2: Context extension flags
	extractCmd.Flags().BoolVar(&extractWithImpl, "with-impl", false,
		"include implementation code in output (opt-in)")
	extractCmd.Flags().BoolVar(&extractWithTests, "with-tests", false,
		"include related test files in output (opt-in)")
	extractCmd.Flags().BoolVar(&extractWithCallers, "with-callers", false,
		"include caller references in output (opt-in)")
}

func runExtract(cmd *cobra.Command, args []string) error {
	nodeName := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.Storage.NodesDir
	if nodesDir == "" {
		nodesDir = ".gdc/nodes"
	}

	// Load target node
	nodePath := filepath.Join(nodesDir, nodeName+".yaml")
	spec, err := node.Load(nodePath)
	if err != nil {
		return fmt.Errorf("node %s not found: %w", nodeName, err)
	}

	// Load all nodes for dependency resolution
	allNodes, _ := loadAllNodes(nodesDir)
	nodeMap := buildSpecLookup(allNodes)

	// Gather dependencies
	deps := gatherDependencies(spec, nodeMap, extractDepth)

	evidence, err := collectExtractEvidence(context.Background(), spec, cfg)
	if err != nil {
		return fmt.Errorf("failed to collect extract evidence: %w", err)
	}

	// Generate prompt
	prompt, err := generatePrompt(spec, deps, cfg, extractIncludeLogic, evidence)
	if err != nil {
		return fmt.Errorf("failed to generate prompt: %w", err)
	}

	// Output
	if extractOutput != "" {
		if err := os.WriteFile(extractOutput, []byte(prompt), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		printSuccess("Prompt saved to %s", extractOutput)
	} else if extractClipboard {
		if err := clipboard.WriteAll(prompt); err != nil {
			printWarning("Failed to copy to clipboard: %v", err)
			fmt.Println(prompt)
		} else {
			printSuccess("Prompt copied to clipboard (%d chars)", len(prompt))
		}
	} else {
		fmt.Println(prompt)
	}

	return nil
}

type DependencyInfo struct {
	Target              string
	Type                string
	Injection           string
	Optional            bool
	Usage               string
	InterfaceCode       string
	Spec                *node.Spec
	MissingDescriptions []string // List of methods/properties that need documentation
}

type extractEvidence struct {
	Implementation *extractctx.CodeLoadResult
	Tests          []*extractctx.TestFileContent
	Callers        []*extractctx.CallerInfo
	References     []*extractctx.ReferenceInfo
	Warnings       []string
}

func collectExtractEvidence(ctx context.Context, spec *node.Spec, cfg *config.Config) (extractEvidence, error) {
	evidence := extractEvidence{}
	if !extractWithImpl && !extractWithTests && !extractWithCallers {
		return evidence, nil
	}

	projectRoot := cfg.ProjectRoot
	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return evidence, err
		}
		projectRoot = cwd
	}

	extractSpec := toExtractNodeSpec(spec)
	if extractWithImpl {
		loader := extractctx.NewFileSystemCodeLoader(projectRoot)
		impl, err := loader.LoadImplementation(ctx, extractSpec)
		if err != nil {
			evidence.Warnings = append(evidence.Warnings, fmt.Sprintf("Implementation evidence unavailable: %v", err))
		} else {
			evidence.Implementation = impl
		}
	}

	if extractWithTests {
		matcher := extractctx.NewNamingConventionTestMatcher(projectRoot)
		tests, err := matcher.FindTests(ctx, extractSpec)
		if err != nil {
			evidence.Warnings = append(evidence.Warnings, fmt.Sprintf("Test evidence unavailable: %v", err))
		} else if len(tests) > 0 {
			contents, loadErr := matcher.LoadTestContent(ctx, tests)
			if loadErr != nil {
				evidence.Warnings = append(evidence.Warnings, fmt.Sprintf("Test content unavailable: %v", loadErr))
			} else {
				evidence.Tests = contents
			}
		}
	}

	if extractWithCallers {
		resolver := extractctx.NewSimpleCallerResolver(projectRoot)
		evidence.Warnings = append(evidence.Warnings, "Caller evidence collected via code search fallback.")

		callers, err := resolver.FindCallers(ctx, extractSpec, 10)
		if err != nil {
			evidence.Warnings = append(evidence.Warnings, fmt.Sprintf("Caller evidence unavailable: %v", err))
		} else {
			evidence.Callers = callers
		}

		refs, err := resolver.FindReferences(ctx, extractSpec, 10)
		if err != nil {
			evidence.Warnings = append(evidence.Warnings, fmt.Sprintf("Reference evidence unavailable: %v", err))
		} else {
			evidence.References = refs
		}
	}

	return evidence, nil
}

func toExtractNodeSpec(spec *node.Spec) *extractctx.NodeSpec {
	if spec == nil {
		return nil
	}

	out := &extractctx.NodeSpec{
		ID:         spec.Node.ID,
		Type:       spec.Node.Type,
		Layer:      spec.Node.Layer,
		Namespace:  spec.Node.Namespace,
		SourcePath: spec.Node.FilePath,
		Responsibility: extractctx.ResponsibilityInfo{
			Summary:    spec.Responsibility.Summary,
			Details:    spec.Responsibility.Details,
			Invariants: append([]string(nil), spec.Responsibility.Invariants...),
			Boundaries: spec.Responsibility.Boundaries,
		},
		Metadata: extractctx.MetadataInfo{
			Status: spec.Metadata.Status,
			Author: spec.Metadata.Author,
			Tags:   append([]string(nil), spec.Metadata.Tags...),
			Notes:  spec.Metadata.Notes,
		},
		Implementations: append([]string(nil), spec.Implementations...),
	}

	if out.SourcePath != "" && len(out.Implementations) == 0 {
		out.Implementations = append(out.Implementations, out.SourcePath)
	}

	for _, ctor := range spec.Interface.Constructors {
		info := extractctx.ConstructorInfo{
			Signature:   ctor.Signature,
			Description: ctor.Description,
		}
		for _, p := range ctor.Parameters {
			info.Parameters = append(info.Parameters, extractctx.ParameterInfo{
				Name:        p.Name,
				Type:        p.Type,
				Description: p.Description,
			})
		}
		out.Interface.Constructors = append(out.Interface.Constructors, info)
	}
	for _, method := range spec.Interface.Methods {
		info := extractctx.MethodInfo{
			Name:        method.Name,
			Signature:   method.Signature,
			Description: method.Description,
			Returns: extractctx.ReturnInfo{
				Type:        method.Returns.Type,
				Description: method.Returns.Description,
			},
		}
		for _, p := range method.Parameters {
			info.Parameters = append(info.Parameters, extractctx.ParameterInfo{
				Name:        p.Name,
				Type:        p.Type,
				Description: p.Description,
			})
		}
		out.Interface.Methods = append(out.Interface.Methods, info)
	}
	for _, prop := range spec.Interface.Properties {
		out.Interface.Properties = append(out.Interface.Properties, extractctx.PropertyInfo{
			Name:        prop.Name,
			Type:        prop.Type,
			Access:      prop.Access,
			Description: prop.Description,
		})
	}
	for _, event := range spec.Interface.Events {
		out.Interface.Events = append(out.Interface.Events, extractctx.EventInfo{
			Name:        event.Name,
			Signature:   event.Signature,
			Description: event.Description,
		})
	}
	for _, dep := range spec.Dependencies {
		out.Dependencies = append(out.Dependencies, extractctx.DependencyRef{
			Target:    dep.Target,
			Type:      dep.Type,
			Injection: dep.Injection,
			Optional:  dep.Optional,
			Usage:     dep.Usage,
		})
	}
	for _, algo := range spec.Logic.Algorithms {
		out.Logic.Algorithms = append(out.Logic.Algorithms, extractctx.AlgorithmInfo{
			Name:        algo.Name,
			Description: algo.Description,
		})
	}
	if spec.Logic.StateMachine != nil {
		sm := &extractctx.StateMachineInfo{
			Initial: spec.Logic.StateMachine.Initial,
		}
		for _, state := range spec.Logic.StateMachine.States {
			stateInfo := extractctx.StateInfo{
				Name:        state.Name,
				Description: state.Description,
			}
			for _, tr := range state.Transitions {
				stateInfo.Transitions = append(stateInfo.Transitions, extractctx.TransitionInfo{
					To:      tr.To,
					Trigger: tr.Trigger,
				})
			}
			sm.States = append(sm.States, stateInfo)
		}
		out.Logic.StateMachine = sm
	}

	return out
}

func gatherDependencies(spec *node.Spec, nodeMap map[string]*node.Spec, depth int) []DependencyInfo {
	var deps []DependencyInfo
	seen := make(map[string]bool)

	var gather func(s *node.Spec, d int)
	gather = func(s *node.Spec, d int) {
		if d <= 0 {
			return
		}

		for _, dep := range s.Dependencies {
			if seen[dep.Target] {
				continue
			}
			seen[dep.Target] = true

			depSpec, exists := nodeMap[dep.Target]
			info := DependencyInfo{
				Target:    dep.Target,
				Type:      dep.Type,
				Injection: dep.Injection,
				Optional:  dep.Optional,
				Usage:     dep.Usage,
			}

			if exists {
				info.Spec = depSpec
				info.InterfaceCode = generateInterfaceCode(depSpec)
				// Collect missing descriptions
				info.MissingDescriptions = collectMissingDescriptions(depSpec)
			} else {
				info.InterfaceCode = fmt.Sprintf("// %s - specification not found", dep.Target)
			}

			deps = append(deps, info)

			if exists && d > 1 {
				gather(depSpec, d-1)
			}
		}
	}

	gather(spec, depth)
	return deps
}

func generateInterfaceCode(spec *node.Spec) string {
	var sb strings.Builder

	// Header
	if spec.Node.Type == "interface" {
		sb.WriteString(fmt.Sprintf("interface %s {\n", spec.Node.ID))
	} else {
		sb.WriteString(fmt.Sprintf("class %s {\n", spec.Node.ID))
	}

	// Constructors
	for _, ctor := range spec.Interface.Constructors {
		if ctor.Description != "" {
			sb.WriteString(fmt.Sprintf("    // %s\n", ctor.Description))
		} else {
			sb.WriteString("    // ⚠️ [NEEDS DESCRIPTION]\n")
		}
		sb.WriteString(fmt.Sprintf("    %s;\n", ctor.Signature))
	}

	if len(spec.Interface.Constructors) > 0 && len(spec.Interface.Methods) > 0 {
		sb.WriteString("\n")
	}

	// Methods
	for _, method := range spec.Interface.Methods {
		if method.Description != "" {
			sb.WriteString(fmt.Sprintf("    // %s\n", method.Description))
		} else {
			sb.WriteString("    // ⚠️ [NEEDS DESCRIPTION]\n")
		}
		sb.WriteString(fmt.Sprintf("    %s;\n", method.Signature))
	}

	// Properties
	if len(spec.Interface.Properties) > 0 {
		sb.WriteString("\n")
		for _, prop := range spec.Interface.Properties {
			if prop.Description != "" {
				sb.WriteString(fmt.Sprintf("    // %s\n", prop.Description))
			} else {
				sb.WriteString("    // ⚠️ [NEEDS DESCRIPTION]\n")
			}
			sb.WriteString(fmt.Sprintf("    %s %s { %s; }\n", prop.Type, prop.Name, prop.Access))
		}
	}

	// Events
	if len(spec.Interface.Events) > 0 {
		sb.WriteString("\n")
		for _, event := range spec.Interface.Events {
			if event.Description != "" {
				sb.WriteString(fmt.Sprintf("    // %s\n", event.Description))
			} else {
				sb.WriteString("    // ⚠️ [NEEDS DESCRIPTION]\n")
			}
			sb.WriteString(fmt.Sprintf("    %s;\n", event.Signature))
		}
	}

	sb.WriteString("}")
	return sb.String()
}

// collectMissingDescriptions returns a list of members that lack documentation
func collectMissingDescriptions(spec *node.Spec) []string {
	var missing []string

	for _, ctor := range spec.Interface.Constructors {
		if strings.TrimSpace(ctor.Description) == "" {
			missing = append(missing, "constructor")
		}
	}
	for _, method := range spec.Interface.Methods {
		if strings.TrimSpace(method.Description) == "" {
			missing = append(missing, method.Name)
		}
	}
	for _, prop := range spec.Interface.Properties {
		if strings.TrimSpace(prop.Description) == "" {
			missing = append(missing, prop.Name)
		}
	}
	for _, event := range spec.Interface.Events {
		if strings.TrimSpace(event.Description) == "" {
			missing = append(missing, event.Name)
		}
	}

	return missing
}

func generatePrompt(spec *node.Spec, deps []DependencyInfo, cfg *config.Config, includeLogic bool, evidence extractEvidence) (string, error) {
	const promptTemplate = `# Implementation Request: {{.Node.ID}}

## Context
This is a structured implementation request. Implement the following {{.Node.Type}} according to the specification.
Use ONLY the provided interfaces - do not assume or access any external systems not listed here.

---

## Target Node Specification

### Basic Information
- **Name**: ` + "`{{.Node.ID}}`" + `
- **Type**: {{.Node.Type}}
- **Layer**: {{.Node.Layer}}
{{- if .Node.Namespace}}
- **Namespace**: ` + "`{{.Node.Namespace}}`" + `
{{- end}}

### Responsibility
{{.Responsibility.Summary}}
{{if .Responsibility.Details}}
**Details:**
{{.Responsibility.Details}}
{{end}}

{{if .Responsibility.Invariants}}
### Invariants (Must Always Hold)
{{range .Responsibility.Invariants}}
- {{.}}
{{end}}
{{end}}

---

## Public Interface to Implement

` + "```{{.Config.Language}}" + `
{{.InterfaceCode}}
` + "```" + `

{{if .Interface.Methods}}
### Method Specifications
{{range .Interface.Methods}}
#### ` + "`{{.Name}}`" + `
{{- if .Description}}
{{.Description}}
{{- end}}
{{- if .Parameters}}

**Parameters:**
{{- range .Parameters}}
- ` + "`{{.Name}}`" + ` ({{.Type}}): {{.Description}}{{if .Constraint}} [{{.Constraint}}]{{end}}
{{- end}}
{{- end}}
{{- if .Returns.Type}}

**Returns:** ` + "`{{.Returns.Type}}`" + `{{if .Returns.Description}} - {{.Returns.Description}}{{end}}
{{- end}}
{{- if .Throws}}

**Throws:**
{{- range .Throws}}
- ` + "`{{.Type}}`" + `: {{.Condition}}
{{- end}}
{{- end}}

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

{{range .Callers}}
### {{.Function}} ({{.File}}:{{.Line}})
{{if .Package}}**Package**: {{.Package}}{{end}}

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

{{if .HasReferences}}
## Usage Evidence (References)

{{range .References}}
- ` + "`{{.File}}:{{.Line}}`" + ` ({{.Type}}): {{.Snippet}}
{{end}}

{{end}}

{{if .IncludeLogic}}
{{if .Logic.StateMachine}}
## State Machine

- **Initial State**: ` + "`{{.Logic.StateMachine.Initial}}`" + `

| State | Transitions |
|-------|-------------|
{{range .Logic.StateMachine.States}}| {{.Name}} | {{range $i, $t := .Transitions}}{{if $i}}, {{end}}→ {{$t.To}} ({{$t.Trigger}}){{end}} |
{{end}}
{{end}}

{{if .Logic.Algorithms}}
## Algorithms
{{range .Logic.Algorithms}}
### {{.Name}}
{{.Description}}
{{end}}
{{end}}
{{end}}

{{if .Warnings}}
## Warnings
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

{{if .Metadata.Notes}}
## Additional Notes
{{.Metadata.Notes}}
{{end}}
`

	// Prepare template data
	type TemplateData struct {
		Node              node.NodeInfo
		Responsibility    node.Responsibility
		Interface         node.Interface
		Config            struct{ Language string }
		InterfaceCode     string
		Dependencies      []DependencyInfo
		IncludeLogic      bool
		Logic             node.Logic
		Metadata          node.Metadata
		HasImplementation bool
		Implementation    *extractctx.CodeLoadResult
		HasTests          bool
		Tests             []*extractctx.TestFileContent
		HasCallers        bool
		Callers           []*extractctx.CallerInfo
		HasReferences     bool
		References        []*extractctx.ReferenceInfo
		Warnings          []string
	}

	data := TemplateData{
		Node:              spec.Node,
		Responsibility:    spec.Responsibility,
		Interface:         spec.Interface,
		Config:            struct{ Language string }{Language: cfg.Project.Language},
		InterfaceCode:     generateInterfaceCodeForLanguage(spec, cfg.Project.Language),
		Dependencies:      deps,
		IncludeLogic:      includeLogic,
		Logic:             spec.Logic,
		Metadata:          spec.Metadata,
		HasImplementation: evidence.Implementation != nil,
		Implementation:    evidence.Implementation,
		HasTests:          len(evidence.Tests) > 0,
		Tests:             evidence.Tests,
		HasCallers:        len(evidence.Callers) > 0,
		Callers:           evidence.Callers,
		HasReferences:     len(evidence.References) > 0,
		References:        evidence.References,
		Warnings:          evidence.Warnings,
	}

	// Try to load language-specific template
	templateContent := promptTemplate
	if cfg.Project.Language != "" {
		langTemplate := loadLanguageTemplate(cfg, cfg.Project.Language)
		if langTemplate != "" {
			templateContent = langTemplate
		}
	}

	tmpl, err := template.New("prompt").Parse(templateContent)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// loadLanguageTemplate attempts to load a language-specific template file
func loadLanguageTemplate(cfg *config.Config, language string) string {
	// Map language to template filename
	var templateName string
	switch strings.ToLower(language) {
	case "go", "golang":
		templateName = "implement.go.md.j2"
	case "csharp", "cs", "c#":
		templateName = "implement.csharp.md.j2"
	case "typescript", "ts":
		templateName = "implement.typescript.md.j2"
	default:
		return ""
	}

	templatePath := filepath.Join(cfg.TemplatesDir(), templateName)
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "" // Fall back to default template
	}

	// Convert Jinja2 syntax to Go template syntax (basic conversion)
	result := string(content)
	result = strings.ReplaceAll(result, "{{-", "{{")
	result = strings.ReplaceAll(result, "-}}", "}}")
	result = strings.ReplaceAll(result, "{#", "{{/*")
	result = strings.ReplaceAll(result, "#}", "*/}}")
	result = strings.ReplaceAll(result, "{% if ", "{{if .")
	result = strings.ReplaceAll(result, "{% elif ", "{{else if .")
	result = strings.ReplaceAll(result, "{% else %}", "{{else}}")
	result = strings.ReplaceAll(result, "{% endif %}", "{{end}}")
	result = strings.ReplaceAll(result, "{% for ", "{{range .")
	result = strings.ReplaceAll(result, "{% endfor %}", "{{end}}")
	result = strings.ReplaceAll(result, " %}", "}}")

	return result
}

// generateInterfaceCodeForLanguage generates interface code in the appropriate language syntax
func generateInterfaceCodeForLanguage(spec *node.Spec, language string) string {
	switch strings.ToLower(language) {
	case "go", "golang":
		return generateGoInterfaceCode(spec)
	case "csharp", "cs", "c#":
		return generateCSharpInterfaceCode(spec)
	case "typescript", "ts":
		return generateTypeScriptInterfaceCode(spec)
	default:
		return generateInterfaceCode(spec)
	}
}

func generateGoInterfaceCode(spec *node.Spec) string {
	var sb strings.Builder

	if spec.Node.Type == "interface" {
		sb.WriteString(fmt.Sprintf("type %s interface {\n", spec.Node.ID))
	} else {
		sb.WriteString(fmt.Sprintf("type %s struct {\n", spec.Node.ID))
	}

	// Properties as struct fields
	for _, prop := range spec.Interface.Properties {
		if prop.Description != "" {
			sb.WriteString(fmt.Sprintf("    // %s\n", prop.Description))
		} else {
			sb.WriteString("    // ⚠️ [NEEDS DESCRIPTION]\n")
		}
		sb.WriteString(fmt.Sprintf("    %s %s\n", strings.Title(prop.Name), prop.Type))
	}

	if len(spec.Interface.Properties) > 0 && len(spec.Interface.Methods) > 0 {
		sb.WriteString("\n")
	}

	// Methods
	for _, method := range spec.Interface.Methods {
		if method.Description != "" {
			sb.WriteString(fmt.Sprintf("    // %s\n", method.Description))
		} else {
			sb.WriteString("    // ⚠️ [NEEDS DESCRIPTION]\n")
		}
		sb.WriteString(fmt.Sprintf("    %s\n", method.Signature))
	}

	sb.WriteString("}")
	return sb.String()
}

func generateCSharpInterfaceCode(spec *node.Spec) string {
	var sb strings.Builder

	keyword := "class"
	if spec.Node.Type == "interface" {
		keyword = "interface"
	}

	sb.WriteString(fmt.Sprintf("public %s %s\n{{\n", keyword, spec.Node.ID))

	// Constructors
	for _, ctor := range spec.Interface.Constructors {
		if ctor.Description != "" {
			sb.WriteString(fmt.Sprintf("    /// <summary>%s</summary>\n", ctor.Description))
		} else {
			sb.WriteString("    /// <summary>⚠️ [NEEDS DESCRIPTION]</summary>\n")
		}
		sb.WriteString(fmt.Sprintf("    %s;\n", ctor.Signature))
	}

	if len(spec.Interface.Constructors) > 0 {
		sb.WriteString("\n")
	}

	// Properties
	for _, prop := range spec.Interface.Properties {
		if prop.Description != "" {
			sb.WriteString(fmt.Sprintf("    /// <summary>%s</summary>\n", prop.Description))
		} else {
			sb.WriteString("    /// <summary>⚠️ [NEEDS DESCRIPTION]</summary>\n")
		}
		sb.WriteString(fmt.Sprintf("    %s %s {{ %s; }}\n", prop.Type, prop.Name, prop.Access))
	}

	if len(spec.Interface.Properties) > 0 {
		sb.WriteString("\n")
	}

	// Methods
	for _, method := range spec.Interface.Methods {
		if method.Description != "" {
			sb.WriteString(fmt.Sprintf("    /// <summary>%s</summary>\n", method.Description))
		} else {
			sb.WriteString("    /// <summary>⚠️ [NEEDS DESCRIPTION]</summary>\n")
		}
		sb.WriteString(fmt.Sprintf("    %s;\n", method.Signature))
	}

	// Events
	for _, event := range spec.Interface.Events {
		if event.Description != "" {
			sb.WriteString(fmt.Sprintf("    /// <summary>%s</summary>\n", event.Description))
		} else {
			sb.WriteString("    /// <summary>⚠️ [NEEDS DESCRIPTION]</summary>\n")
		}
		sb.WriteString(fmt.Sprintf("    %s;\n", event.Signature))
	}

	sb.WriteString("}}")
	return sb.String()
}

func generateTypeScriptInterfaceCode(spec *node.Spec) string {
	var sb strings.Builder

	keyword := "class"
	if spec.Node.Type == "interface" {
		keyword = "interface"
	}

	sb.WriteString(fmt.Sprintf("export %s %s {{\n", keyword, spec.Node.ID))

	// Properties
	for _, prop := range spec.Interface.Properties {
		if prop.Description != "" {
			sb.WriteString(fmt.Sprintf("  /** %s */\n", prop.Description))
		} else {
			sb.WriteString("  /** ⚠️ [NEEDS DESCRIPTION] */\n")
		}
		readonly := ""
		if prop.Access == "get" {
			readonly = "readonly "
		}
		sb.WriteString(fmt.Sprintf("  %s%s: %s;\n", readonly, toCamelCase(prop.Name), prop.Type))
	}

	if len(spec.Interface.Properties) > 0 && len(spec.Interface.Methods) > 0 {
		sb.WriteString("\n")
	}

	// Methods
	for _, method := range spec.Interface.Methods {
		if method.Description != "" {
			sb.WriteString(fmt.Sprintf("  /** %s */\n", method.Description))
		} else {
			sb.WriteString("  /** ⚠️ [NEEDS DESCRIPTION] */\n")
		}
		sb.WriteString(fmt.Sprintf("  %s;\n", method.Signature))
	}

	sb.WriteString("}}")
	return sb.String()
}

func toCamelCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
