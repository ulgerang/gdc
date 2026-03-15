package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetParser(t *testing.T) {
	tests := []struct {
		language    string
		expectError bool
		parserType  string
	}{
		{"go", false, "go"},
		{"golang", false, "go"},
		{"csharp", false, "csharp"},
		{"cs", false, "csharp"},
		{"c#", false, "csharp"},
		{"typescript", false, "typescript"},
		{"ts", false, "typescript"},
		{"python", true, ""},
		{"java", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			p, err := GetParser(tt.language)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for language %s, got nil", tt.language)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for language %s: %v", tt.language, err)
				}
				if p == nil {
					t.Errorf("expected parser for language %s, got nil", tt.language)
				}
			}
		})
	}
}

func TestExtractedNodeToNodeSpec(t *testing.T) {
	extracted := &ExtractedNode{
		ID:        "TestService",
		Type:      "class",
		Namespace: "com.example",
		Constructors: []ExtractedConstructor{
			{
				Signature:   "TestService(ILogger logger)",
				Description: "Creates a new TestService",
				Parameters: []ExtractedParameter{
					{Name: "logger", Type: "ILogger"},
				},
			},
		},
		Methods: []ExtractedMethod{
			{
				Name:        "DoSomething",
				Signature:   "void DoSomething(string input)",
				Description: "Does something with input",
				Parameters: []ExtractedParameter{
					{Name: "input", Type: "string"},
				},
				Returns:  "void",
				IsPublic: true,
			},
			{
				Name:      "PrivateMethod",
				Signature: "void PrivateMethod()",
				IsPublic:  false, // Should be excluded
			},
		},
		Properties: []ExtractedProperty{
			{
				Name:        "IsEnabled",
				Type:        "bool",
				Access:      "get; set",
				Description: "Whether the service is enabled",
				IsPublic:    true,
			},
		},
		Dependencies: []ExtractedDependency{
			{
				Target:    "ILogger",
				FieldName: "logger",
				Injection: "constructor",
			},
		},
	}

	spec := extracted.ToNodeSpec(nil)

	// Verify node info
	if spec.Node.ID != "TestService" {
		t.Errorf("expected ID 'TestService', got '%s'", spec.Node.ID)
	}
	if spec.Node.Type != "class" {
		t.Errorf("expected Type 'class', got '%s'", spec.Node.Type)
	}
	if spec.Node.Namespace != "com.example" {
		t.Errorf("expected Namespace 'com.example', got '%s'", spec.Node.Namespace)
	}

	// Verify constructors
	if len(spec.Interface.Constructors) != 1 {
		t.Errorf("expected 1 constructor, got %d", len(spec.Interface.Constructors))
	}

	// Verify methods (private should be excluded)
	if len(spec.Interface.Methods) != 1 {
		t.Errorf("expected 1 public method, got %d", len(spec.Interface.Methods))
	}
	if len(spec.Interface.Methods) > 0 && spec.Interface.Methods[0].Name != "DoSomething" {
		t.Errorf("expected method 'DoSomething', got '%s'", spec.Interface.Methods[0].Name)
	}

	// Verify properties
	if len(spec.Interface.Properties) != 1 {
		t.Errorf("expected 1 property, got %d", len(spec.Interface.Properties))
	}

	// Verify dependencies
	if len(spec.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(spec.Dependencies))
	}
	if len(spec.Dependencies) > 0 && spec.Dependencies[0].Target != "ILogger" {
		t.Errorf("expected dependency target 'ILogger', got '%s'", spec.Dependencies[0].Target)
	}
}

func TestExtractedNodePreservesOldDescriptions(t *testing.T) {
	// Create old spec with descriptions
	oldSpec := &ExtractedNode{
		ID:   "TestService",
		Type: "class",
		Methods: []ExtractedMethod{
			{
				Name:        "DoSomething",
				Signature:   "void DoSomething(string input)",
				Description: "Original description from old spec",
				IsPublic:    true,
			},
		},
	}
	oldNodeSpec := oldSpec.ToNodeSpec(nil)

	// Create new extracted node without description
	extracted := &ExtractedNode{
		ID:   "TestService",
		Type: "class",
		Methods: []ExtractedMethod{
			{
				Name:        "DoSomething",
				Signature:   "void DoSomething(string input)",
				Description: "", // No description from code
				IsPublic:    true,
			},
		},
	}

	// Convert with old spec - should preserve description
	spec := extracted.ToNodeSpec(oldNodeSpec)

	if len(spec.Interface.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(spec.Interface.Methods))
	}
	if spec.Interface.Methods[0].Description != "Original description from old spec" {
		t.Errorf("expected preserved description 'Original description from old spec', got '%s'",
			spec.Interface.Methods[0].Description)
	}
}

// Helper function to create temp files for testing
func createTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return path
}
