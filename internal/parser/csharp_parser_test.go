package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCSharpParserLanguage(t *testing.T) {
	p := NewCSharpParser()
	if p.Language() != "csharp" {
		t.Errorf("expected language 'csharp', got '%s'", p.Language())
	}
}

func TestCSharpParserParseInterface(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "csharpparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	csCode := `using System;

namespace Example.Repositories
{
    /// <summary>
    /// Provides data access for user entities
    /// </summary>
    public interface IUserRepository
    {
        /// <summary>
        /// Finds a user by their unique identifier
        /// </summary>
        User FindById(string id);

        /// <summary>
        /// Saves a user to the database
        /// </summary>
        void Save(User user);

        /// <summary>
        /// Deletes a user by ID
        /// </summary>
        void Delete(string id);
    }
}
`
	filePath := filepath.Join(tempDir, "IUserRepository.cs")
	if err := os.WriteFile(filePath, []byte(csCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewCSharpParser()
	extracted, err := p.ParseFile(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	// Verify extraction
	if extracted.ID != "IUserRepository" {
		t.Errorf("expected ID 'IUserRepository', got '%s'", extracted.ID)
	}
	if extracted.Type != "interface" {
		t.Errorf("expected Type 'interface', got '%s'", extracted.Type)
	}
	if extracted.Namespace != "Example.Repositories" {
		t.Errorf("expected Namespace 'Example.Repositories', got '%s'", extracted.Namespace)
	}

	// Verify methods
	if len(extracted.Methods) < 3 {
		t.Errorf("expected at least 3 methods, got %d", len(extracted.Methods))
	}
}

func TestCSharpParserParseClass(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "csharpparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	csCode := `using System;

namespace Example.Services
{
    /// <summary>
    /// Service for managing user operations
    /// </summary>
    public class UserService
    {
        private readonly IUserRepository _repository;
        private readonly ILogger _logger;

        /// <summary>
        /// Creates a new UserService with required dependencies
        /// </summary>
        public UserService(IUserRepository repository, ILogger logger)
        {
            _repository = repository;
            _logger = logger;
        }

        /// <summary>
        /// Gets a user by their ID
        /// </summary>
        public User GetUser(string id)
        {
            return _repository.FindById(id);
        }

        /// <summary>
        /// Creates a new user
        /// </summary>
        public User CreateUser(string name, string email)
        {
            var user = new User { Name = name, Email = email };
            _repository.Save(user);
            return user;
        }

        /// <summary>
        /// Gets the service status
        /// </summary>
        public bool IsActive { get; set; }
    }
}
`
	filePath := filepath.Join(tempDir, "UserService.cs")
	if err := os.WriteFile(filePath, []byte(csCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewCSharpParser()
	extracted, err := p.ParseFile(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	// Verify class
	if extracted.ID != "UserService" {
		t.Errorf("expected ID 'UserService', got '%s'", extracted.ID)
	}
	if extracted.Type != "class" {
		t.Errorf("expected Type 'class', got '%s'", extracted.Type)
	}

	// Verify constructor
	if len(extracted.Constructors) != 1 {
		t.Errorf("expected 1 constructor, got %d", len(extracted.Constructors))
	}

	// Verify methods
	if len(extracted.Methods) < 2 {
		t.Errorf("expected at least 2 methods, got %d", len(extracted.Methods))
	}

	// Verify properties
	if len(extracted.Properties) < 1 {
		t.Errorf("expected at least 1 property, got %d", len(extracted.Properties))
	}

	// Verify dependencies
	if len(extracted.Dependencies) < 2 {
		t.Errorf("expected at least 2 dependencies, got %d", len(extracted.Dependencies))
	}

	depTargets := make(map[string]bool)
	for _, dep := range extracted.Dependencies {
		depTargets[dep.Target] = true
	}
	if !depTargets["IUserRepository"] {
		t.Error("expected dependency on IUserRepository")
	}
	if !depTargets["ILogger"] {
		t.Error("expected dependency on ILogger")
	}
}

func TestCSharpParserParseEvents(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "csharpparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	csCode := `using System;

namespace Example
{
    public class EventSource
    {
        /// <summary>
        /// Raised when data changes
        /// </summary>
        public event EventHandler DataChanged;

        /// <summary>
        /// Raised when an error occurs
        /// </summary>
        public event EventHandler<ErrorEventArgs> ErrorOccurred;
    }
}
`
	filePath := filepath.Join(tempDir, "EventSource.cs")
	if err := os.WriteFile(filePath, []byte(csCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewCSharpParser()
	extracted, err := p.ParseFile(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	// Verify events
	if len(extracted.Events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(extracted.Events))
	}
}

func TestCSharpParserParseFileNotExists(t *testing.T) {
	p := NewCSharpParser()
	_, err := p.ParseFile("/nonexistent/path/file.cs")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestCSharpParserHandlesAsyncMethods(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "csharpparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	csCode := `using System.Threading.Tasks;

namespace Example
{
    public interface IAsyncService
    {
        Task<User> GetUserAsync(string id);
        Task SaveAsync(User user);
        ValueTask<int> CountAsync();
    }
}
`
	filePath := filepath.Join(tempDir, "IAsyncService.cs")
	if err := os.WriteFile(filePath, []byte(csCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewCSharpParser()
	extracted, err := p.ParseFile(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	// Verify async methods were parsed
	if len(extracted.Methods) < 3 {
		t.Errorf("expected at least 3 async methods, got %d", len(extracted.Methods))
	}
}
