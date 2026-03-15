package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	listFilter string
	listSort   string
	listFormat string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all nodes",
	Long: `List all node specifications in the project.

Examples:
  $ gdc list
  $ gdc list --filter "layer=domain"
  $ gdc list --filter "type=interface"
  $ gdc list --format json`,
	Aliases: []string{"ls"},
	RunE:    runList,
}

func init() {
	listCmd.Flags().StringVarP(&listFilter, "filter", "f", "", "filter expression (e.g., 'layer=domain')")
	listCmd.Flags().StringVarP(&listSort, "sort", "s", "name", "sort by (name, type, layer, status)")
	listCmd.Flags().StringVar(&listFormat, "format", "table", "output format (table, json, minimal)")
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.NodesDir()

	// Find all YAML files
	nodes, err := loadAllNodes(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}

	if len(nodes) == 0 {
		printInfo("No nodes found. Create one with: gdc node create <name>")
		return nil
	}

	// Apply filter
	if listFilter != "" {
		nodes = filterNodes(nodes, listFilter)
	}

	// Count dependencies
	depCounts := countDependencies(nodes)

	// Output based on format
	switch listFormat {
	case "json":
		return outputJSON(nodes)
	case "minimal":
		for _, n := range nodes {
			fmt.Println(n.Node.ID)
		}
	default:
		outputTable(nodes, depCounts)
	}

	return nil
}

func loadAllNodes(dir string) ([]*node.Spec, error) {
	var nodes []*node.Spec

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		spec, err := node.Load(path)
		if err != nil {
			printWarning("Failed to load %s: %v", path, err)
			return nil
		}
		nodes = append(nodes, spec)
		return nil
	})

	return nodes, err
}

func filterNodes(nodes []*node.Spec, filter string) []*node.Spec {
	parts := strings.SplitN(filter, "=", 2)
	if len(parts) != 2 {
		return nodes
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	var filtered []*node.Spec
	for _, n := range nodes {
		match := false
		switch key {
		case "layer":
			match = n.Node.Layer == value
		case "type":
			match = n.Node.Type == value
		case "status":
			match = n.Metadata.Status == value
		case "tag":
			for _, t := range n.Metadata.Tags {
				if t == value {
					match = true
					break
				}
			}
		}
		if match {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

func countDependencies(nodes []*node.Spec) map[string]int {
	counts := make(map[string]int)
	for _, n := range nodes {
		counts[n.Node.ID] = len(n.Dependencies)
	}
	return counts
}

func outputTable(nodes []*node.Spec, depCounts map[string]int) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Node", "Type", "Layer", "Status", "Deps"})
	table.SetBorder(false)
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
	)

	for _, n := range nodes {
		status := formatStatus(n.Metadata.Status)
		deps := fmt.Sprintf("%d", depCounts[n.Node.ID])

		table.Append([]string{
			n.Node.ID,
			n.Node.Type,
			n.Node.Layer,
			status,
			deps,
		})
	}

	table.Render()
	fmt.Printf("\nTotal: %d nodes\n", len(nodes))
}

func formatStatus(status string) string {
	switch status {
	case "implemented":
		return color.GreenString("✓ impl")
	case "specified":
		return color.CyanString("✓ spec")
	case "tested":
		return color.GreenString("✓ test")
	case "draft":
		return color.YellowString("○ draft")
	case "deprecated":
		return color.RedString("✗ depr")
	default:
		return status
	}
}

func outputJSON(nodes []*node.Spec) error {
	type nodeInfo struct {
		ID     string   `json:"id"`
		Type   string   `json:"type"`
		Layer  string   `json:"layer"`
		Status string   `json:"status"`
		Deps   []string `json:"dependencies"`
	}

	var output []nodeInfo
	for _, n := range nodes {
		deps := make([]string, len(n.Dependencies))
		for i, d := range n.Dependencies {
			deps[i] = d.Target
		}
		output = append(output, nodeInfo{
			ID:     n.Node.ID,
			Type:   n.Node.Type,
			Layer:  n.Node.Layer,
			Status: n.Metadata.Status,
			Deps:   deps,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
