package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/node"
)

func TestBuildImpactReportUsesDeclaredReverseEdges(t *testing.T) {
	auth := &node.Spec{
		Node:      node.NodeInfo{ID: "Service", Namespace: "auth", Type: "interface", FilePath: "internal/auth/service.go"},
		Interface: node.Interface{Methods: []node.Method{{Name: "LoginGuest", Signature: "LoginGuest(playerID string)"}}},
	}
	root := &node.Spec{
		Node:         node.NodeInfo{ID: "VerySimpleServer", Type: "service", FilePath: "cmd/api/main.go"},
		Dependencies: []node.Dependency{{Target: "auth.Service", ContractHash: "cc0e8889", Requires: []string{"LoginGuest"}}},
		ImplementationContract: &node.ImplementationContract{Acceptance: []node.AcceptanceScenario{{
			ID: "AUTH-CONTINUITY-001", Given: "A player.", When: "LoginGuest runs.", Then: []string{"Continuity holds."},
			Covers: []node.AcceptanceCoverage{{Symbol: "auth.Service.LoginGuest", Aspects: []string{"continuity"}}},
		}}},
	}

	report, err := buildImpactReport("auth.Service.LoginGuest", []*node.Spec{auth, root})
	if err != nil {
		t.Fatalf("build impact: %v", err)
	}
	if report.Symbol != "auth.Service.LoginGuest" || report.Completeness != "declared_graph_only" {
		t.Fatalf("unexpected impact identity: %+v", report)
	}
	kinds := map[string]bool{}
	for _, finding := range report.Findings {
		kinds[finding.Kind] = true
		if finding.Provenance != "declared" || finding.Confidence != "high" {
			t.Fatalf("finding omitted provenance: %+v", finding)
		}
	}
	for _, kind := range []string{"contract_holder", "composition_site", "acceptance"} {
		if !kinds[kind] {
			t.Fatalf("missing %s impact edge: %+v", kind, report.Findings)
		}
	}
}

func TestBuildImpactReportDoesNotInferAcceptanceFromProse(t *testing.T) {
	auth := &node.Spec{Node: node.NodeInfo{ID: "Auth", Type: "interface"}, Interface: node.Interface{Methods: []node.Method{{Name: "LoginGuest", Signature: "LoginGuest()"}}}}
	auth.ImplementationContract = &node.ImplementationContract{Acceptance: []node.AcceptanceScenario{{
		ID: "PROSE-ONLY", Given: "A player.", When: "LoginGuest runs.", Then: []string{"It succeeds."},
	}}}
	report, err := buildImpactReport("Auth.LoginGuest", []*node.Spec{auth})
	if err != nil {
		t.Fatalf("build impact: %v", err)
	}
	for _, finding := range report.Findings {
		if finding.Kind == "acceptance" {
			t.Fatalf("acceptance prose was treated as a declared edge: %+v", finding)
		}
	}
}

func TestBuildHashFixPlansSeparatesDependencyAndExternalReview(t *testing.T) {
	rootDir := t.TempDir()
	content := []byte("reviewed contract\n")
	if err := os.WriteFile(filepath.Join(rootDir, "contract.md"), content, 0o644); err != nil {
		t.Fatalf("write contract: %v", err)
	}
	target := &node.Spec{SchemaVersion: "1.1", Node: node.NodeInfo{ID: "Target", Type: "interface"}, Responsibility: node.Responsibility{Summary: "Target."}, Interface: node.Interface{Methods: []node.Method{{Name: "Run", Signature: "Run()"}}}}
	source := &node.Spec{
		Node:         node.NodeInfo{ID: "Source", Type: "service"},
		Dependencies: []node.Dependency{{Target: "Target", ContractHash: "stale"}},
		ImplementationContract: &node.ImplementationContract{ExternalContracts: []node.ExternalContract{{
			ID: "authority", Path: "contract.md", ContractHash: strings.Repeat("0", 64), Description: "Authority.",
		}}},
	}
	nodes := []*node.Spec{source, target}
	plans := buildHashFixPlans(nodes, buildSpecLookup(nodes), rootDir)
	if len(plans) != 2 {
		t.Fatalf("expected dependency and external plans, got %+v", plans)
	}
	for _, plan := range plans {
		switch plan.Kind {
		case "dependency":
			if plan.Disposition != "safe_mechanical" || !plan.AutoApplicable {
				t.Fatalf("dependency plan was not mechanical: %+v", plan)
			}
		case "external_contract":
			if plan.Disposition != "review_required" || plan.AutoApplicable || plan.ObservedHash != rawSHA256(content) {
				t.Fatalf("external plan was not review-only: %+v", plan)
			}
		default:
			t.Fatalf("unexpected plan kind: %+v", plan)
		}
	}
}
