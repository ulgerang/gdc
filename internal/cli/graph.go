package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
)

var (
	graphFormat          string
	graphOutput          string
	graphLayerViolations bool
	graphViolationsOnly  bool
	graphInteractive     bool
)

type graphEdge struct {
	From      string
	To        string
	Type      string
	Optional  bool
	Violation bool
}

type graphView struct {
	Nodes []*node.Spec
	Edges []graphEdge
}

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Export graph data",
	Long: `Export the dependency graph in various formats.

Supported formats:
  dot      - Graphviz DOT format
  json     - JSON graph data
  mermaid  - Mermaid diagram syntax
  html     - Mermaid HTML viewer

Examples:
  $ gdc graph --format dot --output graph.dot
  $ gdc graph --format mermaid
  $ gdc graph --layer-violations
  $ gdc graph --interactive --output graph.html`,
	RunE: runGraph,
}

func init() {
	graphCmd.Flags().StringVarP(&graphFormat, "format", "f", "mermaid", "output format (dot, json, mermaid, html)")
	graphCmd.Flags().StringVarP(&graphOutput, "output", "o", "", "output file (default: stdout)")
	graphCmd.Flags().BoolVar(&graphLayerViolations, "layer-violations", false, "highlight layer-violating dependencies")
	graphCmd.Flags().BoolVar(&graphViolationsOnly, "violations-only", false, "show only nodes and edges participating in layer violations")
	graphCmd.Flags().BoolVar(&graphInteractive, "interactive", false, "emit an HTML viewer with Mermaid rendering")
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

	if graphInteractive && graphFormat == "mermaid" {
		graphFormat = "html"
	}
	if graphViolationsOnly {
		graphLayerViolations = true
	}

	view := buildGraphView(nodes, cfg.Architecture.Layers, graphLayerViolations, graphViolationsOnly)

	var output string
	switch graphFormat {
	case "dot":
		output = generateDOT(view)
	case "json":
		output, err = generateJSON(view)
		if err != nil {
			return err
		}
	case "mermaid":
		output = generateMermaid(view)
	case "html":
		output = generateHTML(view)
	default:
		return fmt.Errorf("unknown format: %s", graphFormat)
	}

	if graphOutput != "" {
		if err := os.WriteFile(graphOutput, []byte(output), 0o644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		printSuccess("Graph exported to %s", graphOutput)
	} else {
		fmt.Println(output)
	}

	return nil
}

func buildGraphView(nodes []*node.Spec, layers []config.LayerRule, highlightViolations bool, violationsOnly bool) graphView {
	nodeMap := buildSpecLookup(nodes)
	violations := make(map[string]bool)
	if highlightViolations {
		violations = detectLayerViolationEdges(nodes, layers)
	}

	edges := make([]graphEdge, 0)
	involved := make(map[string]bool)
	for _, n := range nodes {
		for _, dep := range n.Dependencies {
			key := graphEdgeKey(n.QualifiedID(), dep.Target)
			violation := violations[key]
			if violationsOnly && !violation {
				continue
			}
			edges = append(edges, graphEdge{
				From:      n.QualifiedID(),
				To:        dep.Target,
				Type:      dep.Type,
				Optional:  dep.Optional,
				Violation: violation,
			})
			if violationsOnly {
				involved[n.QualifiedID()] = true
				if _, ok := nodeMap[dep.Target]; ok {
					involved[dep.Target] = true
				}
			}
		}
	}

	filteredNodes := nodes
	if violationsOnly {
		filteredNodes = make([]*node.Spec, 0, len(involved))
		for _, n := range nodes {
			if involved[n.QualifiedID()] {
				filteredNodes = append(filteredNodes, n)
			}
		}
	}

	return graphView{
		Nodes: filteredNodes,
		Edges: edges,
	}
}

func detectLayerViolationEdges(nodes []*node.Spec, layers []config.LayerRule) map[string]bool {
	violations := make(map[string]bool)
	allowed := make(map[string]map[string]bool)
	for _, layer := range layers {
		allowed[layer.Name] = make(map[string]bool)
		for _, dep := range layer.CanDependOn {
			allowed[layer.Name][dep] = true
		}
	}

	nodeMap := buildSpecLookup(nodes)
	for _, n := range nodes {
		srcLayer := strings.TrimSpace(n.Node.Layer)
		if srcLayer == "" {
			continue
		}
		for _, dep := range n.Dependencies {
			target, ok := nodeMap[dep.Target]
			if !ok || target == nil {
				continue
			}
			dstLayer := strings.TrimSpace(target.Node.Layer)
			if dstLayer == "" || dstLayer == srcLayer {
				continue
			}
			if allowedDeps, ok := allowed[srcLayer]; ok && !allowedDeps[dstLayer] {
				violations[graphEdgeKey(n.QualifiedID(), dep.Target)] = true
			}
		}
	}

	return violations
}

func graphEdgeKey(from, to string) string {
	return from + "->" + to
}

func generateDOT(view graphView) string {
	var sb strings.Builder

	sb.WriteString("digraph GDC {\n")
	sb.WriteString("    rankdir=TB;\n")
	sb.WriteString("    node [shape=box, style=rounded];\n")
	sb.WriteString("    edge [arrowhead=vee];\n\n")

	sb.WriteString("    // Nodes\n")
	for _, n := range view.Nodes {
		style := getNodeStyle(n.Node.Type)
		sb.WriteString(fmt.Sprintf("    \"%s\" [%s];\n", n.QualifiedID(), style))
	}

	layers := make(map[string][]*node.Spec)
	layerNames := make([]string, 0)
	for _, n := range view.Nodes {
		layer := n.Node.Layer
		if layer == "" {
			layer = "unknown"
		}
		if _, ok := layers[layer]; !ok {
			layerNames = append(layerNames, layer)
		}
		layers[layer] = append(layers[layer], n)
	}
	sort.Strings(layerNames)

	sb.WriteString("\n    // Layers\n")
	for _, layer := range layerNames {
		layerNodes := layers[layer]
		sb.WriteString(fmt.Sprintf("    subgraph cluster_%s {\n", sanitizeDOTID(layer)))
		sb.WriteString(fmt.Sprintf("        label=\"%s\";\n", layer))
		sb.WriteString("        style=dashed;\n")
		for _, n := range layerNodes {
			sb.WriteString(fmt.Sprintf("        \"%s\";\n", n.QualifiedID()))
		}
		sb.WriteString("    }\n")
	}

	sb.WriteString("\n    // Dependencies\n")
	for _, edge := range view.Edges {
		attrs := []string{}
		if edge.Optional {
			attrs = append(attrs, "style=dashed")
		} else {
			attrs = append(attrs, "style=solid")
		}
		if edge.Violation {
			attrs = append(attrs, "color=\"#C62828\"", "penwidth=2")
		}
		sb.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\" [%s];\n", edge.From, edge.To, strings.Join(attrs, ", ")))
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

func generateJSON(view graphView) (string, error) {
	type edgeJSON struct {
		From      string `json:"from"`
		To        string `json:"to"`
		Type      string `json:"type"`
		Optional  bool   `json:"optional,omitempty"`
		Violation bool   `json:"violation,omitempty"`
	}

	type nodeJSON struct {
		ID     string   `json:"id"`
		Type   string   `json:"type"`
		Layer  string   `json:"layer"`
		Status string   `json:"status"`
		Tags   []string `json:"tags,omitempty"`
	}

	type graphJSON struct {
		Nodes []nodeJSON `json:"nodes"`
		Edges []edgeJSON `json:"edges"`
	}

	graph := graphJSON{
		Nodes: make([]nodeJSON, 0, len(view.Nodes)),
		Edges: make([]edgeJSON, 0, len(view.Edges)),
	}

	for _, n := range view.Nodes {
		graph.Nodes = append(graph.Nodes, nodeJSON{
			ID:     n.QualifiedID(),
			Type:   n.Node.Type,
			Layer:  n.Node.Layer,
			Status: n.Metadata.Status,
			Tags:   n.Metadata.Tags,
		})
	}

	for _, edge := range view.Edges {
		graph.Edges = append(graph.Edges, edgeJSON{
			From:      edge.From,
			To:        edge.To,
			Type:      edge.Type,
			Optional:  edge.Optional,
			Violation: edge.Violation,
		})
	}

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func generateMermaid(view graphView) string {
	return "```mermaid\n" + generateMermaidBody(view) + "\n```\n"
}

func generateMermaidBody(view graphView) string {
	var sb strings.Builder

	sb.WriteString("graph TD\n")
	for _, n := range view.Nodes {
		shape := getMermaidShape(n.Node.Type)
		id := mermaidNodeID(n.QualifiedID())
		label := strings.ReplaceAll(n.QualifiedID(), "\"", "'")
		sb.WriteString(fmt.Sprintf("    %s%s\"%s\"%s\n", id, shape[0], label, shape[1]))
	}

	sb.WriteString("\n")
	for i, edge := range view.Edges {
		arrow := "-->"
		if edge.Optional {
			arrow = "-.->"
		}
		sb.WriteString(fmt.Sprintf("    %s %s %s\n", mermaidNodeID(edge.From), arrow, mermaidNodeID(edge.To)))
		if edge.Violation {
			sb.WriteString(fmt.Sprintf("    linkStyle %d stroke:#C62828,stroke-width:3px\n", i))
		}
	}

	sb.WriteString("\n")
	sb.WriteString("    classDef interface fill:#E3F2FD,stroke:#1565C0\n")
	sb.WriteString("    classDef class fill:#E8F5E9,stroke:#2E7D32\n")
	sb.WriteString("    classDef service fill:#FFF3E0,stroke:#EF6C00\n")
	sb.WriteString("    classDef violation stroke:#C62828,stroke-width:2px\n")

	for _, n := range view.Nodes {
		sb.WriteString(fmt.Sprintf("    class %s %s\n", mermaidNodeID(n.QualifiedID()), n.Node.Type))
	}

	return sb.String()
}

func generateHTML(view graphView) string {
	body := generateMermaidBody(view)
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>GDC Graph</title>
  <style>
    body { margin: 0; font-family: "Segoe UI", sans-serif; background: #f6f8fb; color: #172033; }
    header { padding: 16px 20px; border-bottom: 1px solid #d7deea; background: #ffffff; }
    main { padding: 20px; overflow: auto; height: calc(100vh - 74px); }
    .hint { color: #51607a; font-size: 14px; }
    .canvas { min-width: 960px; background: #ffffff; border: 1px solid #d7deea; border-radius: 12px; padding: 16px; }
  </style>
  <script type="module">
    import mermaid from "https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs";
    mermaid.initialize({ startOnLoad: true, securityLevel: "loose" });
  </script>
</head>
<body>
  <header>
    <strong>GDC Graph Viewer</strong>
    <div class="hint">Scroll to inspect the graph. Red edges indicate layer violations.</div>
  </header>
  <main>
    <div class="canvas">
      <pre class="mermaid">%s</pre>
    </div>
  </main>
</body>
</html>
`, body)
}

func getMermaidShape(nodeType string) [2]string {
	switch nodeType {
	case "interface":
		return [2]string{"([", "])"}
	case "service":
		return [2]string{"{{", "}}"}
	default:
		return [2]string{"[", "]"}
	}
}

func mermaidNodeID(id string) string {
	var sb strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			continue
		}
		sb.WriteRune('_')
	}
	if sb.Len() == 0 {
		return "node"
	}
	return sb.String()
}

func sanitizeDOTID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "unknown"
	}
	return strings.NewReplacer(".", "_", "-", "_", "/", "_", "\\", "_", " ", "_").Replace(id)
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
