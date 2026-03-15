package cli

import (
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
)

func TestBuildGraphViewViolationsOnlyFiltersNodesAndEdges(t *testing.T) {
	nodes := []*node.Spec{
		{
			Node: node.NodeInfo{ID: "BattleScreen", Layer: "presentation"},
			Dependencies: []node.Dependency{
				{Target: "RewardStore"},
			},
		},
		{
			Node: node.NodeInfo{ID: "RewardStore", Layer: "infrastructure"},
		},
		{
			Node: node.NodeInfo{ID: "BattleService", Layer: "application"},
		},
	}

	view := buildGraphView(nodes, []config.LayerRule{
		{Name: "presentation", CanDependOn: []string{"application"}},
		{Name: "application", CanDependOn: []string{"domain", "infrastructure"}},
		{Name: "infrastructure", CanDependOn: []string{"domain"}},
	}, true, true)

	if len(view.Edges) != 1 {
		t.Fatalf("expected 1 violating edge, got %d", len(view.Edges))
	}
	if !view.Edges[0].Violation {
		t.Fatal("expected edge to be marked as a violation")
	}
	if len(view.Nodes) != 2 {
		t.Fatalf("expected only nodes participating in the violation, got %d", len(view.Nodes))
	}
}

func TestGenerateDOTHighlightsLayerViolations(t *testing.T) {
	view := graphView{
		Nodes: []*node.Spec{
			{Node: node.NodeInfo{ID: "BattleScreen", Layer: "presentation"}},
			{Node: node.NodeInfo{ID: "RewardStore", Layer: "infrastructure"}},
		},
		Edges: []graphEdge{
			{From: "BattleScreen", To: "RewardStore", Violation: true},
		},
	}

	output := generateDOT(view)
	if !strings.Contains(output, "color=\"#C62828\"") {
		t.Fatalf("expected violation edge to be highlighted in DOT output, got %q", output)
	}
}

func TestGenerateHTMLWrapsMermaidViewer(t *testing.T) {
	output := generateHTML(graphView{})
	if !strings.Contains(output, "cdn.jsdelivr.net") {
		t.Fatalf("expected HTML graph viewer to include Mermaid loader, got %q", output)
	}
}
