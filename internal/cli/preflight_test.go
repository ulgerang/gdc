package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/node"
)

func TestEvaluateReadinessRequiresExplicitSelectionForMultipleProfiles(t *testing.T) {
	spec := profiledImplementationSpec("Verifier", "sealed")
	spec.ImplementationContract.Profiles = append(spec.ImplementationContract.Profiles,
		node.ImplementationProfile{
			ID:          "unity-publish",
			Description: "Verify published plugin bytes.",
			Requires:    []string{"plugin provenance"},
			Forbids:     []string{"mixed source and plugin binding"},
			Acceptance:  []string{"VERIFIER-SUCCESS"},
		})

	report := evaluateImplementationReadiness(spec, buildSpecLookup([]*node.Spec{spec}), t.TempDir(), "", "implementation")
	if report.ContractComplete || report.ImplementationPermitted {
		t.Fatalf("multiple profiles without selection were accepted: %+v", report)
	}
	if !containsReadinessText(report.Missing, "profile is required") || !containsReadinessText(report.Missing, "headless") {
		t.Fatalf("available profiles were not reported: %+v", report)
	}
}

func TestEvaluateReadinessSeparatesCompleteReadyContractFromSealing(t *testing.T) {
	spec := profiledImplementationSpec("Verifier", "ready")
	report := evaluateImplementationReadiness(spec, buildSpecLookup([]*node.Spec{spec}), t.TempDir(), "headless", "implementation")
	if !report.ContractComplete || !report.DependencyClosureComplete {
		t.Fatalf("complete amendable contract was treated as incomplete: %+v", report)
	}
	if report.Sealed || report.ImplementationPermitted {
		t.Fatalf("ready contract was allowed to authorize implementation: %+v", report)
	}
	if !containsReadinessText(report.BlockedBy, "sealed") {
		t.Fatalf("sealing blocker was not reported: %+v", report)
	}
}

func TestEvaluateReadinessScopesGatesByProfileAndPhase(t *testing.T) {
	spec := profiledImplementationSpec("Verifier", "sealed")
	spec.ImplementationContract.Profiles = append(spec.ImplementationContract.Profiles,
		node.ImplementationProfile{
			ID:          "unity-publish",
			Description: "Verify published plugin bytes.",
			Requires:    []string{"plugin provenance"},
			Forbids:     []string{"headless plugin binding"},
			Acceptance:  []string{"VERIFIER-SUCCESS"},
		})
	spec.ImplementationContract.Gates = []node.ImplementationGate{
		{
			ID: "publish-provenance", Kind: "provenance", Phase: "publish", Status: "blocked",
			Description: "Published bytes are not available.", Profiles: []string{"unity-publish"},
		},
		{
			ID: "unity-implementation-approval", Kind: "approval", Phase: "implementation", Status: "blocked",
			Description: "Unity implementation is not approved.", Profiles: []string{"unity-publish"},
		},
	}

	headless := evaluateImplementationReadiness(spec, buildSpecLookup([]*node.Spec{spec}), t.TempDir(), "headless", "implementation")
	if !headless.ImplementationPermitted || !headless.GatesSatisfied {
		t.Fatalf("unrelated Unity/publish gates blocked headless implementation: %+v", headless)
	}

	unityImplementation := evaluateImplementationReadiness(spec, buildSpecLookup([]*node.Spec{spec}), t.TempDir(), "unity-publish", "implementation")
	if unityImplementation.ImplementationPermitted || !containsReadinessText(unityImplementation.BlockedBy, "unity-implementation-approval") {
		t.Fatalf("relevant implementation gate did not block Unity profile: %+v", unityImplementation)
	}

	unityPublish := evaluateImplementationReadiness(spec, buildSpecLookup([]*node.Spec{spec}), t.TempDir(), "unity-publish", "publish")
	if unityPublish.PhasePermitted || !containsReadinessText(unityPublish.BlockedBy, "publish-provenance") {
		t.Fatalf("publish-phase provenance gate was not applied: %+v", unityPublish)
	}
}

