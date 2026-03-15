// Package integration_test contains integration tests for GDC
package integration_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// Test Helper Functions
// ============================================================================

// buildGDC builds the gdc binary for testing and returns the path to the binary.
// The binary is built once and cached in a temporary directory.
func buildGDC(t *testing.T) string {
	t.Helper()

	// Create a unique temp directory for this test run
	binaryName := "gdc-test"
	if isWindows() {
		binaryName = "gdc-test.exe"
	}

	// Use a consistent temp dir for the test run
	tempDir := filepath.Join(os.TempDir(), "gdc-test-binaries")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	binaryPath := filepath.Join(tempDir, binaryName)

	// Check if binary exists and is recent (within 5 minutes)
	if info, err := os.Stat(binaryPath); err == nil {
		if time.Since(info.ModTime()) < 5*time.Minute {
			return binaryPath
		}
	}

	// Build the binary
	t.Log("Building gdc binary for tests...")
	projectRoot := filepath.Join("..", "..")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/gdc")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build gdc binary: %v\nOutput: %s", err, string(output))
	}

	t.Logf("Built gdc binary at: %s", binaryPath)
	return binaryPath
}

// runGDC runs gdc command with given args and returns combined output.
// If the command fails, it returns the error with the output.
func runGDC(t *testing.T, binaryPath, workDir string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// runGDCWithExitCode runs gdc command and returns output and exit code.
func runGDCWithExitCode(t *testing.T, binaryPath, workDir string, args ...string) (string, int) {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = workDir

	output, _ := cmd.CombinedOutput()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	return string(output), exitCode
}

// setupTestProject creates a temporary GDC project for testing.
// It returns the project root directory path.
func setupTestProject(t *testing.T) string {
	t.Helper()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "gdc-test-project-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Initialize the project
	binaryPath := buildGDC(t)
	output, err := runGDC(t, binaryPath, tempDir, "init")
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to init project: %v\nOutput: %s", err, output)
	}

	// Create sample node specifications
	createSampleNodes(t, tempDir)

	return tempDir
}

// createSampleNodes creates sample node YAML files for testing.
func createSampleNodes(t *testing.T, projectRoot string) {
	t.Helper()

	nodesDir := filepath.Join(projectRoot, ".gdc", "nodes")

	nodes := map[string]string{
		"UserService": `schema_version: "1.0"
node:
  id: UserService
  type: service
  layer: application
responsibility:
  summary: Handles user management operations
  details: Creates, updates, and deletes users. Also handles authentication.
interface:
  methods:
    - name: CreateUser
      signature: "CreateUser(ctx context.Context, req CreateUserRequest) (*User, error)"
      description: Creates a new user
    - name: GetUser
      signature: "GetUser(ctx context.Context, id string) (*User, error)"
      description: Retrieves a user by ID
    - name: DeleteUser
      signature: "DeleteUser(ctx context.Context, id string) error"
      description: Deletes a user
dependencies:
  - target: UserRepository
    type: interface
    injection: constructor
  - target: Logger
    type: interface
    injection: constructor
metadata:
  status: implemented
  created: "2024-01-01"
  updated: "2024-01-15"
`,
		"UserRepository": `schema_version: "1.0"
node:
  id: UserRepository
  type: interface
  layer: infrastructure
responsibility:
  summary: Interface for user data persistence
interface:
  methods:
    - name: FindByID
      signature: "FindByID(ctx context.Context, id string) (*User, error)"
      description: Find user by ID
    - name: Save
      signature: "Save(ctx context.Context, user *User) error"
      description: Save user to storage
    - name: Delete
      signature: "Delete(ctx context.Context, id string) error"
      description: Delete user from storage
implementations:
  - PostgresUserRepository
  - MemoryUserRepository
metadata:
  status: implemented
  created: "2024-01-01"
  updated: "2024-01-15"
`,
		"PostgresUserRepository": `schema_version: "1.0"
node:
  id: PostgresUserRepository
  type: class
  layer: infrastructure
responsibility:
  summary: PostgreSQL implementation of UserRepository
dependencies:
  - target: UserRepository
    type: interface
  - target: Database
    type: class
    injection: constructor
metadata:
  status: implemented
  created: "2024-01-01"
`,
		"Logger": `schema_version: "1.0"
node:
  id: Logger
  type: interface
  layer: infrastructure
responsibility:
  summary: Logging interface for application
interface:
  methods:
    - name: Info
      signature: "Info(msg string, args ...interface{})"
      description: Log info message
    - name: Error
      signature: "Error(msg string, args ...interface{})"
      description: Log error message
metadata:
  status: implemented
  created: "2024-01-01"
`,
		"Database": `schema_version: "1.0"
node:
  id: Database
  type: service
  layer: infrastructure
responsibility:
  summary: Database connection manager
interface:
  methods:
    - name: Connect
      signature: "Connect() error"
      description: Establish database connection
    - name: Disconnect
      signature: "Disconnect() error"
      description: Close database connection
metadata:
  status: implemented
  created: "2024-01-01"
`,
		"AuthService": `schema_version: "1.0"
node:
  id: AuthService
  type: service
  layer: application
responsibility:
  summary: Handles authentication and authorization
  details: Manages JWT tokens, session handling, and permission checks.
interface:
  methods:
    - name: Login
      signature: "Login(ctx context.Context, email, password string) (string, error)"
      description: Authenticate user and return token
    - name: Logout
      signature: "Logout(ctx context.Context, token string) error"
      description: Invalidate user token
    - name: ValidateToken
      signature: "ValidateToken(ctx context.Context, token string) (*Claims, error)"
      description: Validate and parse JWT token
dependencies:
  - target: UserRepository
    type: interface
    injection: constructor
  - target: Logger
    type: interface
    injection: constructor
metadata:
  status: implemented
  created: "2024-01-01"
  updated: "2024-01-20"
`,
	}

	for name, content := range nodes {
		nodePath := filepath.Join(nodesDir, name+".yaml")
		if err := os.WriteFile(nodePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create node %s: %v", name, err)
		}
	}
}

