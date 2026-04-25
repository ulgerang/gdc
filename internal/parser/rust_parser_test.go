package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRustParserLanguage(t *testing.T) {
	p := NewRustParser()
	if p.Language() != "rust" {
		t.Fatalf("expected language rust, got %q", p.Language())
	}
}

func TestRustParserParseFileNodesExtractsTraitAndStruct(t *testing.T) {
	tempDir := t.TempDir()
	rustCode := `use std::sync::Arc;

pub trait UserRepository {
    fn find_by_id(&self, id: &str) -> Result<User, RepoError>;
    fn save(&self, user: User) -> Result<(), RepoError>;
}

pub struct UserService {
    repo: Arc<dyn UserRepository>,
    logger: Logger,
}

impl UserService {
    pub fn new(repo: Arc<dyn UserRepository>, logger: Logger) -> Self {
        Self { repo, logger }
    }

    pub async fn get_user(&self, id: &str) -> Result<User, ServiceError> {
        todo!()
    }

    fn helper(&self) {}
}
`

	filePath := filepath.Join(tempDir, "service.rs")
	if err := os.WriteFile(filePath, []byte(rustCode), 0o644); err != nil {
		t.Fatalf("failed to write rust fixture: %v", err)
	}

	p := NewRustParser()
	nodes, err := p.ParseFileNodes(filePath)
	if err != nil {
		t.Fatalf("failed to parse rust file: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 extracted nodes, got %d", len(nodes))
	}

	nodeMap := make(map[string]*ExtractedNode, len(nodes))
	for _, extracted := range nodes {
		nodeMap[extracted.ID] = extracted
	}

	repo := nodeMap["UserRepository"]
	if repo == nil {
		t.Fatal("expected UserRepository trait to be extracted")
	}
	if repo.Type != "interface" {
		t.Fatalf("expected trait to map to interface, got %q", repo.Type)
	}
	if len(repo.Methods) != 2 {
		t.Fatalf("expected 2 trait methods, got %d", len(repo.Methods))
	}

	service := nodeMap["UserService"]
	if service == nil {
		t.Fatal("expected UserService struct to be extracted")
	}
	if service.Type != "class" {
		t.Fatalf("expected struct to map to class, got %q", service.Type)
	}
	if len(service.Constructors) != 1 {
		t.Fatalf("expected one constructor-style method, got %d", len(service.Constructors))
	}
	if len(service.Methods) != 1 || service.Methods[0].Name != "get_user" {
		t.Fatalf("expected only public inherent methods, got %+v", service.Methods)
	}

	depTargets := make(map[string]bool, len(service.Dependencies))
	for _, dep := range service.Dependencies {
		depTargets[dep.Target] = true
	}
	if !depTargets["UserRepository"] {
		t.Fatalf("expected dependency on UserRepository, got %+v", service.Dependencies)
	}
	if !depTargets["Logger"] {
		t.Fatalf("expected dependency on Logger, got %+v", service.Dependencies)
	}
}

func TestRustParserParseFileReturnsFirstExtractedNode(t *testing.T) {
	tempDir := t.TempDir()
	rustCode := `pub trait Clock {
    fn now(&self) -> i64;
}

pub struct SystemClock;
`

	filePath := filepath.Join(tempDir, "clock.rs")
	if err := os.WriteFile(filePath, []byte(rustCode), 0o644); err != nil {
		t.Fatalf("failed to write rust fixture: %v", err)
	}

	p := NewRustParser()
	extracted, err := p.ParseFile(filePath)
	if err != nil {
		t.Fatalf("failed to parse rust file: %v", err)
	}
	if extracted.ID != "Clock" {
		t.Fatalf("expected first public item to be returned, got %q", extracted.ID)
	}
}

func TestRustParserDoesNotDropParametersContainingSelfInName(t *testing.T) {
	tempDir := t.TempDir()
	rustCode := `pub struct AuditService;

impl AuditService {
    pub fn record(&'a mut self, self_user: User, event: AuditEvent) -> Result<(), AuditError> {
        todo!()
    }

    pub fn replay(self: Box<Self>, self_user: User) -> Result<(), AuditError> {
        todo!()
    }
}
`

	filePath := filepath.Join(tempDir, "audit.rs")
	if err := os.WriteFile(filePath, []byte(rustCode), 0o644); err != nil {
		t.Fatalf("failed to write rust fixture: %v", err)
	}

	p := NewRustParser()
	nodes, err := p.ParseFileNodes(filePath)
	if err != nil {
		t.Fatalf("failed to parse rust file: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected one extracted node, got %d", len(nodes))
	}

	methods := nodes[0].Methods
	if len(methods) != 2 {
		t.Fatalf("expected two public methods, got %+v", methods)
	}

	params := methods[0].Parameters
	if len(params) != 2 {
		t.Fatalf("expected receiver to be skipped and two real params kept, got %+v", params)
	}
	if params[0].Name != "self_user" || params[0].Type != "User" {
		t.Fatalf("expected self_user parameter to be preserved, got %+v", params[0])
	}

	depTargets := make(map[string]bool, len(nodes[0].Dependencies))
	for _, dep := range nodes[0].Dependencies {
		depTargets[dep.Target] = true
	}
	for _, expected := range []string{"User", "AuditEvent"} {
		if !depTargets[expected] {
			t.Fatalf("expected dependency on %s, got %+v", expected, nodes[0].Dependencies)
		}
	}
}
