package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
)

func TestCheckOrphansHonorsIgnoreRules(t *testing.T) {
	nodes := []*node.Spec{
		{Node: node.NodeInfo{ID: "Bootstrap", Type: "class"}},
		{Node: node.NodeInfo{ID: "SceneManager", Type: "service"}},
		{Node: node.NodeInfo{ID: "MainEntry", Type: "class"}},
		{Node: node.NodeInfo{ID: "Worker", Type: "class"}},
	}

	issues := checkOrphans(nodes, buildSpecLookup(nodes), config.OrphanRules{
		IgnorePatterns: []string{"*Manager"},
		EntryPoints:    []string{"Bootstrap"},
	}, false, "Main*")

	if len(issues) != 1 {
		t.Fatalf("expected 1 orphan issue after filtering, got %d", len(issues))
	}
	if issues[0].SourceNode != "Worker" {
		t.Fatalf("expected Worker orphan, got %s", issues[0].SourceNode)
	}
}

func TestCheckImplementationConsistencyDetectsMissingMembers(t *testing.T) {
	projectRoot := t.TempDir()
	sourcePath := filepath.Join(projectRoot, "agent.go")
	if err := os.WriteFile(sourcePath, []byte(`package agent

type Agent struct{}

func (a *Agent) Execute() error { return nil }
`), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	nodes := []*node.Spec{
		{
			Node: node.NodeInfo{
				ID:       "Agent",
				Type:     "class",
				FilePath: "agent.go",
			},
			Interface: node.Interface{
				Methods: []node.Method{
					{Name: "Execute", Signature: "Execute() error"},
					{Name: "CloneForRun", Signature: "CloneForRun() *Agent"},
				},
			},
		},
	}
	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Project: config.Project{
			Language: "go",
		},
	}

	issues := checkImplementationConsistency(nodes, cfg, false)
	if len(issues) != 1 {
		t.Fatalf("expected 1 implementation issue, got %d", len(issues))
	}
	if issues[0].Category != "impl_mismatch" {
		t.Fatalf("expected impl_mismatch category, got %s", issues[0].Category)
	}
	if issues[0].Severity != "warning" {
		t.Fatalf("expected warning severity, got %s", issues[0].Severity)
	}
	if !strings.Contains(issues[0].Message, "CloneForRun") {
		t.Fatalf("expected missing method in message, got %q", issues[0].Message)
	}
}

func TestCheckImplementationConsistencyDetectsMissingSymbol(t *testing.T) {
	projectRoot := t.TempDir()
	sourcePath := filepath.Join(projectRoot, "agent.go")
	if err := os.WriteFile(sourcePath, []byte(`package agent

type Worker struct{}
`), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	nodes := []*node.Spec{
		{
			Node: node.NodeInfo{
				ID:       "Agent",
				Type:     "class",
				FilePath: "agent.go",
			},
		},
	}
	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Project: config.Project{
			Language: "go",
		},
	}

	issues := checkImplementationConsistency(nodes, cfg, false)
	if len(issues) != 1 {
		t.Fatalf("expected 1 implementation issue, got %d", len(issues))
	}
	if issues[0].Category != "impl_missing" {
		t.Fatalf("expected impl_missing category, got %s", issues[0].Category)
	}
	if issues[0].Severity != "error" {
		t.Fatalf("expected error severity, got %s", issues[0].Severity)
	}
}

func TestEvaluateCheckExitPolicy_DefaultFailsOnErrorsOnly(t *testing.T) {
	prevExitOnWarning := checkExitOnWarning
	prevMaxErrors := checkMaxErrors
	prevMaxWarnings := checkMaxWarnings
	prevMaxInfo := checkMaxInfo
	t.Cleanup(func() {
		checkExitOnWarning = prevExitOnWarning
		checkMaxErrors = prevMaxErrors
		checkMaxWarnings = prevMaxWarnings
		checkMaxInfo = prevMaxInfo
	})

	checkExitOnWarning = false
	checkMaxErrors = -1
	checkMaxWarnings = -1
	checkMaxInfo = -1

	breaches, exitCode := evaluateCheckExitPolicy(1, 0, 0)
	if exitCode != 1 || len(breaches) == 0 {
		t.Fatalf("expected default policy to fail on errors, got exit=%d breaches=%v", exitCode, breaches)
	}

	breaches, exitCode = evaluateCheckExitPolicy(0, 2, 10)
	if exitCode != 0 || len(breaches) != 0 {
		t.Fatalf("expected warnings/info to pass by default, got exit=%d breaches=%v", exitCode, breaches)
	}
}

func TestEvaluateCheckExitPolicy_HonorsWarningAndInfoThresholds(t *testing.T) {
	prevExitOnWarning := checkExitOnWarning
	prevMaxErrors := checkMaxErrors
	prevMaxWarnings := checkMaxWarnings
	prevMaxInfo := checkMaxInfo
	t.Cleanup(func() {
		checkExitOnWarning = prevExitOnWarning
		checkMaxErrors = prevMaxErrors
		checkMaxWarnings = prevMaxWarnings
		checkMaxInfo = prevMaxInfo
	})

	checkExitOnWarning = true
	checkMaxErrors = -1
	checkMaxWarnings = 5
	checkMaxInfo = 10

	breaches, exitCode := evaluateCheckExitPolicy(0, 3, 11)
	if exitCode != 1 {
		t.Fatalf("expected policy to fail, got exit=%d", exitCode)
	}
	if len(breaches) != 2 {
		t.Fatalf("expected 2 breaches (warning presence + info threshold), got %v", breaches)
	}
	if !strings.Contains(strings.Join(breaches, " | "), "warnings present") {
		t.Fatalf("expected warning breach, got %v", breaches)
	}
	if !strings.Contains(strings.Join(breaches, " | "), "max-info") {
		t.Fatalf("expected info threshold breach, got %v", breaches)
	}
}

func TestFormatCISummaryIncludesFailureReasons(t *testing.T) {
	summary := formatCISummary(1, 2, 3, []string{"errors present (1)", "warnings present (2)"}, true)
	if !strings.Contains(summary, "result=FAIL") {
		t.Fatalf("expected FAIL result in summary, got %q", summary)
	}
	if !strings.Contains(summary, "errors present (1)") || !strings.Contains(summary, "warnings present (2)") {
		t.Fatalf("expected breaches in summary, got %q", summary)
	}
}

func TestResolveLayerViolationSeverityHonorsConfigAndStrict(t *testing.T) {
	if got := resolveLayerViolationSeverity("", false); got != "warning" {
		t.Fatalf("expected default warning severity, got %q", got)
	}
	if got := resolveLayerViolationSeverity("info", false); got != "info" {
		t.Fatalf("expected info severity from config, got %q", got)
	}
	if got := resolveLayerViolationSeverity("warning", true); got != "error" {
		t.Fatalf("expected strict mode to force error severity, got %q", got)
	}
}