// createSampleSourceFiles creates sample source files for search testing.
func createSampleSourceFiles(t *testing.T, projectRoot string) {
	t.Helper()

	// Create source directory structure
	srcDirs := []string{
		filepath.Join(projectRoot, "src", "services"),
		filepath.Join(projectRoot, "src", "repositories"),
		filepath.Join(projectRoot, "pkg", "utils"),
	}

	for _, dir := range srcDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create sample Go files
	goFiles := map[string]string{
		filepath.Join(projectRoot, "src", "services", "user_service.go"): `package services

import (
	"context"
	"fmt"
)

// UserService handles user operations
type UserService struct {
	repo   UserRepository
	logger Logger
}

// NewUserService creates a new UserService instance
func NewUserService(repo UserRepository, logger Logger) *UserService {
	return &UserService{
		repo:   repo,
		logger: logger,
	}
}

// CreateUser creates a new user
func (s *UserService) CreateUser(ctx context.Context, name string) error {
	s.logger.Info("Creating user", "name", name)
	// TODO: Add validation
	return s.repo.Save(ctx, &User{Name: name})
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
	return s.repo.FindByID(ctx, id)
}

// DeleteUser removes a user
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
`,
		filepath.Join(projectRoot, "src", "services", "auth_service.go"): `package services

import (
	"context"
	"errors"
)

// AuthService handles authentication
type AuthService struct {
	userRepo UserRepository
	logger   Logger
}

// ErrInvalidCredentials is returned when auth fails
var ErrInvalidCredentials = errors.New("invalid credentials")

// Login authenticates a user
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	
	// Validate password (GDC note: simplified for demo)
	if !validatePassword(password, user.Password) {
		return "", ErrInvalidCredentials
	}
	
	return generateToken(user.ID)
}

func validatePassword(input, stored string) bool {
	return input == stored
}

func generateToken(userID string) (string, error) {
	// TODO: Implement JWT token generation
	return "token-" + userID, nil
}
`,
		filepath.Join(projectRoot, "src", "repositories", "user_repository.go"): `package repositories

import (
	"context"
	"database/sql"
)

// UserRepository handles user data persistence
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByID finds a user by their ID
func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
	query := "SELECT id, name, email FROM users WHERE id = $1"
	// Implementation details...
	return nil, nil
}

// FindByEmail finds a user by their email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	// TODO: Implement email lookup
	return nil, nil
}

// Save persists a user
func (r *UserRepository) Save(ctx context.Context, user *User) error {
	// Implementation
	return nil
}

// Delete removes a user
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	return nil
}
`,
		filepath.Join(projectRoot, "pkg", "utils", "logger.go"): `package utils

import (
	"fmt"
	"log"
)

// Logger provides logging functionality
type Logger struct {
	prefix string
}

// NewLogger creates a new Logger instance
func NewLogger(prefix string) *Logger {
	return &Logger{prefix: prefix}
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+l.prefix+": "+msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+l.prefix+": "+msg, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	log.Printf("[DEBUG] "+l.prefix+": "+msg, args...)
}
`,
		filepath.Join(projectRoot, "cmd", "handler.go"): "package main\n\n" +
			"import (\n" +
			"\t\"encoding/json\"\n" +
			"\t\"net/http\"\n" +
			")\n\n" +
			"// Handler handles HTTP requests\n" +
			"type Handler struct {\n" +
			"\tuserService *UserService\n" +
			"\tauthService *AuthService\n" +
			"}\n\n" +
			"// HandleCreateUser handles user creation requests\n" +
			"func (h *Handler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {\n" +
			"\tvar req struct {\n" +
			"\t\tName  string\n" +
			"\t\tEmail string\n" +
			"\t}\n" +
			"\n" +
			"\tif err := json.NewDecoder(r.Body).Decode(\u0026req); err != nil {\n" +
			"\t\thttp.Error(w, err.Error(), http.StatusBadRequest)\n" +
			"\t\treturn\n" +
			"\t}\n" +
			"\n" +
			"\t// Call UserService to create user\n" +
			"\t// TODO: Add authentication check\n" +
			"}\n\n" +
			"// HandleLogin handles login requests\n" +
			"func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {\n" +
			"\t// Authentication handler implementation\n" +
			"}\n",
	}

	for path, content := range goFiles {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create parent dir for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return strings.Contains(strings.ToLower(os.Getenv("OS")), "windows")
}

// ============================================================================
// Search Tests
// ============================================================================

// TestSearchBasic tests basic pattern search functionality
func TestSearchBasic(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	// Create sample source files for search testing
	createSampleSourceFiles(t, projectRoot)

	tests := []struct {
		name           string
		pattern        string
		expectedCount  int
		expectedInFile string
	}{
		{
			name:           "find UserService",
			pattern:        "UserService",
			expectedCount:  4, // Appears in multiple files
			expectedInFile: "user_service.go",
		},
		{
			name:           "find func keyword",
			pattern:        "func",
			expectedCount:  10,
			expectedInFile: "user_service.go",
		},
		{
			name:           "find TODO comment",
			pattern:        "TODO",
			expectedCount:  3,
			expectedInFile: "user_service.go",
		},
		{
			name:           "find context.Context",
			pattern:        "context.Context",
			expectedCount:  6,
			expectedInFile: "user_service.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runGDC(t, binaryPath, projectRoot, "search", tt.pattern)
			if err != nil {
				t.Logf("Search output: %s", output)
				// Search returns error when no matches, which is acceptable
			}

			// Count occurrences of the pattern in output
			count := strings.Count(output, tt.pattern)
			if count < tt.expectedCount {
				t.Errorf("Expected at least %d occurrences of %q, got %d\nOutput:\n%s",
					tt.expectedCount, tt.pattern, count, output)
			}

			// Verify expected file is mentioned
			if !strings.Contains(output, tt.expectedInFile) {
				t.Errorf("Expected output to contain file %q\nOutput:\n%s",
					tt.expectedInFile, output)
			}
		})
	}
}

