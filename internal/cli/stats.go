package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var statsFormat string

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show project statistics",
	Long: `Display statistics about the project's graph structure.

Example:
  $ gdc stats
  $ gdc stats --format json`,
	RunE: runStats,
}

func init() {
	statsCmd.Flags().StringVar(&statsFormat, "format", "text", "output format (text, json)")
}

func runStats(cmd *cobra.Command, args []string) error {
	statsFormat = resolveFormat(statsFormat)
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

	if len(nodes) == 0 {
		if statsFormat == "json" {
			return outputStatsJSON(statsData{})
		}
		printInfo("No nodes found")
		return nil
	}

	data := computeStats(nodes)

	if statsFormat == "json" {
		return outputStatsJSON(data)
	}
	return outputStatsText(data)
}

type statsData struct {
	TotalNodes int
	ByType     map[string]int
	ByLayer    map[string]int
	ByStatus   map[string]int
	TotalEdges int
	IfaceEdges int
	ClassEdges int
	OrphanCount int
}

type statsJSON struct {
	TotalNodes int            `json:"total_nodes"`
	ByType     map[string]int `json:"by_type"`
	ByLayer    map[string]int `json:"by_layer"`
	ByStatus   map[string]int `json:"by_status"`
	Edges      statsEdgesJSON `json:"edges"`
	Health     statsHealthJSON `json:"health"`
}

type statsEdgesJSON struct {
	Total        int `json:"total"`
	InterfaceDeps int `json:"interface_deps"`
	ClassDeps    int `json:"class_deps"`
}

type statsHealthJSON struct {
	OrphanNodes int `json:"orphan_nodes"`
}

func computeStats(nodes []*node.Spec) statsData {
	totalNodes := len(nodes)

	typeCount := make(map[string]int)
	for _, n := range nodes {
		typeCount[n.Node.Type]++
	}

	layerCount := make(map[string]int)
	for _, n := range nodes {
		layer := n.Node.Layer
		if layer == "" {
			layer = "unspecified"
		}
		layerCount[layer]++
	}

	statusCount := make(map[string]int)
	for _, n := range nodes {
		status := n.Metadata.Status
		if status == "" {
			status = "draft"
		}
		statusCount[status]++
	}

	totalEdges := 0
	ifaceEdges := 0
	classEdges := 0
	for _, n := range nodes {
		for _, dep := range n.Dependencies {
			totalEdges++
			if dep.Type == "interface" {
				ifaceEdges++
			} else {
				classEdges++
			}
		}
	}

	referenced := make(map[string]bool)
	for _, n := range nodes {
		for _, dep := range n.Dependencies {
			referenced[dep.Target] = true
		}
	}
	orphanCount := 0
	for _, n := range nodes {
		if n.Node.Type != "interface" && !referenced[n.Node.ID] {
			orphanCount++
		}
	}

	return statsData{
		TotalNodes:  totalNodes,
		ByType:      typeCount,
		ByLayer:     layerCount,
		ByStatus:    statusCount,
		TotalEdges:  totalEdges,
		IfaceEdges:  ifaceEdges,
		ClassEdges:  classEdges,
		OrphanCount: orphanCount,
	}
}

func outputStatsJSON(data statsData) error {
	output := statsJSON{
		TotalNodes: data.TotalNodes,
		ByType:     data.ByType,
		ByLayer:    data.ByLayer,
		ByStatus:   data.ByStatus,
		Edges: statsEdgesJSON{
			Total:         data.TotalEdges,
			InterfaceDeps: data.IfaceEdges,
			ClassDeps:     data.ClassEdges,
		},
		Health: statsHealthJSON{
			OrphanNodes: data.OrphanCount,
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputStatsText(data statsData) error {
	fmt.Println()
	fmt.Println("📊 Project Statistics")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	fmt.Printf("Nodes: %d total\n", data.TotalNodes)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetColumnSeparator("")
	for t, count := range data.ByType {
		pct := float64(count) / float64(data.TotalNodes) * 100
		table.Append([]string{"  ├─", fmt.Sprintf("%s:", t), fmt.Sprintf("%d", count), fmt.Sprintf("(%.1f%%)", pct)})
	}
	table.Render()
	fmt.Println()

	fmt.Println("By Layer:")
	for layer, count := range data.ByLayer {
		fmt.Printf("  ├─ %-15s %d\n", layer+":", count)
	}
	fmt.Println()

	fmt.Println("By Status:")
	for status, count := range data.ByStatus {
		pct := float64(count) / float64(data.TotalNodes) * 100
		fmt.Printf("  ├─ %-15s %d (%.1f%%)\n", status+":", count, pct)
	}
	fmt.Println()

	fmt.Printf("Edges: %d total\n", data.TotalEdges)
	fmt.Printf("  ├─ Interface deps: %d\n", data.IfaceEdges)
	fmt.Printf("  └─ Class deps:     %d\n", data.ClassEdges)
	fmt.Println()

	fmt.Println("Health:")
	if data.OrphanCount > 0 {
		fmt.Printf("  └─ Orphan nodes:   %d\n", data.OrphanCount)
	} else {
		fmt.Println("  └─ No orphan nodes")
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	return nil
}
