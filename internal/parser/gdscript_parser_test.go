package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGDScriptParserExtractsGodotScriptContract(t *testing.T) {
	fixture := filepath.Join("..", "..", "fixtures", "p1", "slot_controller.gd")
	node, err := NewGDScriptParser().ParseFile(fixture)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if node.ID != "SlotController" || node.Type != "class" || node.Language != "gdscript" {
		t.Fatalf("unexpected GDScript node identity: %#v", node)
	}
	if len(node.Constructors) != 1 {
		t.Fatalf("expected one constructor, got %#v", node.Constructors)
	}
	if len(node.Methods) != 2 || !hasGDScriptTestMethod(node.Methods, "spin") || !hasGDScriptTestMethod(node.Methods, "create") {
		t.Fatalf("expected public spin and create methods, got %#v", node.Methods)
	}
	if hasGDScriptTestMethod(node.Methods, "_reset_for_test") {
		t.Fatal("private GDScript method must not be extracted")
	}
	if len(node.Properties) != 2 || len(node.Events) != 1 || node.Events[0].Name != "spin_resolved" {
		t.Fatalf("expected two properties and one signal, got properties=%#v events=%#v", node.Properties, node.Events)
	}
	for _, dependency := range []string{"ReelBoard", "SpinResult", "SpinContext"} {
		if !hasGDScriptTestDependency(node.Dependencies, dependency) {
			t.Fatalf("expected dependency %s in %#v", dependency, node.Dependencies)
		}
	}
	if hasGDScriptTestDependency(node.Dependencies, "RefCounted") || hasGDScriptTestDependency(node.Dependencies, "String") {
		t.Fatalf("Godot built-ins must not become dependencies: %#v", node.Dependencies)
	}
}

func TestGDScriptParserUsesFileStemWithoutClassName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "score_service.gd")
	if err := os.WriteFile(path, []byte("extends Node\n\nfunc total() -> int:\n\treturn 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	node, err := NewGDScriptParser().ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if node.ID != "score_service" || len(node.Methods) != 1 || node.Methods[0].Name != "total" {
		t.Fatalf("unexpected filename-backed script node: %#v", node)
	}
}

func hasGDScriptTestMethod(methods []ExtractedMethod, name string) bool {
	for _, method := range methods {
		if method.Name == name {
			return true
		}
	}
	return false
}

func hasGDScriptTestDependency(dependencies []ExtractedDependency, target string) bool {
	for _, dependency := range dependencies {
		if dependency.Target == target {
			return true
		}
	}
	return false
}