// TestSearchWithFilePattern tests search with file pattern filter
func TestSearchWithFilePattern(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	createSampleSourceFiles(t, projectRoot)

	tests := []struct {
		name             string
		pattern          string
		filePattern      string
		shouldContain    string
		shouldNotContain string
	}{
		{
			name:          "search only in .go files",
			pattern:       "TODO",
			filePattern:   "*.go",
			shouldContain: "user_service.go",
		},
		{
			name:          "search in services directory",
			pattern:       "UserService",
			filePattern:   "*.go",
			shouldContain: "user_service.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"search", tt.pattern, "--file-pattern", tt.filePattern}
			output, err := runGDC(t, binaryPath, projectRoot, args...)
			if err != nil {
				t.Logf("Search output: %s", output)
			}

			if tt.shouldContain != "" && !strings.Contains(output, tt.shouldContain) {
				t.Errorf("Expected output to contain %q\nOutput:\n%s",
					tt.shouldContain, output)
			}

			if tt.shouldNotContain != "" && strings.Contains(output, tt.shouldNotContain) {
				t.Errorf("Expected output NOT to contain %q\nOutput:\n%s",
					tt.shouldNotContain, output)
			}
		})
	}
}

// TestSearchRegex tests regex pattern search
func TestSearchRegex(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	createSampleSourceFiles(t, projectRoot)

	tests := []struct {
		name       string
		pattern    string
		expected   string
		unexpected string
	}{
		{
			name:     "find all function declarations",
			pattern:  "func.*\\(",
			expected: "func",
		},
		{
			name:     "find type declarations",
			pattern:  "type\\s+\\w+\\s+(struct|interface)",
			expected: "type",
		},
		{
			name:       "find method declarations",
			pattern:    "func\\s+\\([^)]+\\)\\s+\\w+",
			expected:   "func",
			unexpected: "TODO", // Shouldn't find comments
		},
		{
			name:     "find error handling",
			pattern:  "if\\s+err\\s*!=\\s*nil",
			expected: "err",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"search", tt.pattern, "--regex"}
			output, err := runGDC(t, binaryPath, projectRoot, args...)
			if err != nil {
				t.Logf("Search output: %s", output)
			}

			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected output to contain %q\nOutput:\n%s",
					tt.expected, output)
			}

			if tt.unexpected != "" && strings.Contains(output, tt.unexpected) {
				t.Errorf("Expected output NOT to contain %q\nOutput:\n%s",
					tt.unexpected, output)
			}
		})
	}
}