func TestEvaluateReadinessClosesAndEmbedsSelectedExternalContracts(t *testing.T) {
	root := t.TempDir()
	contractPath := filepath.Join(root, "contracts", "headless.json")
	if err := os.MkdirAll(filepath.Dir(contractPath), 0o755); err != nil {
		t.Fatalf("create contract directory: %v", err)
	}
	content := []byte("{\"mode\":\"headless\",\"pluginRequired\":false}\n")
	if err := os.WriteFile(contractPath, content, 0o644); err != nil {
		t.Fatalf("write external contract: %v", err)
	}

	spec := profiledImplementationSpec("Verifier", "sealed")
	spec.ImplementationContract.Profiles = append(spec.ImplementationContract.Profiles,
		node.ImplementationProfile{
			ID:          "unity-publish",
			Description: "Verify published plugin bytes.",
			Requires:    []string{"plugin provenance"},
			Forbids:     []string{"headless plugin binding"},
			Acceptance:  []string{"VERIFIER-SUCCESS"},
		})
	spec.ImplementationContract.ExternalContracts = []node.ExternalContract{
		{
			ID: "headless-mode", Path: "contracts/headless.json", ContractHash: rawSHA256(content),
			Description: "Headless verification boundary.", Profiles: []string{"headless"},
		},
		{
			ID: "publish-plugin", Path: "contracts/plugin.json", ContractHash: strings.Repeat("0", 64),
			Description: "Publish-only provenance.", Profiles: []string{"unity-publish"},
		},
	}

	report := evaluateImplementationReadiness(spec, buildSpecLookup([]*node.Spec{spec}), root, "headless", "implementation")
	if !report.ExternalContractsComplete || !report.ImplementationPermitted {
		t.Fatalf("matching selected external contract did not close: %+v", report)
	}
	if len(report.ExternalContracts) != 1 || report.ExternalContracts[0].ID != "headless-mode" || report.ExternalContracts[0].Content != string(content) {
		t.Fatalf("selected contract was not embedded exactly once: %+v", report.ExternalContracts)
	}
}

func TestEvaluateReadinessReportsObservedHashForStaleExternalContract(t *testing.T) {
	root := t.TempDir()
	contractPath := filepath.Join(root, "contract.json")
	content := []byte("{\"state\":\"candidate\"}\n")
	if err := os.WriteFile(contractPath, content, 0o644); err != nil {
		t.Fatalf("write external contract: %v", err)
	}

	spec := profiledImplementationSpec("Verifier", "sealed")
	spec.ImplementationContract.ExternalContracts = []node.ExternalContract{{
		ID: "authority", Path: "contract.json", ContractHash: strings.Repeat("0", 64),
		Description: "Reviewed authority contract.", Profiles: []string{"headless"},
	}}
	report := evaluateImplementationReadiness(spec, buildSpecLookup([]*node.Spec{spec}), root, "headless", "implementation")
	if report.ExternalContractsComplete || report.ImplementationPermitted {
		t.Fatalf("stale external contract was accepted: %+v", report)
	}
	if !containsReadinessText(report.Missing, rawSHA256(content)) {
		t.Fatalf("observed replacement hash was not reported: %+v", report.Missing)
	}
}

func TestEvaluateReadinessRejectsExternalContractPathTraversal(t *testing.T) {
	spec := profiledImplementationSpec("Verifier", "sealed")
	spec.ImplementationContract.ExternalContracts = []node.ExternalContract{{
		ID: "escape", Path: "../outside.json", ContractHash: strings.Repeat("0", 64),
		Description: "Invalid escaped contract.", Profiles: []string{"headless"},
	}}
	report := evaluateImplementationReadiness(spec, buildSpecLookup([]*node.Spec{spec}), t.TempDir(), "headless", "implementation")
	if report.ExternalContractsComplete || !containsReadinessText(report.Missing, "repository-relative") {
		t.Fatalf("path traversal was not rejected: %+v", report)
	}
}

func profiledImplementationSpec(id, status string) *node.Spec {
	spec := readyImplementationSpec(id, "Verify")
	spec.SchemaVersion = "1.2"
	spec.ImplementationContract.Status = status
	spec.ImplementationContract.ClosedWorld = true
	spec.ImplementationContract.Profiles = []node.ImplementationProfile{{
		ID:          "headless",
		Description: "Verify direct repository-local project references.",
		Requires:    []string{"vendored source identity"},
		Forbids:     []string{"published plugin bytes"},
		Acceptance:  []string{strings.ToUpper(id) + "-SUCCESS"},
	}}
	return spec
}

func rawSHA256(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func containsReadinessText(values []string, fragment string) bool {
	return strings.Contains(strings.Join(values, "\n"), fragment)
}
