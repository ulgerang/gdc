package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/db"
	"github.com/gdc-tools/gdc/internal/node"
)

func TestNodeDeleteRejectsReferencedNodeWithoutForce(t *testing.T) {
	projectRoot := setupNodeCommandProject(t, map[string]*node.Spec{
		"Alpha": testNodeSpec("Alpha"),
		"Beta": {
			SchemaVersion: "1.0",
			Node:          node.NodeInfo{ID: "Beta", Type: "class"},
			Dependencies:  []node.Dependency{{Target: "Alpha", Type: "class"}},
			Metadata:      node.Metadata{Status: "draft"},
		},
	})

	nodeForce = false
	t.Cleanup(func() { nodeForce = false })
	err := runNodeDelete(nil, []string{"Alpha"})
	if err == nil || !strings.Contains(err.Error(), "Beta") {
		t.Fatalf("expected deletion to report referencing node Beta, got %v", err)
	}

	assertNodeExists(t, projectRoot, "Alpha")
	beta := loadTestNode(t, projectRoot, "Beta")
	if len(beta.Dependencies) != 1 || beta.Dependencies[0].Target != "Alpha" {
		t.Fatalf("expected dependent spec to remain unchanged, got %+v", beta.Dependencies)
	}
}

func TestNodeDeleteForceRemovesReferencesAndRefreshesDatabase(t *testing.T) {
	projectRoot := setupNodeCommandProject(t, map[string]*node.Spec{
		"Alpha": testNodeSpec("Alpha"),
		"Beta": {
			SchemaVersion: "1.0",
			Node:          node.NodeInfo{ID: "Beta", Type: "class"},
			Dependencies:  []node.Dependency{{Target: "Alpha", Type: "class"}},
			Metadata:      node.Metadata{Status: "draft"},
		},
	})

	nodeForce = true
	t.Cleanup(func() { nodeForce = false })
	if err := runNodeDelete(nil, []string{"Alpha"}); err != nil {
		t.Fatalf("forced deletion failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".gdc", "nodes", "Alpha.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected Alpha.yaml to be deleted, stat error: %v", err)
	}
	beta := loadTestNode(t, projectRoot, "Beta")
	if len(beta.Dependencies) != 0 {
		t.Fatalf("expected forced deletion to remove dependency, got %+v", beta.Dependencies)
	}

	database, err := db.Open(filepath.Join(projectRoot, ".gdc", "graph.db"))
	if err != nil {
		t.Fatalf("open refreshed database: %v", err)
	}
	defer database.Close()
	if _, err := database.GetNode("Beta"); err != nil {
		t.Fatalf("expected Beta in refreshed database: %v", err)
	}
	if record, err := database.GetNode("Alpha"); err != nil || record != nil {
		t.Fatalf("expected Alpha to be absent from refreshed database, record=%+v err=%v", record, err)
	}
}

func TestNodeRenameUpdatesReferencesAndRefreshesDatabase(t *testing.T) {
	projectRoot := setupNodeCommandProject(t, map[string]*node.Spec{
		"Alpha": testNodeSpec("Alpha"),
		"Beta": {
			SchemaVersion: "1.0",
			Node:          node.NodeInfo{ID: "Beta", Type: "class"},
			Dependencies:  []node.Dependency{{Target: "Alpha", Type: "class"}},
			Metadata:      node.Metadata{Status: "draft"},
		},
	})

	if err := runNodeRename(nil, []string{"Alpha", "Gamma"}); err != nil {
		t.Fatalf("rename failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".gdc", "nodes", "Alpha.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected old node file to be absent, stat error: %v", err)
	}
	gamma := loadTestNode(t, projectRoot, "Gamma")
	if gamma.Node.ID != "Gamma" {
		t.Fatalf("expected renamed node ID Gamma, got %q", gamma.Node.ID)
	}
	beta := loadTestNode(t, projectRoot, "Beta")
	if len(beta.Dependencies) != 1 || beta.Dependencies[0].Target != "Gamma" {
		t.Fatalf("expected dependency to target Gamma, got %+v", beta.Dependencies)
	}

	database, err := db.Open(filepath.Join(projectRoot, ".gdc", "graph.db"))
	if err != nil {
		t.Fatalf("open refreshed database: %v", err)
	}
	defer database.Close()
	if _, err := database.GetNode("Gamma"); err != nil {
		t.Fatalf("expected Gamma in refreshed database: %v", err)
	}
	if record, err := database.GetNode("Alpha"); err != nil || record != nil {
		t.Fatalf("expected Alpha to be absent from refreshed database, record=%+v err=%v", record, err)
	}
}

func TestNodeRenameTargetExistsLeavesSpecsUnchanged(t *testing.T) {
	projectRoot := setupNodeCommandProject(t, map[string]*node.Spec{
		"Alpha": testNodeSpec("Alpha"),
		"Gamma": testNodeSpec("Gamma"),
	})

	before, err := os.ReadFile(filepath.Join(projectRoot, ".gdc", "nodes", "Alpha.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := runNodeRename(nil, []string{"Alpha", "Gamma"}); err == nil {
		t.Fatal("expected rename to existing target to fail")
	}
	after, err := os.ReadFile(filepath.Join(projectRoot, ".gdc", "nodes", "Alpha.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("expected failed rename to leave source spec unchanged")
	}
}

func TestNodeDeleteReportsDatabaseRefreshRecovery(t *testing.T) {
	projectRoot := setupNodeCommandProject(t, map[string]*node.Spec{
		"Alpha": testNodeSpec("Alpha"),
	})
	configPath := filepath.Join(projectRoot, ".gdc", "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Database.Path = ".gdc/nodes"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	nodeForce = false
	err = runNodeDelete(nil, []string{"Alpha"})
	if err == nil || !strings.Contains(err.Error(), "gdc sync") {
		t.Fatalf("expected actionable database recovery error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(projectRoot, ".gdc", "nodes", "Alpha.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("expected YAML mutation to remain authoritative, stat error: %v", statErr)
	}
}

func setupNodeCommandProject(t *testing.T, specs map[string]*node.Spec) string {
	t.Helper()
	projectRoot := t.TempDir()
	configPath := filepath.Join(projectRoot, ".gdc", "config.yaml")
	cfg := config.DefaultConfig()
	cfg.Project.Name = "node-command-test"
	cfg.Project.Language = "go"
	cfg.ProjectRoot = projectRoot
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	for name, spec := range specs {
		if err := node.Save(filepath.Join(projectRoot, ".gdc", "nodes", name+".yaml"), spec); err != nil {
			t.Fatalf("save node %s: %v", name, err)
		}
	}
	t.Setenv("GDC_CONFIG", configPath)
	return projectRoot
}

func testNodeSpec(id string) *node.Spec {
	return &node.Spec{
		SchemaVersion: "1.0",
		Node:          node.NodeInfo{ID: id, Type: "class"},
		Metadata:      node.Metadata{Status: "draft"},
	}
}

func loadTestNode(t *testing.T, projectRoot, id string) *node.Spec {
	t.Helper()
	spec, err := node.Load(filepath.Join(projectRoot, ".gdc", "nodes", id+".yaml"))
	if err != nil {
		t.Fatalf("load node %s: %v", id, err)
	}
	return spec
}

func assertNodeExists(t *testing.T, projectRoot, id string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(projectRoot, ".gdc", "nodes", id+".yaml")); err != nil {
		t.Fatalf("expected node %s to exist: %v", id, err)
	}
}
