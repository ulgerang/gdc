package cli

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/search"
	"github.com/spf13/cobra"
)

var (
	traceDepth     int
	traceDirection string
	traceTo        string
	traceReverse   bool
	traceFormat    string
)

var traceCmd = &cobra.Command{
	Use:   "trace <node>",
	Short: "Trace dependency paths",
	Long: `Trace dependency paths from a specific node.

Examples:
  $ gdc trace PlayerController              # Show all dependencies
  $ gdc trace PlayerController --depth 2    # Limit depth
  $ gdc trace PlayerController --reverse    # Show what depends on this (alias for --direction up)
  $ gdc trace PlayerController --direction up  # Same as --reverse
  $ gdc trace PlayerController --to DatabaseService  # Path to specific node`,
	Args: cobra.ExactArgs(1),
	RunE: runTrace,
}

func init() {
	traceCmd.Flags().IntVarP(&traceDepth, "depth", "d", 0, "maximum traversal depth (0 = unlimited)")
	traceCmd.Flags().StringVar(&traceDirection, "direction", "down", "direction (down, up, both)")
	traceCmd.Flags().StringVar(&traceTo, "to", "", "find path to specific node")
	traceCmd.Flags().BoolVarP(&traceReverse, "reverse", "r", false, "show reverse dependencies (alias for --direction up)")
	traceCmd.Flags().StringVar(&traceFormat, "format", "text", "output format (text, json)")
}

func runTrace(cmd *cobra.Command, args []string) error {
	traceFormat = resolveFormat(traceFormat)
	startNode := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Graceful degradation: check project readiness and provide helpful guidance
	if checkErr := search.CheckAndSuggest(cfg.ProjectRoot); checkErr != nil {
		if search.IsGracefulError(checkErr) {
			printWarning("%v", checkErr)
		}
	}

	nodesDir := cfg.Storage.NodesDir
	if nodesDir == "" {
		nodesDir = ".gdc/nodes"
	}

	// Load all nodes
	allNodes, err := loadAllNodes(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}

	lookup := buildSpecLookup(allNodes)
	nodeMap := buildCanonicalSpecMap(allNodes)
	startNode = resolveNodeAlias(startNode, lookup)
	if traceTo != "" {
		traceTo = resolveNodeAlias(traceTo, lookup)
	}

	// Check if start node exists
	if _, ok := nodeMap[startNode]; !ok {
		return fmt.Errorf("node %s not found", startNode)
	}

	// Build dependency graph
	// Handle --reverse flag as alias for --direction up
	if traceReverse {
		traceDirection = "up"
	}

	if traceTo != "" {
		// Find specific path
		path := findPath(startNode, traceTo, nodeMap, lookup)
		if traceFormat == "json" {
			return outputTracePathJSON(startNode, traceTo, path)
		}
		if path == nil {
			printWarning("No path found from %s to %s", startNode, traceTo)
		} else {
			printPath(path)
		}
	} else if traceFormat == "json" {
		return outputTraceTreeJSON(startNode, nodeMap, allNodes, lookup)
	} else {
		// Show dependency tree
		switch traceDirection {
		case "up":
			printReverseTree(startNode, nodeMap, allNodes, 0, traceDepth, make(map[string]bool))
		case "both":
			fmt.Println(color.CyanString("Dependencies (→):"))
			printDependencyTree(startNode, nodeMap, lookup, 0, traceDepth, make(map[string]bool))
			fmt.Println()
			fmt.Println(color.CyanString("Referenced by (←):"))
			printReverseTree(startNode, nodeMap, allNodes, 0, traceDepth, make(map[string]bool))
		default:
			printDependencyTree(startNode, nodeMap, lookup, 0, traceDepth, make(map[string]bool))
		}
	}

	return nil
}

