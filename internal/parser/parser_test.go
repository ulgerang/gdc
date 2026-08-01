package parser

import (
	"os"
	"testing"

	"github.com/gdc-tools/gdc/internal/node"
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
		{"rust", false, "rust"},
		{"rs", false, "rust"},
		{"python", false, "python"},
		{"py", false, "python"},
		{"gdscript", false, "gdscript"},
		{"gd", false, "gdscript"},
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

func TestExtractedFunctionNodePreservesItsOwnContract(t *testing.T) {
	extracted := &ExtractedNode{
		ID:   "localHelper",
		Type: "function",
		Methods: []ExtractedMethod{{
			Name:      "localHelper",
			Signature: "localHelper(value string) string",
			IsPublic:  false,
		}},
	}

	spec := extracted.ToNodeSpec(nil)
	if len(spec.Interface.Methods) != 1 || spec.Interface.Methods[0].Name != "localHelper" {
		t.Fatalf("top-level function node must preserve its own contract, got %#v", spec.Interface.Methods)
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

func TestExtractedNodeMergePreservesAuthoredContractAndRefreshesCodeShape(t *testing.T) {
	oldSpec := &node.Spec{
		SchemaVersion: "1.1",
		Node: node.NodeInfo{
			ID:        "AuthService",
			Type:      "interface",
			Layer:     "application",
			Namespace: "httpapi",
			FilePath:  "internal/httpapi/auth_handler.go",
		},
		Responsibility: node.Responsibility{
			Summary:    "Bound authentication use cases.",
			Invariants: []string{"Provider outages never block guest continuity."},
		},
		Interface: node.Interface{
			Types: []node.TypeContract{{
				Name:        "AuthOutcome",
				Signature:   "type AuthOutcome string",
				Description: "Stable authentication outcome.",
			}},
			Methods: []node.Method{{
				Name:        "LoginGuest",
				Signature:   "LoginGuest(ctx context.Context, playerID string, restoreToken string) (auth.LoginResult, error)",
				Description: "Creates or resumes a guest session.",
				Parameters: []node.Parameter{
					{Name: "ctx", Type: "context.Context", Description: "Request context."},
					{Name: "playerID", Type: "string", Description: "Existing player ID.", Constraint: "opaque"},
					{Name: "restoreToken", Type: "string", Description: "Obsolete restore input."},
				},
				Returns:        node.Returns{Type: "(auth.LoginResult, error)", Description: "Issued credentials.", Constraint: "credentials are never logged"},
				Throws:         []node.Throws{{Type: "ErrInvalidCredential", Condition: "unknown player"}},
				Preconditions:  []string{"The request body passed strict decoding."},
				Postconditions: []string{"A successful result has an active player epoch."},
				SideEffects:    []string{"May create a guest player."},
				Exported:       true,
			}},
		},
		Dependencies: []node.Dependency{{
			Target:       "store.Store",
			Type:         "interface",
			Injection:    "field",
			Usage:        "Persists players and refresh credentials.",
			ContractHash: "eadaa4eb",
			Requires:     []string{"GetPlayer", "CreatePlayer", "UpdatePlayerEpoch"},
		}},
		Implementations: []string{"SessionAuthService"},
		ImplementationContract: &node.ImplementationContract{
			Status:      "ready",
			Lifecycle:   []string{"One implementation is injected before route registration."},
			Constraints: []string{"Google availability is optional."},
			Acceptance: []node.AcceptanceScenario{{
				ID: "AUTH-CONTINUITY-001", Given: "An established player.", When: "Google is unavailable.", Then: []string{"Login still succeeds."},
			}},
		},
		Metadata: node.Metadata{Status: "implemented", Origin: "hand_authored", Tags: []string{"auth"}},
	}

	extracted := &ExtractedNode{
		ID:        "AuthService",
		Type:      "interface",
		Namespace: "httpapi",
		Language:  "go",
		Package:   "httpapi",
		FilePath:  "internal/httpapi/auth_handler.go",
		Methods: []ExtractedMethod{{
			Name:      "LoginGuest",
			Signature: "LoginGuest(ctx context.Context, playerID string) (auth.LoginResult, error)",
			Parameters: []ExtractedParameter{
				{Name: "ctx", Type: "context.Context"},
				{Name: "playerID", Type: "string"},
			},
			Returns:  "(auth.LoginResult, error)",
			IsPublic: true,
			Exported: true,
		}},
		Dependencies: []ExtractedDependency{{
			Target: "Store", Type: "interface", Injection: "field",
		}},
	}

	merged := extracted.ToNodeSpec(oldSpec)

	if merged.SchemaVersion != "1.1" {
		t.Fatalf("schema_version = %q, want preserved 1.1", merged.SchemaVersion)
	}
	if merged.ImplementationContract == nil || merged.ImplementationContract.Status != "ready" || len(merged.ImplementationContract.Acceptance) != 1 {
		t.Fatalf("implementation contract was not preserved: %#v", merged.ImplementationContract)
	}
	if len(merged.Interface.Types) != 1 || merged.Interface.Types[0].Name != "AuthOutcome" {
		t.Fatalf("authored interface types were not preserved: %#v", merged.Interface.Types)
	}
	if len(merged.Implementations) != 1 || merged.Implementations[0] != "SessionAuthService" {
		t.Fatalf("authored implementations were not preserved: %#v", merged.Implementations)
	}
	if len(merged.Interface.Methods) != 1 {
		t.Fatalf("methods = %d, want 1", len(merged.Interface.Methods))
	}
	method := merged.Interface.Methods[0]
	if method.Signature != extracted.Methods[0].Signature {
		t.Fatalf("signature = %q, want refreshed %q", method.Signature, extracted.Methods[0].Signature)
	}
	if len(method.Parameters) != 2 {
		t.Fatalf("parameters = %#v, obsolete restoreToken must be removed", method.Parameters)
	}
	if method.Parameters[1].Name != "playerID" || method.Parameters[1].Description != "Existing player ID." || method.Parameters[1].Constraint != "opaque" {
		t.Fatalf("playerID metadata was not preserved on refreshed parameter: %#v", method.Parameters[1])
	}
	if method.Description == "" || len(method.Preconditions) != 1 || len(method.Postconditions) != 1 || len(method.SideEffects) != 1 || method.Returns.Description == "" {
		t.Fatalf("authored method behavior was not preserved: %#v", method)
	}
	if len(merged.Dependencies) != 1 || len(merged.Dependencies[0].Requires) != 3 || merged.Dependencies[0].ContractHash != "eadaa4eb" {
		t.Fatalf("dependency contract metadata was not preserved: %#v", merged.Dependencies)
	}
}

func cleanupTempDir(t *testing.T, path string) {
	t.Helper()
	t.Cleanup(func() {
		if err := os.RemoveAll(path); err != nil {
			t.Errorf("failed to remove temporary directory %s: %v", path, err)
		}
	})
}
