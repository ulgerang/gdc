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
}

func runTrace(cmd *cobra.Command, args []string) error {
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

	// Build node map
	nodeMap := make(map[string]*node.Spec)
	for _, n := range allNodes {
		nodeMap[n.Node.ID] = n
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
		path := findPath(startNode, traceTo, nodeMap)
		if path == nil {
			printWarning("No path found from %s to %s", startNode, traceTo)
		} else {
			printPath(path)
		}
	} else {
		// Show dependency tree
		switch traceDirection {
		case "up":
			printReverseTree(startNode, nodeMap, 0, traceDepth, make(map[string]bool))
		case "both":
			fmt.Println(color.CyanString("Dependencies (→):"))
			printDependencyTree(startNode, nodeMap, 0, traceDepth, make(map[string]bool))
			fmt.Println()
			fmt.Println(color.CyanString("Referenced by (←):"))
			printReverseTree(startNode, nodeMap, 0, traceDepth, make(map[string]bool))
		default:
			printDependencyTree(startNode, nodeMap, 0, traceDepth, make(map[string]bool))
		}
	}

	return nil
}

func printDependencyTree(nodeName string, nodeMap map[string]*node.Spec, depth int, maxDepth int, visited map[string]bool) {
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

		depSpec, exists := nodeMap[dep.Target]
		nodeType := ""
		if exists {
			nodeType = fmt.Sprintf(" (%s)", depSpec.Node.Type)
		} else {
			nodeType = color.RedString(" (missing)")
		}

		fmt.Printf("%s%s %s%s%s\n", indent, connector, dep.Target, nodeType, optional)

		if exists && len(depSpec.Dependencies) > 0 {
			printDependencyTree(dep.Target, nodeMap, depth+1, maxDepth, visited)
		}
	}
}

func printReverseTree(nodeName string, nodeMap map[string]*node.Spec, depth int, maxDepth int, visited map[string]bool) {
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
	var refs []string
	for _, n := range nodeMap {
		for _, dep := range n.Dependencies {
			if dep.Target == nodeName {
				refs = append(refs, n.Node.ID)
				break
			}
		}
	}

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

		printReverseTree(ref, nodeMap, depth+1, maxDepth, visited)
	}
}

func findPath(from, to string, nodeMap map[string]*node.Spec) []string {
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
			if dep.Target == to {
				return append(path, to)
			}
			if !visited[dep.Target] {
				newPath := make([]string, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = dep.Target
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