func printDependencyTree(nodeName string, nodeMap map[string]*node.Spec, lookup map[string]*node.Spec, depth int, maxDepth int, visited map[string]bool) {
	if maxDepth > 0 && depth > maxDepth {
		return
	}

	if _, canonical, ok := resolveNodeSpec(nodeName, lookup); ok {
		nodeName = canonical
	}

	if visited[nodeName] {
		indent := strings.Repeat("│   ", depth)
		fmt.Printf("%s└── %s %s\n", indent, nodeName, color.YellowString("(circular)"))
		return
	}

	visited[nodeName] = true
	defer func() { visited[nodeName] = false }()

	spec, ok := nodeMap[nodeName]
	if !ok {
		indent := strings.Repeat("│   ", depth)
		fmt.Printf("%s%s %s\n", indent, nodeName, color.RedString("(not found)"))
		return
	}

	if depth == 0 {
		fmt.Println(color.CyanString(nodeName))
	}

	for i, dep := range spec.Dependencies {
		indent := strings.Repeat("│   ", depth)
		isLast := i == len(spec.Dependencies)-1

		connector := "├──"
		if isLast {
			connector = "└──"
		}

		optional := ""
		if dep.Optional {
			optional = color.YellowString(" [opt]")
		}

		depSpec, canonicalTarget, exists := resolveNodeSpec(dep.Target, lookup)
		displayTarget := dep.Target
		nodeType := ""
		if exists {
			displayTarget = canonicalTarget
			nodeType = fmt.Sprintf(" (%s)", depSpec.Node.Type)
		} else {
			nodeType = color.RedString(" (missing)")
		}

		fmt.Printf("%s%s %s%s%s\n", indent, connector, displayTarget, nodeType, optional)

		if exists && len(depSpec.Dependencies) > 0 {
			printDependencyTree(canonicalTarget, nodeMap, lookup, depth+1, maxDepth, visited)
		}
	}
}

func printReverseTree(nodeName string, nodeMap map[string]*node.Spec, allNodes []*node.Spec, depth int, maxDepth int, visited map[string]bool) {
	if maxDepth > 0 && depth > maxDepth {
		return
	}

	if visited[nodeName] {
		indent := strings.Repeat("│   ", depth)
		fmt.Printf("%s└── %s %s\n", indent, nodeName, color.YellowString("(circular)"))
		return
	}

	visited[nodeName] = true
	defer func() { visited[nodeName] = false }()

	if depth == 0 {
		fmt.Println(color.CyanString(nodeName))
	}

	// Find all nodes that depend on this one
	refs := findReferences(nodeName, allNodes)

	for i, ref := range refs {
		indent := strings.Repeat("│   ", depth)
		isLast := i == len(refs)-1

		connector := "├──"
		if isLast {
			connector = "└──"
		}

		refSpec := nodeMap[ref]
		nodeType := fmt.Sprintf(" (%s)", refSpec.Node.Type)

		fmt.Printf("%s%s %s%s\n", indent, connector, ref, nodeType)

		printReverseTree(ref, nodeMap, allNodes, depth+1, maxDepth, visited)
	}
}

func findPath(from, to string, nodeMap map[string]*node.Spec, lookup map[string]*node.Spec) []string {
	from = resolveNodeAlias(from, lookup)
	to = resolveNodeAlias(to, lookup)
	if from == to {
		return []string{from}
	}

	visited := make(map[string]bool)
	queue := [][]string{{from}}

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		current := path[len(path)-1]

		if visited[current] {
			continue
		}
		visited[current] = true

		spec, ok := nodeMap[current]
		if !ok {
			continue
		}

		for _, dep := range spec.Dependencies {
			_, canonicalTarget, exists := resolveNodeSpec(dep.Target, lookup)
			if !exists {
				continue
			}
			if canonicalTarget == to {
				return append(path, to)
			}
			if !visited[canonicalTarget] {
				newPath := make([]string, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = canonicalTarget
				queue = append(queue, newPath)
			}
		}
	}

	return nil
}

func printPath(path []string) {
	fmt.Println(color.CyanString("Path found:"))
	for i, node := range path {
		if i == 0 {
			fmt.Printf("  %s\n", color.GreenString(node))
		} else {
			fmt.Printf("  └─→ %s\n", color.GreenString(node))
		}
	}
}

type traceNodeJSON struct {
	ID       string           `json:"id"`
	Type     string           `json:"type,omitempty"`
	Layer    string           `json:"layer,omitempty"`
	DepType  string           `json:"dep_type,omitempty"`
	Optional bool             `json:"optional,omitempty"`
	Children []*traceNodeJSON `json:"children"`
}

type traceTreeResult struct {
	Root       string         `json:"root"`
	Direction  string         `json:"direction"`
	DepthLimit int            `json:"depth_limit"`
	Tree       *traceNodeJSON `json:"tree,omitempty"`
	Dependencies *traceNodeJSON `json:"dependencies,omitempty"`
	ReferencedBy *traceNodeJSON `json:"referenced_by,omitempty"`
}

