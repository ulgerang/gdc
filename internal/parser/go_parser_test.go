package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoParserLanguage(t *testing.T) {
	p := NewGoParser()
	if p.Language() != "go" {
		t.Errorf("expected language 'go', got '%s'", p.Language())
	}
}

func TestGoParserParseInterface(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "goparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test Go file with interface
	goCode := `package example

// UserRepository provides data access for users
type UserRepository interface {
	// FindByID retrieves a user by their ID
	FindByID(id string) (*User, error)

	// Save persists a user to storage
	Save(user *User) error

	// Delete removes a user by ID
	Delete(id string) error
}
`
	filePath := filepath.Join(tempDir, "user_repository.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewGoParser()
	extracted, err := p.ParseFile(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	// Verify extraction
	if extracted.ID != "UserRepository" {
		t.Errorf("expected ID 'UserRepository', got '%s'", extracted.ID)
	}
	if extracted.Type != "interface" {
		t.Errorf("expected Type 'interface', got '%s'", extracted.Type)
	}

	// Verify methods
	if len(extracted.Methods) != 3 {
		t.Errorf("expected 3 methods, got %d", len(extracted.Methods))
	}

	// Check method details
	methodMap := make(map[string]ExtractedMethod)
	for _, m := range extracted.Methods {
		methodMap[m.Name] = m
	}

	if m, ok := methodMap["FindByID"]; ok {
		if m.Description == "" {
			t.Error("expected description for FindByID")
		}
	} else {
		t.Error("method FindByID not found")
	}
}

func TestGoParserParseStruct(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goCode := `package service

// UserService handles user-related business logic
type UserService struct {
	// repo is the user repository
	repo UserRepository
	// logger is the logging service
	logger Logger
}

// NewUserService creates a new UserService with dependencies
func NewUserService(repo UserRepository, logger Logger) *UserService {
	return &UserService{repo: repo, logger: logger}
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id string) (*User, error) {
	return s.repo.FindByID(id)
}

// CreateUser creates a new user
func (s *UserService) CreateUser(name string, email string) (*User, error) {
	user := &User{Name: name, Email: email}
	return user, s.repo.Save(user)
}
`
	filePath := filepath.Join(tempDir, "user_service.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewGoParser()
	extracted, err := p.ParseFile(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	// Verify struct
	if extracted.ID != "UserService" {
		t.Errorf("expected ID 'UserService', got '%s'", extracted.ID)
	}
	if extracted.Type != "class" {
		t.Errorf("expected Type 'class', got '%s'", extracted.Type)
	}

	// Verify constructor was detected
	if len(extracted.Constructors) != 1 {
		t.Errorf("expected 1 constructor, got %d", len(extracted.Constructors))
	}
	if len(extracted.Constructors) > 0 {
		if extracted.Constructors[0].Description == "" {
			t.Error("expected constructor description")
		}
	}

	// Verify methods
	if len(extracted.Methods) < 2 {
		t.Errorf("expected at least 2 methods, got %d", len(extracted.Methods))
	}

	// Verify dependencies from constructor
	if len(extracted.Dependencies) < 2 {
		t.Errorf("expected at least 2 dependencies, got %d", len(extracted.Dependencies))
	}

	depTargets := make(map[string]bool)
	for _, dep := range extracted.Dependencies {
		depTargets[dep.Target] = true
	}
	if !depTargets["UserRepository"] {
		t.Error("expected dependency on UserRepository")
	}
	if !depTargets["Logger"] {
		t.Error("expected dependency on Logger")
	}
}

func TestGoParserParseEmptyFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goCode := `package empty
`
	filePath := filepath.Join(tempDir, "empty.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewGoParser()
	extracted, err := p.ParseFile(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	// Empty file should still parse but have no ID
	if extracted.ID != "" {
		t.Errorf("expected empty ID for empty file, got '%s'", extracted.ID)
	}
}

func TestGoParserParseFileNotExists(t *testing.T) {
	p := NewGoParser()
	_, err := p.ParseFile("/nonexistent/path/file.go")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestGoParserParseInvalidSyntax(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goCode := `package invalid
	
type Broken struct {
	// missing closing brace
`
	filePath := filepath.Join(tempDir, "invalid.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewGoParser()
	_, err = p.ParseFile(filePath)
	if err == nil {
		t.Error("expected error for invalid syntax")
	}
}

func TestGoParserParseFileNodesSeparatesTypes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goCode := `package trace

import (
	"io"
	"time"
)

type Clock interface {
	Now() time.Time
}

type NDJSONWriter struct{}

func NewNDJSONWriter(w io.Writer) *NDJSONWriter {
	return &NDJSONWriter{}
}

func (w *NDJSONWriter) Close() error {
	return nil
}
`
	filePath := filepath.Join(tempDir, "trace.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewGoParser()
	extractedNodes, err := p.ParseFileNodes(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	if len(extractedNodes) != 2 {
		t.Fatalf("expected 2 extracted nodes, got %d", len(extractedNodes))
	}

	nodeMap := make(map[string]*ExtractedNode, len(extractedNodes))
	for _, extracted := range extractedNodes {
		nodeMap[extracted.ID] = extracted
	}

	clock := nodeMap["Clock"]
	if clock == nil {
		t.Fatal("expected Clock node to be extracted")
	}
	if len(clock.Constructors) != 0 {
		t.Fatalf("expected Clock to have no constructors, got %d", len(clock.Constructors))
	}
	if len(clock.Dependencies) != 0 {
		t.Fatalf("expected Clock to have no dependencies, got %d", len(clock.Dependencies))
	}
	if len(clock.Methods) != 1 || clock.Methods[0].Name != "Now" {
		t.Fatalf("expected Clock to expose only Now(), got %+v", clock.Methods)
	}

	writer := nodeMap["NDJSONWriter"]
	if writer == nil {
		t.Fatal("expected NDJSONWriter node to be extracted")
	}
	if len(writer.Constructors) != 1 {
		t.Fatalf("expected NDJSONWriter to have 1 constructor, got %d", len(writer.Constructors))
	}
	if len(writer.Dependencies) != 0 {
		t.Fatalf("expected stdlib io.Writer dependency to be skipped, got %d dependencies", len(writer.Dependencies))
	}
	if len(writer.Methods) != 1 || writer.Methods[0].Name != "Close" {
		t.Fatalf("expected NDJSONWriter to expose Close(), got %+v", writer.Methods)
	}
}

func TestGoParserParseFileNodesNormalizesDependencies(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goMod := `module example.com/test

go 1.23
`
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	depDir := filepath.Join(tempDir, "dep")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatalf("failed to create dep dir: %v", err)
	}
	depCode := `package dep

type AuditLogger interface {
	LogAudit(entry string) error
}
`
	if err := os.WriteFile(filepath.Join(depDir, "dep.go"), []byte(depCode), 0644); err != nil {
		t.Fatalf("failed to write dep file: %v", err)
	}

	goCode := `package hooks

import (
	"io"
	"log/slog"

	"example.com/test/dep"
)

type ProviderClientFactory func(provider, model string) Client

type RotationConfig struct{}

type ProvidersReader interface {
	GetDefaultProvider() string
}

type Client interface {
	Chat() error
}

type RotatingClient struct{}

func NewRotatingClient(factory ProviderClientFactory, config *RotationConfig, providers ProvidersReader, audit dep.AuditLogger, logger *slog.Logger, reader io.Reader) *RotatingClient {
	return &RotatingClient{}
}
`
	filePath := filepath.Join(tempDir, "rotation.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewGoParser()
	extractedNodes, err := p.ParseFileNodes(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	nodeMap := make(map[string]*ExtractedNode, len(extractedNodes))
	for _, extracted := range extractedNodes {
		nodeMap[extracted.ID] = extracted
	}

	rotatingClient := nodeMap["RotatingClient"]
	if rotatingClient == nil {
		t.Fatal("expected RotatingClient node to be extracted")
	}

	depTargets := make(map[string]bool, len(rotatingClient.Dependencies))
	for _, dep := range rotatingClient.Dependencies {
		depTargets[dep.Target] = true
	}

	if !depTargets["ProvidersReader"] {
		t.Fatal("expected ProvidersReader dependency to be preserved")
	}
	if !depTargets["AuditLogger"] {
		t.Fatal("expected local-module dep.AuditLogger dependency to normalize to AuditLogger")
	}
	if depTargets["ProviderClientFactory"] {
		t.Fatal("expected same-file func type dependency to be skipped")
	}
	if depTargets["RotationConfig"] {
		t.Fatal("expected config-like struct dependency to be skipped")
	}
	if depTargets["Logger"] || depTargets["Reader"] {
		t.Fatal("expected stdlib-qualified dependencies to be skipped")
	}
}

func TestGoParserParseFileNodesExtractsTopLevelFunctions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goCode := `package agent

type ConfigFieldEntry struct{}

// ConfigOwnershipMatrix returns the config ownership inventory.
func ConfigOwnershipMatrix() []ConfigFieldEntry {
	return nil
}
`
	filePath := filepath.Join(tempDir, "config_ownership.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewGoParser()
	extractedNodes, err := p.ParseFileNodes(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	nodeMap := make(map[string]*ExtractedNode, len(extractedNodes))
	for _, extracted := range extractedNodes {
		nodeMap[extracted.ID] = extracted
	}

	matrix := nodeMap["ConfigOwnershipMatrix"]
	if matrix == nil {
		t.Fatal("expected ConfigOwnershipMatrix node to be extracted")
	}
	if matrix.Type != "function" {
		t.Fatalf("expected ConfigOwnershipMatrix type function, got %q", matrix.Type)
	}
	if len(matrix.Methods) != 1 || matrix.Methods[0].Name != "ConfigOwnershipMatrix" {
		t.Fatalf("expected function node to expose ConfigOwnershipMatrix method, got %+v", matrix.Methods)
	}
	if matrix.Methods[0].Description == "" {
		t.Fatal("expected top-level function description to be preserved")
	}
}

func TestGoParserParseFileNodesSkipsSelfTypedConstructorDetails(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goCode := `package errors

type OpenSandboxContractError struct {
	ErrorCode string
}

func NewSandboxWorkspaceMissing(details OpenSandboxContractError) *OpenSandboxContractError {
	return &details
}
`
	filePath := filepath.Join(tempDir, "opensandbox_contract.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewGoParser()
	extractedNodes, err := p.ParseFileNodes(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	if len(extractedNodes) != 1 {
		t.Fatalf("expected 1 extracted node, got %d", len(extractedNodes))
	}

	contractErr := extractedNodes[0]
	if contractErr.ID != "OpenSandboxContractError" {
		t.Fatalf("expected OpenSandboxContractError node, got %s", contractErr.ID)
	}
	if len(contractErr.Constructors) != 1 {
		t.Fatalf("expected 1 constructor, got %d", len(contractErr.Constructors))
	}
	if len(contractErr.Dependencies) != 0 {
		t.Fatalf("expected self-typed constructor details to be ignored, got %+v", contractErr.Dependencies)
	}
}

func TestGoParserParseFileNodesExtractsFieldAndMethodDependencies(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	goCode := `package agent

type LLM interface {
	Chat() error
}

type Config struct{}
type Session struct{}
type RunContext struct{}

type Agent struct {
	llm     LLM
	config  *Config
	session Session
}

func (a *Agent) CloneForRun(ctx RunContext) *Agent {
	return a
}
`
	filePath := filepath.Join(tempDir, "agent.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewGoParser()
	extractedNodes, err := p.ParseFileNodes(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	nodeMap := make(map[string]*ExtractedNode, len(extractedNodes))
	for _, extracted := range extractedNodes {
		nodeMap[extracted.ID] = extracted
	}

	agent := nodeMap["Agent"]
	if agent == nil {
		t.Fatal("expected Agent node to be extracted")
	}

	depTargets := make(map[string]bool, len(agent.Dependencies))
	for _, dep := range agent.Dependencies {
		depTargets[dep.Target] = true
	}

	for _, expected := range []string{"LLM", "Config", "Session", "RunContext"} {
		if !depTargets[expected] {
			t.Fatalf("expected dependency on %s, got %+v", expected, agent.Dependencies)
		}
	}
}
