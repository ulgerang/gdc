package db

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitSchemaResetsLegacyNodesTableAndAllowsFunctionType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "graph.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	_, err = database.conn.Exec(`
		CREATE TABLE nodes (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL CHECK(type IN ('class', 'interface', 'module', 'service', 'enum')),
			layer TEXT,
			namespace TEXT,
			spec_path TEXT,
			impl_path TEXT,
			responsibility TEXT,
			status TEXT DEFAULT 'draft',
			spec_hash TEXT,
			impl_hash TEXT,
			contract_hash TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE edges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_node TEXT NOT NULL,
			to_node TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("failed to create legacy schema: %v", err)
	}

	if err := database.InitSchema(); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	err = database.UpsertNode(&NodeRecord{
		QualifiedID: "ValidateConfigOwnershipMatrix",
		ID:          "ValidateConfigOwnershipMatrix",
		Type:        "function",
		Status:      "implemented",
		SpecPath:    ".gdc/nodes/ValidateConfigOwnershipMatrix.yaml",
		UpdatedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("expected function type to be accepted after schema init, got %v", err)
	}
}

func TestGetStatsReturnsContextualErrorWhenDatabaseIsClosed(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "graph.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := database.InitSchema(); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	stats, err := database.GetStats()
	if err == nil {
		t.Fatal("expected GetStats to report the closed database")
	}
	if stats != nil {
		t.Fatalf("expected no partial statistics, got %#v", stats)
	}
	if !strings.Contains(err.Error(), "count nodes") {
		t.Fatalf("expected node count context, got %v", err)
	}
}
