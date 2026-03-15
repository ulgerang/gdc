package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestInitSchemaResetsLegacyNodesTableAndAllowsFunctionType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "graph.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

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
		ID:        "ValidateConfigOwnershipMatrix",
		Type:      "function",
		Status:    "implemented",
		SpecPath:  ".gdc/nodes/ValidateConfigOwnershipMatrix.yaml",
		UpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("expected function type to be accepted after schema init, got %v", err)
	}
}