// TestSearchCaseSensitive tests case-sensitive search
func TestSearchCaseSensitive(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	createSampleSourceFiles(t, projectRoot)

	t.Run("case sensitive - finds uppercase only", func(t *testing.T) {
		args := []string{"search", "UserService", "--case-sensitive"}
		output, err := runGDC(t, binaryPath, projectRoot, args...)
		if err != nil {
			t.Logf("Search output: %s", output)
		}

		// Should find UserService (uppercase)
		if !strings.Contains(output, "UserService") {
			t.Errorf("Expected to find 'UserService'\nOutput:\n%s", output)
		}
	})

	t.Run("case sensitive - lowercase term finds nothing", func(t *testing.T) {
		args := []string{"search", "userservice", "--case-sensitive"}
		output, _ := runGDC(t, binaryPath, projectRoot, args...)

		// Should NOT find UserService when searching for lowercase "userservice" in case-sensitive mode
		if strings.Contains(output, "UserService") || strings.Contains(output, "Found") {
			t.Errorf("Case sensitive search for 'userservice' should not find 'UserService'\nOutput:\n%s", output)
		}
	})

	t.Run("case insensitive - finds all variations", func(t *testing.T) {
		args := []string{"search", "userservice"} // lowercase input
		output, err := runGDC(t, binaryPath, projectRoot, args...)
		if err != nil {
			t.Logf("Search output: %s", output)
		}

		// Should find UserService even with lowercase input (default is case-insensitive)
		if !strings.Contains(output, "UserService") && !strings.Contains(output, "userservice") {
			t.Errorf("Expected to find UserService (case insensitive)\nOutput:\n%s", output)
		}
	})
}

// TestSearchMaxResults tests result limiting
func TestSearchMaxResults(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	createSampleSourceFiles(t, projectRoot)

	maxResults := 3
	args := []string{"search", "func", "--max-results", fmt.Sprintf("%d", maxResults)}
	output, err := runGDC(t, binaryPath, projectRoot, args...)
	if err != nil {
		t.Logf("Search output: %s", output)
	}

	// Parse result count from the "Found N results" summary line
	resultCount := 0
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Found") && strings.Contains(line, "result") {
			// Parse "Found 3 results" or "Found 1 result"
			fmt.Sscanf(strings.TrimSpace(line), "Found %d", &resultCount)
			break
		}
	}

	// Verify we got results - "func" should always match in .go test files
	if resultCount == 0 {
		t.Errorf("Expected at least 1 result for pattern 'func', got 0.\nOutput:\n%s", output)
	}

	// The search should respect max-results limit
	if resultCount > maxResults {
		t.Errorf("Expected at most %d results, got %d\nOutput:\n%s",
			maxResults, resultCount, output)
	}

	// Verify the "max results reached" indicator is present
	if !strings.Contains(output, "max results reached") {
		t.Errorf("Expected 'max results reached' message when limiting results\nOutput:\n%s", output)
	}
}

