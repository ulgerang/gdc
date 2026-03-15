package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
)

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

	hints := findSourceHints(cfg, "GhostService")
	if len(hints) != 1 {
		t.Fatalf("expected one source hint, got %d (%v)", len(hints), hints)
	}
	if hints[0] != "src/service.go" {
		t.Fatalf("expected src/service.go hint, got %s", hints[0])
	}
}
