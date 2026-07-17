package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
)

var (
	showDeps      bool
	showRefs      bool
	showFull      bool
	showInterface bool
	showFormat    string
)

var showCmd = &cobra.Command{
	Use:   "show <node>",
	Short: "Show detailed node information",
	Long: `Display detailed information about a specific node.

Examples:
  $ gdc show PlayerController
  $ gdc show PlayerController --deps --refs
  $ gdc show IInputManager --full`,
	Args: cobra.ExactArgs(1),
	RunE: runShow,
}

func init() {
	showCmd.Flags().BoolVarP(&showDeps, "deps", "d", false, "show dependencies")
	showCmd.Flags().BoolVarP(&showRefs, "refs", "r", false, "show references (nodes that depend on this)")
	showCmd.Flags().BoolVarP(&showFull, "full", "F", false, "show full specification")
	showCmd.Flags().BoolVarP(&showInterface, "interface-only", "i", false, "show interface only")
	showCmd.Flags().StringVar(&showFormat, "format", "text", "output format (text, json)")
}

func runShow(cmd *cobra.Command, args []string) error {
	showFormat = resolveFormat(showFormat)
	nodeName := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.NodesDir()

	// Load all nodes for reference lookup
	allNodes, err := loadAllNodes(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}
	spec := buildSpecLookup(allNodes)[nodeName]
	if spec == nil {
		return fmt.Errorf("node %s not found", nodeName)
	}

	if showInterface {
		printInterfaceOnly(spec, cfg.Project.Language)
		return nil
	}

	// JSON output
	if showFormat == "json" {
		return outputShowJSON(spec, allNodes)
	}

	// Print header
	printNodeHeader(spec)

	// Print responsibility
	printResponsibility(spec)

	// Print interface
	printInterface(spec)

	// Print dependencies if requested
	if showDeps || showFull {
		printDependencies(spec.Dependencies)
	}

	// Print references if requested
	if showRefs || showFull {
		refs := findReferences(spec.QualifiedID(), allNodes)
		printReferences(refs)
	}

	// Print metadata
	if showFull {
		printMetadata(spec)
	}

	return nil
}

func printNodeHeader(spec *node.Spec) {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	fmt.Println(strings.Repeat("═", 70))
	_, _ = bold.Printf("  %s\n", spec.Node.ID)
	fmt.Println(strings.Repeat("═", 70))

	fmt.Printf("  Type: %s | Layer: %s | Status: %s\n",
		cyan.Sprint(spec.Node.Type),
		cyan.Sprint(spec.Node.Layer),
		formatStatus(spec.Metadata.Status),
	)

	if spec.Node.Namespace != "" {
		fmt.Printf("  Namespace: %s\n", spec.Node.Namespace)
	}
	if spec.Node.FilePath != "" {
		fmt.Printf("  File: %s\n", spec.Node.FilePath)
	}

	fmt.Println(strings.Repeat("─", 70))
}

func printResponsibility(spec *node.Spec) {
	bold := color.New(color.Bold)

	_, _ = bold.Println("  Responsibility:")
	fmt.Printf("  %s\n", spec.Responsibility.Summary)

	if spec.Responsibility.Details != "" {
		fmt.Println()
		lines := strings.Split(spec.Responsibility.Details, "\n")
		for _, line := range lines {
			fmt.Printf("    %s\n", strings.TrimSpace(line))
		}
	}

	if len(spec.Responsibility.Invariants) > 0 {
		fmt.Println()
		_, _ = bold.Println("  Invariants:")
		for _, inv := range spec.Responsibility.Invariants {
			fmt.Printf("    • %s\n", inv)
		}
	}

	fmt.Println(strings.Repeat("─", 70))
}

func printInterface(spec *node.Spec) {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)

	_, _ = bold.Println("  Interface:")

	// Constructors
	if len(spec.Interface.Constructors) > 0 {
		for _, ctor := range spec.Interface.Constructors {
			_, _ = green.Printf("    ⊕ %s\n", ctor.Signature)
		}
	}

	// Methods
	for _, method := range spec.Interface.Methods {
		_, _ = green.Printf("    ▸ %s\n", method.Signature)
		if method.Description != "" && !quiet {
			fmt.Printf("      %s\n", color.HiBlackString(method.Description))
		}
	}

	// Properties
	for _, prop := range spec.Interface.Properties {
		fmt.Printf("    ◦ %s: %s {%s}\n", prop.Name, prop.Type, prop.Access)
	}

	// Events
	for _, event := range spec.Interface.Events {
		fmt.Printf("    ⚡ %s\n", event.Signature)
	}

	fmt.Println(strings.Repeat("─", 70))
}

