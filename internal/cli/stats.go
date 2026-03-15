package cli

import (
	"fmt"
	"os"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show project statistics",
	Long: `Display statistics about the project's graph structure.

Example:
  $ gdc stats
  $ gdc stats --format json`,
	RunE: runStats,
}

func runStats(cmd *cobra.Command, args []string) error {
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
		printInfo("No nodes found")
		return nil
	}

	// Calculate statistics
	totalNodes := len(nodes)

	// By type
	typeCount := make(map[string]int)
	for _, n := range nodes {
		typeCount[n.Node.Type]++
	}

	// By layer
	layerCount := make(map[string]int)
	for _, n := range nodes {
		layer := n.Node.Layer
		if layer == "" {
			layer = "unspecified"
		}
		layerCount[layer]++
	}

	// By status
	statusCount := make(map[string]int)
	for _, n := range nodes {
		status := n.Metadata.Status
		if status == "" {
			status = "draft"
		}
		statusCount[status]++
	}

	// Count edges
	totalEdges := 0
	interfaceEdges := 0
	classEdges := 0
	for _, n := range nodes {
		for _, dep := range n.Dependencies {
			totalEdges++
			if dep.Type == "interface" {
				interfaceEdges++
			} else {
				classEdges++
			}
		}
	}

	// Find orphans
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

	// Output
	fmt.Println()
	fmt.Println("📊 Project Statistics")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Nodes by type
	fmt.Printf("Nodes: %d total\n", totalNodes)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetColumnSeparator("")
	for t, count := range typeCount {
		pct := float64(count) / float64(totalNodes) * 100
		table.Append([]string{"  ├─", fmt.Sprintf("%s:", t), fmt.Sprintf("%d", count), fmt.Sprintf("(%.1f%%)", pct)})
	}
	table.Render()
	fmt.Println()

	// By layer
	fmt.Println("By Layer:")
	for layer, count := range layerCount {
		fmt.Printf("  ├─ %-15s %d\n", layer+":", count)
	}
	fmt.Println()

	// By status
	fmt.Println("By Status:")
	for status, count := range statusCount {
		pct := float64(count) / float64(totalNodes) * 100
		fmt.Printf("  ├─ %-15s %d (%.1f%%)\n", status+":", count, pct)
	}
	fmt.Println()

	// Edges
	fmt.Printf("Edges: %d total\n", totalEdges)
	fmt.Printf("  ├─ Interface deps: %d\n", interfaceEdges)
	fmt.Printf("  └─ Class deps:     %d\n", classEdges)
	fmt.Println()

	// Health
	fmt.Println("Health:")
	if orphanCount > 0 {
		fmt.Printf("  └─ Orphan nodes:   %d\n", orphanCount)
	} else {
		fmt.Println("  └─ No orphan nodes")
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return nil
}
