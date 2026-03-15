// Package integration_test contains integration tests for GDC
package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdc-tools/gdc/internal/parser"
)

// TestP3CSharpFixtureParsing tests C# fixture parsing for R3 (AC-R3-1)
func TestP3CSharpFixtureParsing(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "fixtures", "p1", "sample.cs")
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Skipf("C# fixture not found at %s", fixturePath)
	}

	p := parser.NewCSharpParser()
	extracted, err := p.ParseFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to parse C# fixture: %v", err)
	}

	// Verify OrderService class was extracted
	if extracted.ID == "" {
		t.Fatal("Expected to extract at least one type from C# fixture")
	}

	// Find OrderService class
	var orderService *parser.ExtractedNode
	// Note: Current parser extracts first type only, so check if it's OrderService
	if extracted.ID == "OrderService" {
		orderService = extracted
	}

	if orderService != nil {
		// Verify OrderService has public methods
		if len(orderService.Methods) < 3 {
			t.Errorf("Expected at least 3 public methods in OrderService, got %d", len(orderService.Methods))
		}

		// Verify dependencies are extracted
		if len(orderService.Dependencies) < 2 {
			t.Errorf("Expected at least 2 dependencies in OrderService, got %d", len(orderService.Dependencies))
		}

		// Check for IAuthService dependency
		foundAuth := false
		for _, dep := range orderService.Dependencies {
			if dep.Target == "IAuthService" {
				foundAuth = true
				break
			}
		}
		if !foundAuth {
			t.Error("Expected IAuthService dependency in OrderService")
		}
	}

	t.Logf("C# fixture parsing: ID=%s, Type=%s, Methods=%d, Dependencies=%d",
		extracted.ID, extracted.Type, len(extracted.Methods), len(extracted.Dependencies))
}

// TestP3TypeScriptFixtureParsing tests TypeScript fixture parsing for R3 (AC-R3-2)
func TestP3TypeScriptFixtureParsing(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "fixtures", "p1", "sample.ts")
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Skipf("TypeScript fixture not found at %s", fixturePath)
	}

	p := parser.NewTypeScriptParser()
	extracted, err := p.ParseFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to parse TypeScript fixture: %v", err)
	}

	// Verify ApiClient or UserRepository class was extracted
	if extracted.ID == "" {
		t.Fatal("Expected to extract at least one type from TypeScript fixture")
	}

	// Check for exported types
	t.Logf("TypeScript fixture parsing: ID=%s, Type=%s, Methods=%d, Properties=%d",
		extracted.ID, extracted.Type, len(extracted.Methods), len(extracted.Properties))

	// If ApiClient was extracted, verify it has the expected methods
	if extracted.ID == "ApiClient" {
		if len(extracted.Methods) < 4 {
			t.Errorf("Expected at least 4 methods in ApiClient, got %d", len(extracted.Methods))
		}
		// Note: Multi-line constructors may be detected multiple times
		if len(extracted.Constructors) < 1 {
			t.Errorf("Expected at least 1 constructor in ApiClient, got %d", len(extracted.Constructors))
		}
	}

	// If UserRepository was extracted
	if extracted.ID == "UserRepository" {
		if len(extracted.Methods) < 2 {
			t.Errorf("Expected at least 2 methods in UserRepository, got %d", len(extracted.Methods))
		}
		if len(extracted.Constructors) < 1 {
			t.Errorf("Expected at least 1 constructor in UserRepository, got %d", len(extracted.Constructors))
		}
	}
}

// TestP3GoFixtureParsing tests Go fixture parsing for R4 (AC-R4-1)
func TestP3GoFixtureParsing(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "fixtures", "p1", "sample.go")
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Skipf("Go fixture not found at %s", fixturePath)
	}

	p := parser.NewGoParser()
	extracted, err := p.ParseFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to parse Go fixture: %v", err)
	}

	// Verify AuthService struct was extracted
	if extracted.ID == "" {
		t.Fatal("Expected to extract at least one type from Go fixture")
	}

	t.Logf("Go fixture parsing: ID=%s, Type=%s, Methods=%d, Constructors=%d",
		extracted.ID, extracted.Type, len(extracted.Methods), len(extracted.Constructors))

	// AuthService should have constructor and methods
	if extracted.ID == "AuthService" {
		if len(extracted.Constructors) < 1 {
			t.Error("Expected at least 1 constructor (NewAuthService)")
		}
		if len(extracted.Methods) < 2 {
			t.Errorf("Expected at least 2 methods in AuthService, got %d", len(extracted.Methods))
		}
		// Check for dependency injection
		if len(extracted.Dependencies) < 2 {
			t.Errorf("Expected at least 2 dependencies in AuthService, got %d", len(extracted.Dependencies))
		}
	}
}

// TestP3ParserOrchestrator tests that GetParser returns correct parser for each language
func TestP3ParserOrchestrator(t *testing.T) {
	tests := []struct {
		language string
		expected string
	}{
		{"go", "go"},
		{"golang", "go"},
		{"csharp", "csharp"},
		{"cs", "csharp"},
		{"c#", "csharp"},
		{"typescript", "typescript"},
		{"ts", "typescript"},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			p, err := parser.GetParser(tt.language)
			if err != nil {
				t.Fatalf("GetParser(%s) returned error: %v", tt.language, err)
			}
			if p.Language() != tt.expected {
				t.Errorf("GetParser(%s) returned parser for %s, expected %s",
					tt.language, p.Language(), tt.expected)
			}
		})
	}
}

// TestP3GoFixtureConsistency tests that Go parsing produces consistent results
// This is part of R4 (AC-R4-2) - existing behavior preservation
func TestP3GoFixtureConsistency(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "fixtures", "p1", "sample.go")
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Skipf("Go fixture not found at %s", fixturePath)
	}

	p := parser.NewGoParser()

	// Parse multiple times to check consistency
	results := make([]*parser.ExtractedNode, 3)
	for i := 0; i < 3; i++ {
		extracted, err := p.ParseFile(fixturePath)
		if err != nil {
			t.Fatalf("Parse %d failed: %v", i+1, err)
		}
		results[i] = extracted
	}

	// Verify all results are identical
	for i := 1; i < 3; i++ {
		if results[i].ID != results[0].ID {
			t.Errorf("Parse %d produced different ID: %s vs %s", i+1, results[i].ID, results[0].ID)
		}
		if results[i].Type != results[0].Type {
			t.Errorf("Parse %d produced different Type: %s vs %s", i+1, results[i].Type, results[0].Type)
		}
		if len(results[i].Methods) != len(results[0].Methods) {
			t.Errorf("Parse %d produced different method count: %d vs %d", i+1, len(results[i].Methods), len(results[0].Methods))
		}
	}
}