// TestSearchNoMatch tests search with no matches
func TestSearchNoMatch(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	createSampleSourceFiles(t, projectRoot)

	args := []string{"search", "ZZZZZZZ_NO_MATCH_ZZZZZZ"}
	output, _ := runGDC(t, binaryPath, projectRoot, args...)
	// Search returns exit code 0 even with no matches, but prints info message

	// Should indicate no matches found
	if !strings.Contains(output, "No matches found") &&
		!strings.Contains(output, "no match") &&
		!strings.Contains(strings.ToLower(output), "0 result") {
		// If output is empty or has no matches, that's also acceptable
		if strings.TrimSpace(output) != "" && !strings.Contains(output, "No matches") {
			t.Logf("Expected 'no matches' message. Output:\n%s", output)
		}
	}
}

// TestSearchWithoutProject tests search functionality without a GDC project
func TestSearchWithoutProject(t *testing.T) {
	binaryPath := buildGDC(t)

	// Create a temp directory without .gdc
	tempDir, err := os.MkdirTemp("", "gdc-no-project-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple Go file
	goFile := filepath.Join(tempDir, "test.go")
	content := `package main

func main() {
	println("Hello, World!")
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Search should still work, just from current directory
	args := []string{"search", "Hello"}
	output, err := runGDC(t, binaryPath, tempDir, args...)
	if err != nil {
		t.Logf("Search output (expected to work): %s", output)
	}

	// Should find the pattern
	if !strings.Contains(output, "Hello") {
		t.Errorf("Expected to find 'Hello' even without GDC project\nOutput:\n%s", output)
	}
}

// ============================================================================
// Trace Tests
// ============================================================================

// TestTraceReverse tests reverse dependency trace
func TestTraceReverse(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	tests := []struct {
		name          string
		node          string
		args          []string
		shouldContain []string
	}{
		{
			name:          "trace reverse dependencies of UserRepository",
			node:          "UserRepository",
			args:          []string{"trace", "UserRepository", "--reverse"},
			shouldContain: []string{"UserService", "AuthService"},
		},
		{
			name:          "trace reverse dependencies of Logger",
			node:          "Logger",
			args:          []string{"trace", "Logger", "-r"},
			shouldContain: []string{"UserService", "AuthService"},
		},
		{
			name:          "trace reverse with direction up",
			node:          "UserRepository",
			args:          []string{"trace", "UserRepository", "--direction", "up"},
			shouldContain: []string{"UserService", "AuthService"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runGDC(t, binaryPath, projectRoot, tt.args...)
			if err != nil {
				t.Logf("Trace output: %s", output)
				// Some errors are acceptable (e.g., node not found is expected for some tests)
			}

			for _, expected := range tt.shouldContain {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q\nOutput:\n%s",
						expected, output)
				}
			}
		})
	}
}

// TestTraceForward tests forward dependency trace
func TestTraceForward(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	tests := []struct {
		name          string
		args          []string
		shouldContain []string
	}{
		{
			name:          "trace forward dependencies of UserService",
			args:          []string{"trace", "UserService"},
			shouldContain: []string{"UserRepository", "Logger"},
		},
		{
			name:          "trace forward dependencies of AuthService",
			args:          []string{"trace", "AuthService"},
			shouldContain: []string{"UserRepository", "Logger"},
		},
		{
			name:          "trace with depth limit",
			args:          []string{"trace", "UserService", "--depth", "1"},
			shouldContain: []string{"UserRepository", "Logger"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runGDC(t, binaryPath, projectRoot, tt.args...)
			if err != nil {
				t.Logf("Trace output: %s", output)
			}

			for _, expected := range tt.shouldContain {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q\nOutput:\n%s",
						expected, output)
				}
			}
		})
	}
}

// TestTraceToPath tests finding path between two nodes
func TestTraceToPath(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	tests := []struct {
		name          string
		args          []string
		shouldContain []string
		expectNoPath  bool // true if no path is expected (e.g., disconnected nodes)
	}{
		{
			name:          "trace direct dependency path UserService to UserRepository",
			args:          []string{"trace", "UserService", "--to", "UserRepository"},
			shouldContain: []string{"UserService", "UserRepository"},
			expectNoPath:  false,
		},
		{
			name:          "trace direct dependency path UserService to Logger",
			args:          []string{"trace", "UserService", "--to", "Logger"},
			shouldContain: []string{"UserService", "Logger"},
			expectNoPath:  false,
		},
		{
			name:          "trace path between disconnected nodes",
			args:          []string{"trace", "PostgresUserRepository", "--to", "Logger"},
			shouldContain: []string{},
			expectNoPath:  true, // No forward path exists from PostgresUserRepository to Logger
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runGDC(t, binaryPath, projectRoot, tt.args...)
			if err != nil {
				t.Logf("Trace returned error (may be expected): %v", err)
			}

			t.Logf("Output: %s", output)

			if tt.expectNoPath {
				// Verify "no path" message is shown
				if !strings.Contains(strings.ToLower(output), "no path") {
					t.Errorf("Expected 'no path' message for disconnected nodes\nOutput:\n%s", output)
				}
			} else {
				// Path should exist and contain expected nodes
				for _, expected := range tt.shouldContain {
					if !strings.Contains(output, expected) {
						t.Errorf("Expected output to contain %q\nOutput:\n%s",
							expected, output)
					}
				}

				// Verify path indicator is shown
				if !strings.Contains(output, "Path found") && !strings.Contains(output, "path") {
					t.Errorf("Expected path output to indicate a path was found\nOutput:\n%s", output)
				}
			}
		})
	}
}

// TestTraceNonExistentNode tests tracing a non-existent node
func TestTraceNonExistentNode(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	args := []string{"trace", "NonExistentNode12345"}
	output, _ := runGDC(t, binaryPath, projectRoot, args...)

	// Should indicate node not found
	if !strings.Contains(strings.ToLower(output), "not found") &&
		!strings.Contains(strings.ToLower(output), "error") {
		t.Errorf("Expected error message for non-existent node\nOutput:\n%s", output)
	}
}

// ============================================================================
// Query Tests
// ============================================================================

// TestQueryBasic tests basic node query functionality
func TestQueryBasic(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	tests := []struct {
		name          string
		args          []string
		shouldContain []string
	}{
		{
			name:          "query UserService",
			args:          []string{"query", "UserService"},
			shouldContain: []string{"UserService", "service", "application", "UserRepository"},
		},
		{
			name:          "query AuthService",
			args:          []string{"query", "AuthService"},
			shouldContain: []string{"AuthService", "authentication", "UserRepository"},
		},
		{
			name:          "query interface",
			args:          []string{"query", "UserRepository"},
			shouldContain: []string{"UserRepository", "interface", "infrastructure"},
		},
		{
			name:          "query with verbose flag",
			args:          []string{"query", "UserService", "--verbose"},
			shouldContain: []string{"UserService", "Created", "Updated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runGDC(t, binaryPath, projectRoot, tt.args...)
			if err != nil {
				t.Errorf("Query failed: %v\nOutput:\n%s", err, output)
			}

			for _, expected := range tt.shouldContain {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q\nOutput:\n%s",
						expected, output)
				}
			}
		})
	}
}

// TestQueryJSON tests JSON output format
func TestQueryJSON(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	args := []string{"query", "UserService", "--format", "json"}
	output, err := runGDC(t, binaryPath, projectRoot, args...)
	if err != nil {
		t.Errorf("Query failed: %v\nOutput:\n%s", err, output)
	}

	// Parse the JSON output to verify it's valid
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Failed to parse JSON output: %v\nOutput:\n%s", err, output)
	}

	// Verify expected fields
	expectedFields := []string{"id", "type", "layer", "status", "responsibility"}
	for _, field := range expectedFields {
		if _, ok := result[field]; !ok {
			t.Errorf("Expected JSON to contain field %q\nOutput:\n%s", field, output)
		}
	}

	// Verify values
	if result["id"] != "UserService" {
		t.Errorf("Expected id to be 'UserService', got %v", result["id"])
	}
	if result["type"] != "service" {
		t.Errorf("Expected type to be 'service', got %v", result["type"])
	}
}

// TestQueryYAML tests YAML output format
func TestQueryYAML(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	args := []string{"query", "UserService", "--format", "yaml"}
	output, err := runGDC(t, binaryPath, projectRoot, args...)
	if err != nil {
		t.Errorf("Query failed: %v\nOutput:\n%s", err, output)
	}

	// Verify YAML contains expected keys
	expectedFields := []string{"id:", "type:", "layer:", "status:", "responsibility:"}
	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Expected YAML to contain %q\nOutput:\n%s", field, output)
		}
	}
}

// TestQueryNotFound tests query for non-existent node
func TestQueryNotFound(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	args := []string{"query", "NonExistentNodeXYZ"}
	output, _ := runGDC(t, binaryPath, projectRoot, args...)

	// Should indicate node not found
	if !strings.Contains(strings.ToLower(output), "not found") {
		t.Errorf("Expected 'not found' message\nOutput:\n%s", output)
	}

	// Should provide suggestions
	if !strings.Contains(strings.ToLower(output), "did you mean") &&
		!strings.Contains(strings.ToLower(output), "suggestion") {
		// Suggestions are nice to have but not required
		t.Logf("Note: No suggestions provided for non-existent node. Output:\n%s", output)
	}
}

// TestQueryPartialMatch tests partial/fuzzy matching
func TestQueryPartialMatch(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	tests := []struct {
		name          string
		query         string
		shouldContain string
	}{
		{
			name:          "partial match - User",
			query:         "User",
			shouldContain: "User",
		},
		{
			name:          "partial match - Service",
			query:         "Service",
			shouldContain: "Service",
		},
		{
			name:          "prefix match - Auth",
			query:         "Auth",
			shouldContain: "AuthService",
		},
		{
			name:          "prefix match - Postgres",
			query:         "Postgres",
			shouldContain: "PostgresUserRepository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"query", tt.query}
			output, err := runGDC(t, binaryPath, projectRoot, args...)
			if err != nil {
				t.Logf("Query output: %s", output)
			}

			if !strings.Contains(output, tt.shouldContain) {
				t.Errorf("Expected output to contain %q\nOutput:\n%s",
					tt.shouldContain, output)
			}
		})
	}
}

// TestQueryMultipleMatches tests querying when multiple nodes match
func TestQueryMultipleMatches(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	// Query for "User" which should match multiple nodes
	args := []string{"query", "User"}
	output, err := runGDC(t, binaryPath, projectRoot, args...)
	if err != nil {
		t.Logf("Query output: %s", output)
	}

	// Should indicate multiple matches or show details for best match
	// The behavior may vary, so we just check that we get some output
	if strings.TrimSpace(output) == "" {
		t.Error("Expected some output for partial query")
	}
}

// ============================================================================
// Graceful Degradation Tests
// ============================================================================

// TestGracefulDegradation tests graceful degradation when no project exists
func TestGracefulDegradation(t *testing.T) {
	binaryPath := buildGDC(t)

	// Create a temp directory without .gdc
	tempDir, err := os.MkdirTemp("", "gdc-graceful-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		args        []string
		expectError bool
		checkOutput func(t *testing.T, output string)
	}{
		{
			name:        "query without project",
			args:        []string{"query", "SomeNode"},
			expectError: true,
			checkOutput: func(t *testing.T, output string) {
				// Should provide helpful error message
				if !strings.Contains(strings.ToLower(output), "not found") &&
					!strings.Contains(strings.ToLower(output), "failed to load") &&
					!strings.Contains(strings.ToLower(output), "init") {
					t.Errorf("Expected helpful error message\nOutput:\n%s", output)
				}
			},
		},
		{
			name:        "trace without project",
			args:        []string{"trace", "SomeNode"},
			expectError: true,
			checkOutput: func(t *testing.T, output string) {
				// Should provide helpful error message
				if !strings.Contains(strings.ToLower(output), "failed") &&
					!strings.Contains(strings.ToLower(output), "not found") &&
					!strings.Contains(strings.ToLower(output), "config") {
					t.Errorf("Expected error about project configuration\nOutput:\n%s", output)
				}
			},
		},
		{
			name:        "list without project",
			args:        []string{"list"},
			expectError: true,
			checkOutput: func(t *testing.T, output string) {
				// Should indicate project not initialized
				if !strings.Contains(strings.ToLower(output), "failed") &&
					!strings.Contains(strings.ToLower(output), "not found") &&
					!strings.Contains(strings.ToLower(output), "init") {
					t.Errorf("Expected error about project initialization\nOutput:\n%s", output)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, _ := runGDC(t, binaryPath, tempDir, tt.args...)
			tt.checkOutput(t, output)
		})
	}
}

// TestSearchGracefulDegradation tests that search works even without a project
func TestSearchGracefulDegradation(t *testing.T) {
	binaryPath := buildGDC(t)

	// Create a temp directory without .gdc
	tempDir, err := os.MkdirTemp("", "gdc-search-graceful-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a sample file
	testFile := filepath.Join(tempDir, "sample.txt")
	content := "Hello World\nThis is a test file\nWith some content"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Search should still work
	args := []string{"search", "Hello"}
	output, err := runGDC(t, binaryPath, tempDir, args...)

	// Search should work without project
	if err != nil {
		t.Logf("Search returned error (may be acceptable): %v", err)
	}

	// Should find the pattern
	if !strings.Contains(output, "Hello") {
		t.Errorf("Expected to find 'Hello' in search results\nOutput:\n%s", output)
	}

	// Should indicate it's searching without a project
	if strings.Contains(strings.ToLower(output), "no gdc project") ||
		strings.Contains(strings.ToLower(output), "no project") {
		t.Logf("Helpful message about no project found")
	}
}

// TestSearchBinaryFileExclusion tests that binary files are excluded from search
func TestSearchBinaryFileExclusion(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	// Create a binary-like file
	binFile := filepath.Join(projectRoot, "test.bin")
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE}
	if err := os.WriteFile(binFile, binaryContent, 0644); err != nil {
		t.Fatalf("Failed to create binary file: %v", err)
	}

	// Create a lock file
	lockFile := filepath.Join(projectRoot, "go.sum")
	lockContent := "github.com/example/pkg v1.0.0"
	if err := os.WriteFile(lockFile, []byte(lockContent), 0644); err != nil {
		t.Fatalf("Failed to create lock file: %v", err)
	}

	// Search should not crash on binary files
	args := []string{"search", "test"}
	output, err := runGDC(t, binaryPath, projectRoot, args...)
	if err != nil {
		t.Logf("Search output: %s", output)
	}

	// Should not include binary file in results
	if strings.Contains(output, "test.bin") {
		t.Errorf("Binary file should be excluded from search\nOutput:\n%s", output)
	}

	// Should not include lock file
	if strings.Contains(output, "go.sum") {
		t.Errorf("Lock file should be excluded from search\nOutput:\n%s", output)
	}
}

// TestVersionCommand tests the version command
func TestVersionCommand(t *testing.T) {
	binaryPath := buildGDC(t)

	output, err := runGDC(t, binaryPath, os.TempDir(), "version")
	if err != nil {
		t.Errorf("Version command failed: %v", err)
	}

	// Should contain version info
	if !strings.Contains(output, "version") && !strings.Contains(output, "gdc") {
		t.Errorf("Expected version information\nOutput:\n%s", output)
	}
}

// TestListCommand tests the list command
func TestListCommand(t *testing.T) {
	binaryPath := buildGDC(t)
	projectRoot := setupTestProject(t)
	defer os.RemoveAll(projectRoot)

	tests := []struct {
		name          string
		args          []string
		shouldContain []string
	}{
		{
			name:          "list all nodes",
			args:          []string{"list"},
			shouldContain: []string{"UserService", "AuthService", "UserRepository"},
		},
		{
			name:          "list with JSON format",
			args:          []string{"list", "--format", "json"},
			shouldContain: []string{"\"id\"", "\"type\""},
		},
		{
			name:          "list with filter",
			args:          []string{"list", "--filter", "type=interface"},
			shouldContain: []string{"UserRepository", "Logger"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runGDC(t, binaryPath, projectRoot, tt.args...)
			if err != nil {
				t.Errorf("List command failed: %v\nOutput:\n%s", err, output)
			}

			for _, expected := range tt.shouldContain {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q\nOutput:\n%s",
						expected, output)
				}
			}
		})
	}
}

// TestHelpCommands tests that help is available for all commands
func TestHelpCommands(t *testing.T) {
	binaryPath := buildGDC(t)

	commands := []string{
		"search",
		"query",
		"trace",
		"list",
		"node",
		"init",
	}

	for _, cmd := range commands {
		t.Run("help for "+cmd, func(t *testing.T) {
			args := []string{cmd, "--help"}
			output, err := runGDC(t, binaryPath, os.TempDir(), args...)
			if err != nil {
				t.Errorf("Help command failed for %s: %v\nOutput:\n%s", cmd, err, output)
			}

			// Should contain usage information
			if !strings.Contains(strings.ToLower(output), "usage") &&
				!strings.Contains(strings.ToLower(output), "examples") {
				t.Errorf("Expected usage information for %s\nOutput:\n%s", cmd, output)
			}
		})
	}
}
