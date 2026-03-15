package cli

import (
	"fmt"
	"path/filepath"
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
}

func runShow(cmd *cobra.Command, args []string) error {
	nodeName := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.NodesDir()

	// Load node
	nodePath := filepath.Join(nodesDir, nodeName+".yaml")
	spec, err := node.Load(nodePath)
	if err != nil {
		return fmt.Errorf("node %s not found: %w", nodeName, err)
	}

	// Load all nodes for reference lookup
	allNodes, _ := loadAllNodes(nodesDir)

	if showInterface {
		printInterfaceOnly(spec, cfg.Project.Language)
		return nil
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
		refs := findReferences(nodeName, allNodes)
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
	bold.Printf("  %s\n", spec.Node.ID)
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

	bold.Println("  Responsibility:")
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
		bold.Println("  Invariants:")
		for _, inv := range spec.Responsibility.Invariants {
			fmt.Printf("    • %s\n", inv)
		}
	}

	fmt.Println(strings.Repeat("─", 70))
}

func printInterface(spec *node.Spec) {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)

	bold.Println("  Interface:")

	// Constructors
	if len(spec.Interface.Constructors) > 0 {
		for _, ctor := range spec.Interface.Constructors {
			green.Printf("    ⊕ %s\n", ctor.Signature)
		}
	}

	// Methods
	for _, method := range spec.Interface.Methods {
		green.Printf("    ▸ %s\n", method.Signature)
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

	bold.Println("  Dependencies (→):")

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
	for _, n := range allNodes {
		for _, dep := range n.Dependencies {
			if dep.Target == nodeName {
				refs = append(refs, n.Node.ID)
				break
			}
		}
	}
	return refs
}

func printReferences(refs []string) {
	bold := color.New(color.Bold)

	bold.Println("  Referenced by (←):")

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

	bold.Println("  Metadata:")
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
