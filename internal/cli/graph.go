package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
)

var (
	graphFormat string
	graphOutput string
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Export graph data",
	Long: `Export the dependency graph in various formats.

Supported formats:
  • dot      - Graphviz DOT format
  • json     - JSON graph data
  • mermaid  - Mermaid diagram syntax

Examples:
  $ gdc graph --format dot --output graph.dot
  $ gdc graph --format mermaid
  $ gdc graph --format json > graph.json`,
	RunE: runGraph,
}

func init() {
	graphCmd.Flags().StringVarP(&graphFormat, "format", "f", "mermaid", "output format (dot, json, mermaid)")
	graphCmd.Flags().StringVarP(&graphOutput, "output", "o", "", "output file (default: stdout)")
}

func runGraph(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.Storage.NodesDir
	if nodesDir == "" {
		nodesDir = ".gdc/nodes"
	}

	nodes, err := loadAllNodes(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}

	var output string
	switch graphFormat {
	case "dot":
		output = generateDOT(nodes)
	case "json":
		output, err = generateJSON(nodes)
		if err != nil {
			return err
		}
	case "mermaid":
		output = generateMermaid(nodes)
	default:
		return fmt.Errorf("unknown format: %s", graphFormat)
	}

	if graphOutput != "" {
		if err := os.WriteFile(graphOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		printSuccess("Graph exported to %s", graphOutput)
	} else {
		fmt.Println(output)
	}

	return nil
}

func generateDOT(nodes []*node.Spec) string {
	var sb strings.Builder

	sb.WriteString("digraph GDC {\n")
	sb.WriteString("    rankdir=TB;\n")
	sb.WriteString("    node [shape=box, style=rounded];\n")
	sb.WriteString("    edge [arrowhead=vee];\n\n")

	// Define nodes with styles based on type
	sb.WriteString("    // Nodes\n")
	for _, n := range nodes {
		style := getNodeStyle(n.Node.Type)
		sb.WriteString(fmt.Sprintf("    \"%s\" [%s];\n", n.Node.ID, style))
	}

	// Define subgraphs for layers
	layers := make(map[string][]*node.Spec)
	for _, n := range nodes {
		layer := n.Node.Layer
		if layer == "" {
			layer = "unknown"
		}
		layers[layer] = append(layers[layer], n)
	}

	sb.WriteString("\n    // Layers\n")
	for layer, layerNodes := range layers {
		sb.WriteString(fmt.Sprintf("    subgraph cluster_%s {\n", layer))
		sb.WriteString(fmt.Sprintf("        label=\"%s\";\n", layer))
		sb.WriteString("        style=dashed;\n")
		for _, n := range layerNodes {
			sb.WriteString(fmt.Sprintf("        \"%s\";\n", n.Node.ID))
		}
		sb.WriteString("    }\n")
	}

	// Define edges
	sb.WriteString("\n    // Dependencies\n")
	for _, n := range nodes {
		for _, dep := range n.Dependencies {
			style := "solid"
			if dep.Optional {
				style = "dashed"
			}
			sb.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\" [style=%s];\n",
				n.Node.ID, dep.Target, style))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

func getNodeStyle(nodeType string) string {
	switch nodeType {
	case "interface":
		return "fillcolor=\"#E3F2FD\", style=\"filled,rounded\""
	case "class":
		return "fillcolor=\"#E8F5E9\", style=\"filled,rounded\""
	case "service":
		return "fillcolor=\"#FFF3E0\", style=\"filled,rounded\""
	case "module":
		return "fillcolor=\"#F3E5F5\", style=\"filled,rounded\""
	default:
		return "style=rounded"
	}
}

func generateJSON(nodes []*node.Spec) (string, error) {
	type Edge struct {
		From     string `json:"from"`
		To       string `json:"to"`
		Type     string `json:"type"`
		Optional bool   `json:"optional,omitempty"`
	}

	type Node struct {
		ID     string   `json:"id"`
		Type   string   `json:"type"`
		Layer  string   `json:"layer"`
		Status string   `json:"status"`
		Tags   []string `json:"tags,omitempty"`
	}

	type Graph struct {
		Nodes []Node `json:"nodes"`
		Edges []Edge `json:"edges"`
	}

	graph := Graph{
		Nodes: make([]Node, 0, len(nodes)),
		Edges: make([]Edge, 0),
	}

	for _, n := range nodes {
		graph.Nodes = append(graph.Nodes, Node{
			ID:     n.Node.ID,
			Type:   n.Node.Type,
			Layer:  n.Node.Layer,
			Status: n.Metadata.Status,
			Tags:   n.Metadata.Tags,
		})

		for _, dep := range n.Dependencies {
			graph.Edges = append(graph.Edges, Edge{
				From:     n.Node.ID,
				To:       dep.Target,
				Type:     dep.Type,
				Optional: dep.Optional,
			})
		}
	}

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func generateMermaid(nodes []*node.Spec) string {
	var sb strings.Builder

	sb.WriteString("```mermaid\n")
	sb.WriteString("graph TD\n")

	// Define nodes
	for _, n := range nodes {
		shape := getMermaidShape(n.Node.Type)
		sb.WriteString(fmt.Sprintf("    %s%s%s\n", n.Node.ID, shape[0], shape[1]))
	}

	sb.WriteString("\n")

	// Define edges
	for _, n := range nodes {
		for _, dep := range n.Dependencies {
			arrow := "-->"
			if dep.Optional {
				arrow = "-.->"
			}
			sb.WriteString(fmt.Sprintf("    %s %s %s\n", n.Node.ID, arrow, dep.Target))
		}
	}

	// Style definitions
	sb.WriteString("\n")
	sb.WriteString("    classDef interface fill:#E3F2FD,stroke:#1565C0\n")
	sb.WriteString("    classDef class fill:#E8F5E9,stroke:#2E7D32\n")
	sb.WriteString("    classDef service fill:#FFF3E0,stroke:#EF6C00\n")

	// Apply styles
	for _, n := range nodes {
		sb.WriteString(fmt.Sprintf("    class %s %s\n", n.Node.ID, n.Node.Type))
	}

	sb.WriteString("```\n")
	return sb.String()
}

func getMermaidShape(nodeType string) [2]string {
	switch nodeType {
	case "interface":
		return [2]string{"([", "])"} // Stadium shape
	case "service":
		return [2]string{"{{", "}}"} // Hexagon
	default:
		return [2]string{"[", "]"} // Rectangle
	}
}

// Export for sync command
func GetNodesDir() string {
	cfg, err := config.Load("")
	if err != nil {
		return ".gdc/nodes"
	}
	nodesDir := cfg.Storage.NodesDir
	if nodesDir == "" {
		return ".gdc/nodes"
	}
	return nodesDir
}

func GetDBPath() string {
	cfg, err := config.Load("")
	if err != nil {
		return ".gdc/graph.db"
	}
	dbPath := cfg.Database.Path
	if dbPath == "" {
		return ".gdc/graph.db"
	}
	return dbPath
}

func GetAbsPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, path)
}