func printDependencies(deps []node.Dependency) {
	bold := color.New(color.Bold)

	_, _ = bold.Println("  Dependencies (→):")

	if len(deps) == 0 {
		fmt.Println("    (none)")
	} else {
		for _, dep := range deps {
			optional := ""
			if dep.Optional {
				optional = color.YellowString(" [optional]")
			}
			fmt.Printf("    → %s [%s]%s\n",
				color.CyanString(dep.Target),
				dep.Type,
				optional,
			)
		}
	}

	fmt.Println(strings.Repeat("─", 70))
}

func findReferences(nodeName string, allNodes []*node.Spec) []string {
	var refs []string
	lookup := buildSpecLookup(allNodes)
	nodeName = resolveNodeAlias(nodeName, lookup)
	for _, n := range allNodes {
		for _, dep := range n.Dependencies {
			if resolveNodeAlias(dep.Target, lookup) == nodeName {
				refs = append(refs, n.QualifiedID())
				break
			}
		}
	}
	return refs
}

func printReferences(refs []string) {
	bold := color.New(color.Bold)

	_, _ = bold.Println("  Referenced by (←):")

	if len(refs) == 0 {
		fmt.Println("    (none)")
	} else {
		for _, ref := range refs {
			fmt.Printf("    ← %s\n", color.CyanString(ref))
		}
	}

	fmt.Println(strings.Repeat("─", 70))
}

func printMetadata(spec *node.Spec) {
	bold := color.New(color.Bold)

	_, _ = bold.Println("  Metadata:")
	fmt.Printf("    Created: %s | Updated: %s\n",
		spec.Metadata.Created, spec.Metadata.Updated)

	if len(spec.Metadata.Tags) > 0 {
		fmt.Printf("    Tags: %s\n", strings.Join(spec.Metadata.Tags, ", "))
	}

	if spec.Metadata.SpecHash != "" {
		fmt.Printf("    Spec Hash: %s\n", spec.Metadata.SpecHash)
	}
	if spec.Metadata.ImplHash != "" {
		fmt.Printf("    Impl Hash: %s\n", spec.Metadata.ImplHash)
	}

	fmt.Println(strings.Repeat("═", 70))
}

func printInterfaceOnly(spec *node.Spec, language string) {
	fmt.Printf("// %s interface\n\n", spec.Node.ID)

	switch spec.Node.Type {
	case "interface":
		fmt.Printf("interface %s {\n", spec.Node.ID)
	case "class":
		fmt.Printf("class %s {\n", spec.Node.ID)
	default:
		fmt.Printf("// %s\n{\n", spec.Node.ID)
	}

	for _, ctor := range spec.Interface.Constructors {
		fmt.Printf("    %s;\n", ctor.Signature)
	}

	if len(spec.Interface.Constructors) > 0 && len(spec.Interface.Methods) > 0 {
		fmt.Println()
	}

	for _, method := range spec.Interface.Methods {
		fmt.Printf("    %s;\n", method.Signature)
	}

	if len(spec.Interface.Properties) > 0 {
		fmt.Println()
		for _, prop := range spec.Interface.Properties {
			fmt.Printf("    %s %s { %s; }\n", prop.Type, prop.Name, prop.Access)
		}
	}

	if len(spec.Interface.Events) > 0 {
		fmt.Println()
		for _, event := range spec.Interface.Events {
			fmt.Printf("    %s;\n", event.Signature)
		}
	}

	fmt.Println("}")
}

type showNodeJSON struct {
	Node            showNodeInfoJSON       `json:"node"`
	Responsibility  showResponsibilityJSON `json:"responsibility"`
	Interface       showInterfaceJSON      `json:"interface"`
	Dependencies    []showDependencyJSON   `json:"dependencies"`
	Implementations []string               `json:"implementations,omitempty"`
	Metadata        showMetadataJSON       `json:"metadata"`
	References      []string               `json:"references,omitempty"`
}

