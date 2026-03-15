// Package db handles SQLite database operations for GDC
package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Database wraps the SQLite connection
type Database struct {
	conn *sql.DB
	path string
}

// NodeRecord represents a node in the database
type NodeRecord struct {
	QualifiedID    string
	ID             string
	Type           string
	Layer          string
	Namespace      string
	SpecPath       string
	ImplPath       string
	Responsibility string
	Status         string
	SpecHash       string
	ImplHash       string
	ContractHash   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EdgeRecord represents a dependency edge
type EdgeRecord struct {
	ID             int64
	FromNode       string
	ToNode         string
	DependencyType string
	InjectionType  string
	IsOptional     bool
	ContractHash   string
	UsageSummary   string
}

// InterfaceMember represents a method/property/event
type InterfaceMember struct {
	ID          int64
	NodeID      string
	MemberType  string // method, property, event, constructor
	Name        string
	Signature   string
	ReturnType  string
	Description string
}

// Open opens or creates a SQLite database
func Open(path string) (*Database, error) {
	conn, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set connection options
	conn.SetMaxOpenConns(1) // SQLite only supports one writer

	return &Database{conn: conn, path: path}, nil
}

// Close closes the database connection
func (db *Database) Close() error {
	return db.conn.Close()
}

// InitSchema creates the database schema
func (db *Database) InitSchema() error {
	if err := db.resetDerivedTablesForLegacyNodeTypes(); err != nil {
		return err
	}

	schema := `
	-- Nodes table
	CREATE TABLE IF NOT EXISTS nodes (
		qualified_id TEXT PRIMARY KEY,
		id TEXT NOT NULL,
		type TEXT NOT NULL CHECK(type IN ('class', 'interface', 'module', 'service', 'enum', 'function')),
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

	-- Edges (dependencies) table
	CREATE TABLE IF NOT EXISTS edges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_node TEXT NOT NULL,
		to_node TEXT NOT NULL,
		dependency_type TEXT DEFAULT 'interface',
		injection_type TEXT DEFAULT 'constructor',
		is_optional INTEGER DEFAULT 0,
		contract_hash TEXT,
		usage_summary TEXT,
		FOREIGN KEY (from_node) REFERENCES nodes(id) ON DELETE CASCADE,
		-- Note: to_node has no FK to allow referencing not-yet-created nodes
		UNIQUE(from_node, to_node)
	);

	-- Interface members table
	CREATE TABLE IF NOT EXISTS interface_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id TEXT NOT NULL,
		member_type TEXT NOT NULL CHECK(member_type IN ('method', 'property', 'event', 'constructor')),
		name TEXT NOT NULL,
		signature TEXT,
		return_type TEXT,
		description TEXT,
		FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
	);

	-- Tags table
	CREATE TABLE IF NOT EXISTS tags (
		node_id TEXT NOT NULL,
		tag TEXT NOT NULL,
		PRIMARY KEY (node_id, tag),
		FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
	);

	-- Sync log table
	CREATE TABLE IF NOT EXISTS sync_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		nodes_created INTEGER DEFAULT 0,
		nodes_updated INTEGER DEFAULT 0,
		nodes_deleted INTEGER DEFAULT 0,
		duration_ms INTEGER
	);

	-- Validation issues table
	CREATE TABLE IF NOT EXISTS validation_issues (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		check_time DATETIME DEFAULT CURRENT_TIMESTAMP,
		node_id TEXT,
		category TEXT NOT NULL,
		severity TEXT NOT NULL,
		message TEXT NOT NULL,
		suggestion TEXT,
		is_resolved INTEGER DEFAULT 0,
		FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE SET NULL
	);

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_nodes_id ON nodes(id);
	CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
	CREATE INDEX IF NOT EXISTS idx_nodes_layer ON nodes(layer);
	CREATE INDEX IF NOT EXISTS idx_nodes_namespace ON nodes(namespace);
	CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
	CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_node);
	CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_node);
	CREATE INDEX IF NOT EXISTS idx_interface_members_node ON interface_members(node_id);
	CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
	`

	_, err := db.conn.Exec(schema)
	return err
}

func (db *Database) resetDerivedTablesForLegacyNodeTypes() error {
	var createSQL sql.NullString
	err := db.conn.QueryRow(`
		SELECT sql
		FROM sqlite_master
		WHERE type = 'table' AND name = 'nodes'
	`).Scan(&createSQL)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	if strings.Contains(createSQL.String, "'function'") && strings.Contains(createSQL.String, "qualified_id") {
		return nil
	}

	_, err = db.conn.Exec(`
		DROP TABLE IF EXISTS edges;
		DROP TABLE IF EXISTS interface_members;
		DROP TABLE IF EXISTS tags;
		DROP TABLE IF EXISTS nodes;
	`)
	return err
}

// GetAllNodes returns all nodes from the database
func (db *Database) GetAllNodes() ([]*NodeRecord, error) {
	rows, err := db.conn.Query(`
		SELECT qualified_id, id, type, layer, namespace, spec_path, impl_path, 
			   responsibility, status, spec_hash, impl_hash, 
			   created_at, updated_at
		FROM nodes
		ORDER BY qualified_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*NodeRecord
	for rows.Next() {
		n := &NodeRecord{}
		var layer, namespace, specPath, implPath, responsibility sql.NullString
		var specHash, implHash sql.NullString

		err := rows.Scan(
			&n.QualifiedID, &n.ID, &n.Type, &layer, &namespace, &specPath, &implPath,
			&responsibility, &n.Status, &specHash, &implHash,
			&n.CreatedAt, &n.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		n.Layer = layer.String
		n.Namespace = namespace.String
		n.SpecPath = specPath.String
		n.ImplPath = implPath.String
		n.Responsibility = responsibility.String
		n.SpecHash = specHash.String
		n.ImplHash = implHash.String

		nodes = append(nodes, n)
	}

	return nodes, rows.Err()
}

// GetNode returns a single node by ID
func (db *Database) GetNode(qualifiedID string) (*NodeRecord, error) {
	n := &NodeRecord{}
	var layer, namespace, specPath, implPath, responsibility sql.NullString
	var specHash, implHash sql.NullString

	err := db.conn.QueryRow(`
		SELECT qualified_id, id, type, layer, namespace, spec_path, impl_path,
			   responsibility, status, spec_hash, impl_hash,
			   created_at, updated_at
		FROM nodes WHERE qualified_id = ?
	`, qualifiedID).Scan(
		&n.QualifiedID, &n.ID, &n.Type, &layer, &namespace, &specPath, &implPath,
		&responsibility, &n.Status, &specHash, &implHash,
		&n.CreatedAt, &n.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	n.Layer = layer.String
	n.Namespace = namespace.String
	n.SpecPath = specPath.String
	n.ImplPath = implPath.String
	n.Responsibility = responsibility.String
	n.SpecHash = specHash.String
	n.ImplHash = implHash.String

	return n, nil
}

// UpsertNode inserts or updates a node
func (db *Database) UpsertNode(n *NodeRecord) error {
	_, err := db.conn.Exec(`
		INSERT INTO nodes (qualified_id, id, type, layer, namespace, spec_path, impl_path,
						   responsibility, status, spec_hash, impl_hash, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(qualified_id) DO UPDATE SET
			id = excluded.id,
			type = excluded.type,
			layer = excluded.layer,
			namespace = excluded.namespace,
			spec_path = excluded.spec_path,
			impl_path = excluded.impl_path,
			responsibility = excluded.responsibility,
			status = excluded.status,
			spec_hash = excluded.spec_hash,
			impl_hash = excluded.impl_hash,
			updated_at = excluded.updated_at
	`, n.QualifiedID, n.ID, n.Type, n.Layer, n.Namespace, n.SpecPath, n.ImplPath,
		n.Responsibility, n.Status, n.SpecHash, n.ImplHash, time.Now())

	return err
}

// DeleteNode removes a node from the database
func (db *Database) DeleteNode(qualifiedID string) error {
	_, err := db.conn.Exec("DELETE FROM nodes WHERE qualified_id = ?", qualifiedID)
	return err
}

// InsertEdge inserts a dependency edge
func (db *Database) InsertEdge(e *EdgeRecord) error {
	optional := 0
	if e.IsOptional {
		optional = 1
	}

	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO edges 
		(from_node, to_node, dependency_type, injection_type, is_optional, contract_hash, usage_summary)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, e.FromNode, e.ToNode, e.DependencyType, e.InjectionType,
		optional, e.ContractHash, e.UsageSummary)

	return err
}

// DeleteEdgesFrom removes all edges from a node
func (db *Database) DeleteEdgesFrom(nodeID string) error {
	_, err := db.conn.Exec("DELETE FROM edges WHERE from_node = ?", nodeID)
	return err
}

// GetEdgesFrom returns all edges from a node
func (db *Database) GetEdgesFrom(nodeID string) ([]*EdgeRecord, error) {
	rows, err := db.conn.Query(`
		SELECT id, from_node, to_node, dependency_type, injection_type, is_optional, contract_hash, usage_summary
		FROM edges WHERE from_node = ?
	`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []*EdgeRecord
	for rows.Next() {
		e := &EdgeRecord{}
		var optional int
		var contractHash, usageSummary sql.NullString

		err := rows.Scan(
			&e.ID, &e.FromNode, &e.ToNode, &e.DependencyType,
			&e.InjectionType, &optional, &contractHash, &usageSummary,
		)
		if err != nil {
			return nil, err
		}

		e.IsOptional = optional == 1
		e.ContractHash = contractHash.String
		e.UsageSummary = usageSummary.String

		edges = append(edges, e)
	}

	return edges, rows.Err()
}

// GetEdgesTo returns all edges to a node
func (db *Database) GetEdgesTo(nodeID string) ([]*EdgeRecord, error) {
	rows, err := db.conn.Query(`
		SELECT id, from_node, to_node, dependency_type, injection_type, is_optional
		FROM edges WHERE to_node = ?
	`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []*EdgeRecord
	for rows.Next() {
		e := &EdgeRecord{}
		var optional int

		err := rows.Scan(
			&e.ID, &e.FromNode, &e.ToNode, &e.DependencyType,
			&e.InjectionType, &optional,
		)
		if err != nil {
			return nil, err
		}

		e.IsOptional = optional == 1
		edges = append(edges, e)
	}

	return edges, rows.Err()
}

// InsertInterfaceMember inserts an interface member
func (db *Database) InsertInterfaceMember(m *InterfaceMember) error {
	_, err := db.conn.Exec(`
		INSERT INTO interface_members 
		(node_id, member_type, name, signature, return_type, description)
		VALUES (?, ?, ?, ?, ?, ?)
	`, m.NodeID, m.MemberType, m.Name, m.Signature, m.ReturnType, m.Description)

	return err
}

// DeleteInterfaceMembers removes all interface members for a node
func (db *Database) DeleteInterfaceMembers(nodeID string) error {
	_, err := db.conn.Exec("DELETE FROM interface_members WHERE node_id = ?", nodeID)
	return err
}

// InsertTag inserts a tag for a node
func (db *Database) InsertTag(nodeID, tag string) error {
	_, err := db.conn.Exec(`
		INSERT OR IGNORE INTO tags (node_id, tag) VALUES (?, ?)
	`, nodeID, tag)
	return err
}

// DeleteTags removes all tags for a node
func (db *Database) DeleteTags(nodeID string) error {
	_, err := db.conn.Exec("DELETE FROM tags WHERE node_id = ?", nodeID)
	return err
}

// GetNodesByTag returns all nodes with a specific tag
func (db *Database) GetNodesByTag(tag string) ([]string, error) {
	rows, err := db.conn.Query("SELECT node_id FROM tags WHERE tag = ?", tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodeIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		nodeIDs = append(nodeIDs, id)
	}

	return nodeIDs, rows.Err()
}

// LogSync records a sync operation
func (db *Database) LogSync(created, updated, deleted int, durationMs int64) error {
	_, err := db.conn.Exec(`
		INSERT INTO sync_log (nodes_created, nodes_updated, nodes_deleted, duration_ms)
		VALUES (?, ?, ?, ?)
	`, created, updated, deleted, durationMs)
	return err
}

// GetStats returns basic statistics
func (db *Database) GetStats() (map[string]int, error) {
	stats := make(map[string]int)

	// Total nodes
	var total int
	db.conn.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&total)
	stats["total_nodes"] = total

	// Total edges
	var edges int
	db.conn.QueryRow("SELECT COUNT(*) FROM edges").Scan(&edges)
	stats["total_edges"] = edges

	// By type
	rows, err := db.conn.Query("SELECT type, COUNT(*) FROM nodes GROUP BY type")
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var t string
		var count int
		rows.Scan(&t, &count)
		stats["type_"+t] = count
	}

	return stats, nil
}
