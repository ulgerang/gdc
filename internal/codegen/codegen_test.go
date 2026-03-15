package codegen

import (
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/node"
)

func TestNewGenerator(t *testing.T) {
	tests := []struct {
		language    string
		expectError bool
	}{
		{"go", false},
		{"golang", false},
		{"csharp", false},
		{"cs", false},
		{"c#", false},
		{"typescript", false},
		{"ts", false},
		{"python", true},
		{"java", true},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			_, err := NewGenerator(tt.language)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for language %s", tt.language)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for language %s: %v", tt.language, err)
				}
			}
		})
	}
}

func TestGeneratorLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"go", "go"},
		{"golang", "go"},
		{"csharp", "csharp"},
		{"cs", "csharp"},
		{"typescript", "typescript"},
		{"ts", "typescript"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			g, _ := NewGenerator(tt.input)
			if g.Language() != tt.expected {
				t.Errorf("expected language '%s', got '%s'", tt.expected, g.Language())
			}
		})
	}
}

func TestAnalyzeSpec(t *testing.T) {
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:   "TestService",
			Type: "class",
		},
		Interface: node.Interface{
			Constructors: []node.Constructor{
				{
					Signature:   "TestService(ILogger logger)",
					Description: "", // Missing description
				},
			},
			Methods: []node.Method{
				{
					Name:        "Process",
					Signature:   "void Process(string input)",
					Description: "Processes the input", // Has description
				},
				{
					Name:        "Validate",
					Signature:   "bool Validate()",
					Description: "", // Missing description
				},
			},
			Properties: []node.Property{
				{
					Name:        "IsEnabled",
					Type:        "bool",
					Description: "Whether enabled", // Has description
				},
				{
					Name:        "Status",
					Type:        "string",
					Description: "", // Missing description
				},
			},
			Events: []node.Event{
				{
					Name:        "OnCompleted",
					Signature:   "event EventHandler OnCompleted",
					Description: "", // Missing description
				},
			},
		},
	}

	info := AnalyzeSpec(spec)

	// Verify basic info
	if info.NodeID != "TestService" {
		t.Errorf("expected NodeID 'TestService', got '%s'", info.NodeID)
	}
	if info.NodeType != "class" {
		t.Errorf("expected NodeType 'class', got '%s'", info.NodeType)
	}

	// Verify members count
	expectedMembers := 1 + 2 + 2 + 1 // 1 ctor + 2 methods + 2 props + 1 event
	if len(info.Members) != expectedMembers {
		t.Errorf("expected %d members, got %d", expectedMembers, len(info.Members))
	}

	// Count members needing description
	needsDescCount := 0
	for _, m := range info.Members {
		if m.NeedsDescription {
			needsDescCount++
		}
	}
	expectedNeedsDesc := 4 // ctor, Validate, Status, OnCompleted
	if needsDescCount != expectedNeedsDesc {
		t.Errorf("expected %d members needing description, got %d", expectedNeedsDesc, needsDescCount)
	}
}

func TestGenerateInterfaceGo(t *testing.T) {
	g, _ := NewGenerator("go")
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:   "UserRepository",
			Type: "interface",
		},
		Interface: node.Interface{
			Methods: []node.Method{
				{
					Name:        "FindByID",
					Signature:   "FindByID(id string) (*User, error)",
					Description: "Finds a user by ID",
				},
				{
					Name:        "Save",
					Signature:   "Save(user *User) error",
					Description: "", // Missing
				},
			},
		},
	}

	code := g.GenerateInterface(spec)

	// Verify Go syntax
	if !strings.Contains(code, "type UserRepository interface {") {
		t.Error("expected Go interface declaration")
	}
	if !strings.Contains(code, "// Finds a user by ID") {
		t.Error("expected description comment")
	}
	if !strings.Contains(code, "// ⚠️ [NEEDS DESCRIPTION]") {
		t.Error("expected needs description marker")
	}
}

func TestGenerateInterfaceCSharp(t *testing.T) {
	g, _ := NewGenerator("csharp")
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:   "IUserRepository",
			Type: "interface",
		},
		Interface: node.Interface{
			Methods: []node.Method{
				{
					Name:        "FindById",
					Signature:   "User FindById(string id)",
					Description: "Finds a user by ID",
				},
			},
			Properties: []node.Property{
				{
					Name:        "ConnectionString",
					Type:        "string",
					Access:      "get",
					Description: "", // Missing
				},
			},
		},
	}

	code := g.GenerateInterface(spec)

	// Verify C# syntax
	if !strings.Contains(code, "public interface IUserRepository") {
		t.Error("expected C# interface declaration")
	}
	if !strings.Contains(code, "/// <summary>") {
		t.Error("expected XML doc comment")
	}
	if !strings.Contains(code, "⚠️ [NEEDS DESCRIPTION]") {
		t.Error("expected needs description marker")
	}
}

func TestGenerateInterfaceTypeScript(t *testing.T) {
	g, _ := NewGenerator("typescript")
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:   "IUserService",
			Type: "interface",
		},
		Interface: node.Interface{
			Methods: []node.Method{
				{
					Name:        "getUser",
					Signature:   "getUser(id: string): Promise<User>",
					Description: "Gets a user by ID",
				},
			},
			Properties: []node.Property{
				{
					Name:        "isActive",
					Type:        "boolean",
					Access:      "get",
					Description: "", // Missing
				},
			},
		},
	}

	code := g.GenerateInterface(spec)

	// Verify TypeScript syntax
	if !strings.Contains(code, "export interface IUserService {") {
		t.Error("expected TypeScript interface declaration")
	}
	if !strings.Contains(code, "/** Gets a user by ID */") {
		t.Error("expected JSDoc comment")
	}
	if !strings.Contains(code, "/** ⚠️ [NEEDS DESCRIPTION] */") {
		t.Error("expected needs description marker")
	}
	if !strings.Contains(code, "readonly") {
		t.Error("expected readonly for get-only property")
	}
}

func TestGenerateClass(t *testing.T) {
	g, _ := NewGenerator("csharp")
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:   "UserService",
			Type: "class",
		},
		Interface: node.Interface{
			Constructors: []node.Constructor{
				{
					Signature:   "UserService(IUserRepository repository)",
					Description: "Creates a new UserService",
				},
			},
			Methods: []node.Method{
				{
					Name:        "GetUser",
					Signature:   "User GetUser(string id)",
					Description: "Gets a user",
				},
			},
		},
	}

	code := g.GenerateInterface(spec)

	// Verify class syntax
	if !strings.Contains(code, "public class UserService") {
		t.Error("expected C# class declaration")
	}
}

func TestAnalyzeSpecEmpty(t *testing.T) {
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:   "EmptyClass",
			Type: "class",
		},
		Interface: node.Interface{},
	}

	info := AnalyzeSpec(spec)

	if len(info.Members) != 0 {
		t.Errorf("expected 0 members for empty spec, got %d", len(info.Members))
	}
}
