package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
)

var (
	depsDepth      int
	depsTransitive bool
)

var depsCmd = &cobra.Command{
	Use:   "deps <node-id>",
	Short: "List dependencies of a node (JSON output)",
	Long: `List direct and transitive dependencies of a node.

Always outputs JSON. This is a machine-facing command.

Examples:
  $ gdc deps PlayerController
  $ gdc deps PlayerController --depth 2
  $ gdc deps PlayerController --transitive`,
	Args: cobra.ExactArgs(1),
	RunE: runDeps,
}

func init() {
	depsCmd.Flags().IntVarP(&depsDepth, "depth", "d", 1, "dependency depth (1 = direct only)")
	depsCmd.Flags().BoolVar(&depsTransitive, "transitive", false, "show all transitive deps flattened (deduped)")
}

func runDeps(cmd *cobra.Command, args []string) error {
	nodeID := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.NodesDir()

	allNodes, err := loadAllNodes(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}

	lookup := buildSpecLookup(allNodes)
	spec, canonical, found := resolveNodeSpec(nodeID, lookup)
	if !found {
		return fmt.Errorf("node %s not found", nodeID)
	}

	if depsTransitive {
		return outputTransitiveDeps(spec, canonical, lookup)
	}
	return outputDeps(spec, canonical, lookup, allNodes, depsDepth)
}

type depTargetSpec struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Layer     string `json:"layer"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
}

type depEntry struct {
	Target     string         `json:"target"`
	Type       string         `json:"type"`
	Injection  string         `json:"injection"`
	Optional   bool           `json:"optional"`
	Usage      string         `json:"usage"`
	Resolved   bool           `json:"resolved"`
	TargetSpec *depTargetSpec `json:"target_spec,omitempty"`
}

type depsOutput struct {
	Node         string     `json:"node"`
	Depth        int        `json:"depth"`
	Dependencies []depEntry `json:"dependencies"`
}

func outputDeps(spec *node.Spec, canonical string, lookup map[string]*node.Spec, allNodes []*node.Spec, depth int) error {
	if depth < 1 {
		depth = 1
	}

	entries := gatherDepsRecursive(spec, lookup, depth, make(map[string]bool))
	out := depsOutput{
		Node:         canonical,
		Depth:        depth,
		Dependencies: entries,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func outputTransitiveDeps(spec *node.Spec, canonical string, lookup map[string]*node.Spec) error {
	entries := gatherDepsFlat(spec, lookup)
	out := depsOutput{
		Node:         canonical,
		Depth:        0,
		Dependencies: entries,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func gatherDepsRecursive(spec *node.Spec, lookup map[string]*node.Spec, depth int, seen map[string]bool) []depEntry {
	var entries []depEntry
	if depth <= 0 {
		return entries
	}

	for _, dep := range spec.Dependencies {
		if seen[dep.Target] {
			continue
		}
		seen[dep.Target] = true

		entry := depEntry{
			Target:    dep.Target,
			Type:      dep.Type,
			Injection: dep.Injection,
			Optional:  dep.Optional,
			Usage:     dep.Usage,
		}

		depSpec, _, exists := resolveNodeSpec(dep.Target, lookup)
		if exists && depSpec != nil {
			entry.Resolved = true
			entry.TargetSpec = &depTargetSpec{
				ID:        depSpec.Node.ID,
				Type:      depSpec.Node.Type,
				Layer:     depSpec.Node.Layer,
				Namespace: depSpec.Node.Namespace,
				Status:    depSpec.Metadata.Status,
			}
			if depth > 1 {
				subEntries := gatherDepsRecursive(depSpec, lookup, depth-1, seen)
				entries = append(entries, subEntries...)
			}
		}

		entries = append(entries, entry)
	}

	return entries
}

func gatherDepsFlat(spec *node.Spec, lookup map[string]*node.Spec) []depEntry {
	seen := make(map[string]bool)
	var entries []depEntry

	queue := []*node.Spec{spec}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, dep := range current.Dependencies {
			if seen[dep.Target] {
				continue
			}
			seen[dep.Target] = true

			entry := depEntry{
				Target:    dep.Target,
				Type:      dep.Type,
				Injection: dep.Injection,
				Optional:  dep.Optional,
				Usage:     dep.Usage,
			}

			depSpec, _, exists := resolveNodeSpec(dep.Target, lookup)
			if exists && depSpec != nil {
				entry.Resolved = true
				entry.TargetSpec = &depTargetSpec{
					ID:        depSpec.Node.ID,
					Type:      depSpec.Node.Type,
					Layer:     depSpec.Node.Layer,
					Namespace: depSpec.Node.Namespace,
					Status:    depSpec.Metadata.Status,
				}
				queue = append(queue, depSpec)
			}

			entries = append(entries, entry)
		}
	}

	return entries
}
