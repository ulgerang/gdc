package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
)

var refsDepth int

var refsCmd = &cobra.Command{
	Use:   "refs <node-id>",
	Short: "List nodes that reference a node (JSON output)",
	Long: `List all nodes that depend on (reference) the given node.

Always outputs JSON. This is a machine-facing command.

Examples:
  $ gdc refs IInputManager
  $ gdc refs IInputManager --depth 2`,
	Args: cobra.ExactArgs(1),
	RunE: runRefs,
}

func init() {
	refsCmd.Flags().IntVarP(&refsDepth, "depth", "d", 1, "reference depth (1 = direct only)")
}

func runRefs(cmd *cobra.Command, args []string) error {
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
	_, canonical, found := resolveNodeSpec(nodeID, lookup)
	if !found {
		return fmt.Errorf("node %s not found", nodeID)
	}

	entries := gatherRefsRecursive(canonical, allNodes, lookup, refsDepth, make(map[string]bool))

	out := refsOutput{
		Node:       canonical,
		Depth:      refsDepth,
		References: entries,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

type refEntry struct {
	Node       string `json:"node"`
	Type       string `json:"type"`
	Layer      string `json:"layer"`
	Namespace  string `json:"namespace"`
	DepType    string `json:"dep_type"`
	Injection  string `json:"injection"`
	Optional   bool   `json:"optional"`
}

type refsOutput struct {
	Node       string     `json:"node"`
	Depth      int        `json:"depth"`
	References []refEntry `json:"references"`
}

func gatherRefsRecursive(nodeName string, allNodes []*node.Spec, lookup map[string]*node.Spec, depth int, seen map[string]bool) []refEntry {
	if depth <= 0 {
		return nil
	}

	var entries []refEntry
	resolved := resolveNodeAlias(nodeName, lookup)

	for _, n := range allNodes {
		for _, dep := range n.Dependencies {
			depResolved := resolveNodeAlias(dep.Target, lookup)
			if depResolved != resolved {
				continue
			}

			refCanonical := n.QualifiedID()
			if seen[refCanonical] {
				continue
			}
			seen[refCanonical] = true

			entry := refEntry{
				Node:      refCanonical,
				Type:      n.Node.Type,
				Layer:     n.Node.Layer,
				Namespace: n.Node.Namespace,
				DepType:   dep.Type,
				Injection: dep.Injection,
				Optional:  dep.Optional,
			}
			entries = append(entries, entry)

			if depth > 1 {
				subEntries := gatherRefsRecursive(refCanonical, allNodes, lookup, depth-1, seen)
				entries = append(entries, subEntries...)
			}

			break
		}
	}

	return entries
}
