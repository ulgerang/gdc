package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
)

func TestMarshalQueryMatchesAllReturnsEveryMatchAsJSONArray(t *testing.T) {
	matches := []*queryMatch{
		{Spec: &node.Spec{Node: node.NodeInfo{ID: "First", FilePath: "shared.go"}}, CanonicalID: "pkg.First", ImplPath: "shared.go"},
		{Spec: &node.Spec{Node: node.NodeInfo{ID: "Second", FilePath: "shared.go"}}, CanonicalID: "pkg.Second", ImplPath: "shared.go"},
	}

	data, err := marshalQueryMatches(matches, true)
	if err != nil {
		t.Fatalf("marshalQueryMatches: %v", err)
	}

	var got []queryNodeJSON
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode all-match JSON: %v\n%s", err, data)
	}
	if len(got) != 2 || got[0].CanonicalID != "pkg.First" || got[1].CanonicalID != "pkg.Second" {
		t.Fatalf("unexpected all-match result: %+v", got)
	}
}

func TestMarshalQueryMatchesAllReturnsEmptyArrayForNoMatches(t *testing.T) {
	data, err := marshalQueryMatches(nil, true)
	if err != nil {
		t.Fatalf("marshalQueryMatches: %v", err)
	}
	if strings.TrimSpace(string(data)) != "[]" {
		t.Fatalf("expected empty JSON array, got %s", data)
	}
}

func TestStructuredQueryOutputSuppressesHumanGuidance(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		if queryAllowsHumanGuidance(format) {
			t.Fatalf("%s output must not include human guidance", format)
		}
	}
	if !queryAllowsHumanGuidance("text") {
		t.Fatal("text output should preserve human guidance")
	}
}

func TestFindMatchingNodesSupportsQualifiedNameAndPaths(t *testing.T) {
	projectRoot := t.TempDir()
	nodesDir := filepath.Join(projectRoot, ".gdc", "nodes")
	controllerPath := filepath.Join(projectRoot, "src", "Controllers", "PlayerController.cs")

	nodes := []*node.Spec{
		{
			Node: node.NodeInfo{
				ID:        "PlayerController",
				Type:      "class",
				Namespace: "Game.Controllers",
				FilePath:  controllerPath,
			},
			Metadata: node.Metadata{Status: "implemented"},
		},
	}

	qualifiedMatches := findMatchingNodes("Game.Controllers.PlayerController", nodes, projectRoot, nodesDir)
	if len(qualifiedMatches) != 1 {
		t.Fatalf("expected one qualified-name match, got %d", len(qualifiedMatches))
	}
	if qualifiedMatches[0].CanonicalID != "Game.Controllers.PlayerController" {
		t.Fatalf("expected canonical qualified ID, got %s", qualifiedMatches[0].CanonicalID)
	}
	if qualifiedMatches[0].MatchedBy != "exact qualified name" {
		t.Fatalf("expected qualified-name match, got %s", qualifiedMatches[0].MatchedBy)
	}

	fileMatches := findMatchingNodes("src/Controllers/PlayerController.cs", nodes, projectRoot, nodesDir)
	if len(fileMatches) != 1 {
		t.Fatalf("expected one file-path match, got %d", len(fileMatches))
	}
	if fileMatches[0].MatchedBy != "exact implementation file" {
		t.Fatalf("expected implementation-file match, got %s", fileMatches[0].MatchedBy)
	}

	partialMatches := findMatchingNodes("playercontroller", nodes, projectRoot, nodesDir)
	if len(partialMatches) != 1 {
		t.Fatalf("expected one partial match, got %d", len(partialMatches))
	}
}

func TestFindSourceHintsDetectsSourceOnlySymbols(t *testing.T) {
	projectRoot := t.TempDir()
	sourceDir := filepath.Join(projectRoot, "src")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	filePath := filepath.Join(sourceDir, "service.go")
	content := `package sample

type GhostService struct{}
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Project: config.Project{
			Language:  "go",
			SourceDir: "src",
		},
	}

	hints, err := findSourceHints(cfg, "GhostService")
	if err != nil {
		t.Fatalf("find source hints: %v", err)
	}
	if len(hints) != 1 {
		t.Fatalf("expected one source hint, got %d (%v)", len(hints), hints)
	}
	if hints[0] != "src/service.go" {
		t.Fatalf("expected src/service.go hint, got %s", hints[0])
	}
}

func TestFindSourceHintsReportsTraversalFailure(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Project: config.Project{
			Language:  "go",
			SourceDir: "missing-source",
		},
	}

	if _, err := findSourceHints(cfg, "GhostService"); err == nil || !strings.Contains(err.Error(), "missing-source") {
		t.Fatalf("expected missing source traversal error, got %v", err)
	}
}

func TestQueryQualifiedNameSuppressesRedundantDottedID(t *testing.T) {
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:        "tools.Runtime",
			Namespace: "tools",
		},
	}

	if got := queryQualifiedName(spec, spec.QualifiedID()); got != "" {
		t.Fatalf("expected redundant qualified name to be suppressed, got %q", got)
	}
}

func TestQueryQualifiedNameKeepsDistinctQualifiedID(t *testing.T) {
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:        "Runtime",
			Namespace: "tools",
		},
	}

	if got := queryQualifiedName(spec, spec.QualifiedID()); got != "tools.Runtime" {
		t.Fatalf("expected qualified name to be kept, got %q", got)
	}
}

func TestQueryQualifiedNameSuppressesPathLikeSyncedID(t *testing.T) {
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:        "infra.config.env_lookup_os.osEnvLookup",
			Namespace: "config",
		},
	}

	if got := queryQualifiedName(spec, spec.QualifiedID()); got != "" {
		t.Fatalf("expected path-like qualified name to be suppressed, got %q", got)
	}
}

func TestSourceExtensionsForLanguageIncludesRust(t *testing.T) {
	extensions := sourceExtensionsForLanguage("rust")
	if len(extensions) != 1 || extensions[0] != ".rs" {
		t.Fatalf("expected rust extensions [.rs], got %v", extensions)
	}
}

func TestSourceExtensionsForLanguageIncludesPython(t *testing.T) {
	extensions := sourceExtensionsForLanguage("python")
	if len(extensions) != 1 || extensions[0] != ".py" {
		t.Fatalf("expected python extensions [.py], got %v", extensions)
	}
}