type tracePathResult struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Found bool     `json:"found"`
	Path  []string `json:"path,omitempty"`
}

func outputTracePathJSON(from, to string, path []string) error {
	result := tracePathResult{
		From:  from,
		To:    to,
		Found: path != nil,
		Path:  path,
	}
	return outputJSONValue(result)
}

func outputTraceTreeJSON(startNode string, nodeMap map[string]*node.Spec, allNodes []*node.Spec, lookup map[string]*node.Spec) error {
	result := traceTreeResult{
		Root:       startNode,
		Direction:  traceDirection,
		DepthLimit: traceDepth,
	}

	switch traceDirection {
	case "up":
		result.Tree = buildReverseTreeJSON(startNode, nodeMap, allNodes, 0, traceDepth, make(map[string]bool))
	case "both":
		result.Dependencies = buildDependencyTreeJSON(startNode, nodeMap, lookup, 0, traceDepth, make(map[string]bool))
		result.ReferencedBy = buildReverseTreeJSON(startNode, nodeMap, allNodes, 0, traceDepth, make(map[string]bool))
	default:
		result.Tree = buildDependencyTreeJSON(startNode, nodeMap, lookup, 0, traceDepth, make(map[string]bool))
	}

	return outputJSONValue(result)
}

func buildDependencyTreeJSON(nodeName string, nodeMap map[string]*node.Spec, lookup map[string]*node.Spec, depth int, maxDepth int, visited map[string]bool) *traceNodeJSON {
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	if _, canonical, ok := resolveNodeSpec(nodeName, lookup); ok {
		nodeName = canonical
	}

	if visited[nodeName] {
		return &traceNodeJSON{ID: nodeName, Children: []*traceNodeJSON{}}
	}

	visited[nodeName] = true
	defer func() { visited[nodeName] = false }()

	spec, ok := nodeMap[nodeName]
	if !ok {
		return &traceNodeJSON{ID: nodeName, Children: []*traceNodeJSON{}}
	}

	result := &traceNodeJSON{
		ID:       nodeName,
		Type:     spec.Node.Type,
		Layer:    spec.Node.Layer,
		Children: make([]*traceNodeJSON, 0),
	}

	for _, dep := range spec.Dependencies {
		depSpec, canonicalTarget, exists := resolveNodeSpec(dep.Target, lookup)
		child := &traceNodeJSON{
			ID:       dep.Target,
			DepType:  dep.Type,
			Optional: dep.Optional,
			Children: make([]*traceNodeJSON, 0),
		}
		if exists {
			child.ID = canonicalTarget
			child.Type = depSpec.Node.Type
			child.Layer = depSpec.Node.Layer
			if exists && len(depSpec.Dependencies) > 0 {
				if sub := buildDependencyTreeJSON(canonicalTarget, nodeMap, lookup, depth+1, maxDepth, visited); sub != nil {
					child.Children = sub.Children
				}
			}
		}
		result.Children = append(result.Children, child)
	}

	return result
}

func buildReverseTreeJSON(nodeName string, nodeMap map[string]*node.Spec, allNodes []*node.Spec, depth int, maxDepth int, visited map[string]bool) *traceNodeJSON {
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	if visited[nodeName] {
		return &traceNodeJSON{ID: nodeName, Children: []*traceNodeJSON{}}
	}

	visited[nodeName] = true
	defer func() { visited[nodeName] = false }()

	spec, ok := nodeMap[nodeName]
	if !ok {
		return &traceNodeJSON{ID: nodeName, Children: []*traceNodeJSON{}}
	}

	result := &traceNodeJSON{
		ID:       nodeName,
		Type:     spec.Node.Type,
		Layer:    spec.Node.Layer,
		Children: make([]*traceNodeJSON, 0),
	}

	refs := findReferences(nodeName, allNodes)
	for _, ref := range refs {
		refSpec := nodeMap[ref]
		child := &traceNodeJSON{
			ID:       ref,
			Children: make([]*traceNodeJSON, 0),
		}
		if refSpec != nil {
			child.Type = refSpec.Node.Type
			child.Layer = refSpec.Node.Layer
		}
		if sub := buildReverseTreeJSON(ref, nodeMap, allNodes, depth+1, maxDepth, visited); sub != nil {
			child.Children = sub.Children
		}
		result.Children = append(result.Children, child)
	}

	return result
}
