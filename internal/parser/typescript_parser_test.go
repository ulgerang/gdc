package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTypeScriptParserLanguage(t *testing.T) {
	p := NewTypeScriptParser()
	if p.Language() != "typescript" {
		t.Errorf("expected language 'typescript', got '%s'", p.Language())
	}
}

func TestTypeScriptParserParseInterface(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tsparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tsCode := `/**
 * Repository for user data access
 */
export interface IUserRepository {
  /**
   * Find a user by their ID
   * @param id The user ID
   * @returns The user or null
   */
  findById(id: string): Promise<User | null>;

  /**
   * Save a user to storage
   */
  save(user: User): Promise<void>;

  /**
   * Delete a user by ID
   */
  delete(id: string): Promise<void>;
}
`
	filePath := filepath.Join(tempDir, "IUserRepository.ts")
	if err := os.WriteFile(filePath, []byte(tsCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewTypeScriptParser()
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

	// Verify methods
	if len(extracted.Methods) < 3 {
		t.Errorf("expected at least 3 methods, got %d", len(extracted.Methods))
	}
}

func TestTypeScriptParserParseClass(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tsparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tsCode := `import { IUserRepository } from './IUserRepository';
import { ILogger } from './ILogger';

/**
 * Service for managing users
 */
export class UserService {
  private readonly repository: IUserRepository;
  private readonly logger: ILogger;

  /** Whether the service is active */
  public isActive: boolean;

  /**
   * Creates a new UserService
   * @param repository The user repository
   * @param logger The logger
   */
  constructor(
    repository: IUserRepository,
    logger: ILogger
  ) {
    this.repository = repository;
    this.logger = logger;
    this.isActive = true;
  }

  /**
   * Gets a user by ID
   * @param id The user ID
   * @returns The user or null
   */
  public async getUser(id: string): Promise<User | null> {
    return this.repository.findById(id);
  }

  /**
   * Creates a new user
   */
  public async createUser(name: string, email: string): Promise<User> {
    const user = { name, email };
    await this.repository.save(user);
    return user;
  }
}
`
	filePath := filepath.Join(tempDir, "UserService.ts")
	if err := os.WriteFile(filePath, []byte(tsCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewTypeScriptParser()
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

	// Verify dependencies from constructor
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

func TestTypeScriptParserParseExtendsImplements(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tsparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tsCode := `export class UserService extends BaseService implements IUserService, IDisposable {
  constructor() {
    super();
  }
}
`
	filePath := filepath.Join(tempDir, "ExtendedService.ts")
	if err := os.WriteFile(filePath, []byte(tsCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewTypeScriptParser()
	extracted, err := p.ParseFile(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	// Verify class
	if extracted.ID != "UserService" {
		t.Errorf("expected ID 'UserService', got '%s'", extracted.ID)
	}

	// Verify dependencies (extends + implements)
	depMap := make(map[string]string)
	for _, dep := range extracted.Dependencies {
		depMap[dep.Target] = dep.Injection
	}

	if _, ok := depMap["BaseService"]; !ok {
		t.Error("expected extends dependency on BaseService")
	}
	if _, ok := depMap["IUserService"]; !ok {
		t.Error("expected implements dependency on IUserService")
	}
}

func TestTypeScriptParserParseFileNotExists(t *testing.T) {
	p := NewTypeScriptParser()
	_, err := p.ParseFile("/nonexistent/path/file.ts")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestTypeScriptParserParseArrowFunctions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tsparser_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tsCode := `export class HandlerClass {
  public handleClick = (event: MouseEvent): void => {
    console.log(event);
  };

  private processData = async (data: string): Promise<void> => {
    // process
  };
}
`
	filePath := filepath.Join(tempDir, "Handler.ts")
	if err := os.WriteFile(filePath, []byte(tsCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := NewTypeScriptParser()
	extracted, err := p.ParseFile(filePath)
	if err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}

	// Verify arrow function methods were extracted
	if len(extracted.Methods) < 1 {
		t.Errorf("expected at least 1 arrow method, got %d", len(extracted.Methods))
	}
}