type showNodeInfoJSON struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Layer     string `json:"layer"`
	Namespace string `json:"namespace,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
}

type showResponsibilityJSON struct {
	Summary    string   `json:"summary"`
	Details    string   `json:"details,omitempty"`
	Invariants []string `json:"invariants,omitempty"`
	Boundaries string   `json:"boundaries,omitempty"`
}

type showInterfaceJSON struct {
	Constructors []showConstructorJSON `json:"constructors,omitempty"`
	Methods      []showMethodJSON      `json:"methods,omitempty"`
	Properties   []showPropertyJSON    `json:"properties,omitempty"`
	Events       []showEventJSON       `json:"events,omitempty"`
}

type showConstructorJSON struct {
	Signature   string              `json:"signature"`
	Description string              `json:"description,omitempty"`
	Parameters  []showParameterJSON `json:"parameters,omitempty"`
}

type showMethodJSON struct {
	Name        string              `json:"name"`
	Signature   string              `json:"signature"`
	Description string              `json:"description,omitempty"`
	Parameters  []showParameterJSON `json:"parameters,omitempty"`
	Returns     *showReturnsJSON    `json:"returns,omitempty"`
	Async       bool                `json:"async"`
	Access      string              `json:"access,omitempty"`
}

type showParameterJSON struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type showReturnsJSON struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type showPropertyJSON struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Access      string `json:"access,omitempty"`
	Description string `json:"description,omitempty"`
}

type showEventJSON struct {
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	Description string `json:"description,omitempty"`
}

type showDependencyJSON struct {
	Target    string `json:"target"`
	Type      string `json:"type,omitempty"`
	Injection string `json:"injection,omitempty"`
	Optional  bool   `json:"optional"`
}

type showMetadataJSON struct {
	Status   string   `json:"status"`
	Created  string   `json:"created,omitempty"`
	Updated  string   `json:"updated,omitempty"`
	Author   string   `json:"author,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	SpecHash string   `json:"spec_hash,omitempty"`
	ImplHash string   `json:"impl_hash,omitempty"`
}

func outputShowJSON(spec *node.Spec, allNodes []*node.Spec) error {
	out := buildShowNodeJSON(spec, allNodes)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func buildShowNodeJSON(spec *node.Spec, allNodes []*node.Spec) showNodeJSON {
	out := showNodeJSON{
		Node: showNodeInfoJSON{
			ID:        spec.Node.ID,
			Type:      spec.Node.Type,
			Layer:     spec.Node.Layer,
			Namespace: spec.Node.Namespace,
			FilePath:  spec.Node.FilePath,
		},
		Responsibility: showResponsibilityJSON{
			Summary:    spec.Responsibility.Summary,
			Details:    spec.Responsibility.Details,
			Invariants: spec.Responsibility.Invariants,
			Boundaries: spec.Responsibility.Boundaries,
		},
		Implementations: spec.Implementations,
		Metadata: showMetadataJSON{
			Status:   spec.Metadata.Status,
			Created:  spec.Metadata.Created,
			Updated:  spec.Metadata.Updated,
			Author:   spec.Metadata.Author,
			Tags:     spec.Metadata.Tags,
			SpecHash: spec.Metadata.SpecHash,
			ImplHash: spec.Metadata.ImplHash,
		},
	}

	out.Interface = buildShowInterfaceJSON(spec)
	out.Dependencies = buildShowDependenciesJSON(spec)
	out.References = findReferences(spec.QualifiedID(), allNodes)

	return out
}

func buildShowInterfaceJSON(spec *node.Spec) showInterfaceJSON {
	iface := showInterfaceJSON{}

	for _, ctor := range spec.Interface.Constructors {
		params := make([]showParameterJSON, len(ctor.Parameters))
		for i, p := range ctor.Parameters {
			params[i] = showParameterJSON{
				Name:        p.Name,
				Type:        p.Type,
				Description: p.Description,
			}
		}
		iface.Constructors = append(iface.Constructors, showConstructorJSON{
			Signature:   ctor.Signature,
			Description: ctor.Description,
			Parameters:  params,
		})
	}

	for _, m := range spec.Interface.Methods {
		params := make([]showParameterJSON, len(m.Parameters))
		for i, p := range m.Parameters {
			params[i] = showParameterJSON{
				Name:        p.Name,
				Type:        p.Type,
				Description: p.Description,
			}
		}
		var ret *showReturnsJSON
		if m.Returns.Type != "" || m.Returns.Description != "" {
			ret = &showReturnsJSON{
				Type:        m.Returns.Type,
				Description: m.Returns.Description,
			}
		}
		iface.Methods = append(iface.Methods, showMethodJSON{
			Name:        m.Name,
			Signature:   m.Signature,
			Description: m.Description,
			Parameters:  params,
			Returns:     ret,
			Async:       m.Async,
			Access:      m.Access,
		})
	}

	for _, p := range spec.Interface.Properties {
		iface.Properties = append(iface.Properties, showPropertyJSON{
			Name:        p.Name,
			Type:        p.Type,
			Access:      p.Access,
			Description: p.Description,
		})
	}

	for _, e := range spec.Interface.Events {
		iface.Events = append(iface.Events, showEventJSON{
			Name:        e.Name,
			Signature:   e.Signature,
			Description: e.Description,
		})
	}

	return iface
}

func buildShowDependenciesJSON(spec *node.Spec) []showDependencyJSON {
	deps := make([]showDependencyJSON, len(spec.Dependencies))
	for i, d := range spec.Dependencies {
		deps[i] = showDependencyJSON{
			Target:    d.Target,
			Type:      d.Type,
			Injection: d.Injection,
			Optional:  d.Optional,
		}
	}
	return deps
}
