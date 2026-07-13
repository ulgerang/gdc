package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPythonParserExtractsPublicNodesAndClassContracts(t *testing.T) {
	fixture := filepath.Join("..", "..", "fixtures", "p1", "sample.py")
	p := NewPythonParser()

	nodes, err := p.ParseFileNodes(fixture)
	if err != nil {
		t.Fatalf("ParseFileNodes() error = %v", err)
	}
	if len(nodes) != 4 {
		t.Fatalf("expected 4 public Python nodes, got %d", len(nodes))
	}

	repository := findPythonTestNode(nodes, "UserRepository")
	if repository == nil || repository.Type != "interface" || len(repository.Methods) != 1 {
		t.Fatalf("expected UserRepository interface with one method, got %#v", repository)
	}

	audit := findPythonTestNode(nodes, "AuditSink")
	if audit == nil || audit.Type != "interface" {
		t.Fatalf("expected ABC class to be extracted as interface, got %#v", audit)
	}

	service := findPythonTestNode(nodes, "UserService")
	if service == nil {
		t.Fatal("expected UserService node")
	}
	if len(service.Constructors) != 1 {
		t.Fatalf("expected one constructor, got %d", len(service.Constructors))
	}
	if len(service.Methods) != 1 || service.Methods[0].Name != "get_user" || !service.Methods[0].Async {
		t.Fatalf("expected one public async method, got %#v", service.Methods)
	}
	if len(service.Properties) != 2 {
		t.Fatalf("expected annotated field and @property, got %#v", service.Properties)
	}
	if hasPythonTestMethod(service.Methods, "_reset_cache") {
		t.Fatal("private method must not be extracted")
	}
	for _, dependency := range []string{"UserRepository", "AuditSink"} {
		if !hasPythonTestDependency(service.Dependencies, dependency) {
			t.Fatalf("expected dependency %s in %#v", dependency, service.Dependencies)
		}
	}

	function := findPythonTestNode(nodes, "build_user_service")
	if function == nil || function.Type != "function" || len(function.Methods) != 1 || !function.Methods[0].Async {
		t.Fatalf("expected public async module function node, got %#v", function)
	}
}

func TestPythonParserHandlesDecoratorsAndMultilineTypeHints(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "handlers.py")
	source := `class Handler:
    @classmethod
    async def create(
        cls,
        repository: "Repository",
        callbacks: list[Callback],
    ) -> "Handler":
        return cls()

    @staticmethod
    def validate(value: str) -> bool:
        return bool(value)
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	nodes, err := NewPythonParser().ParseFileNodes(path)
	if err != nil {
		t.Fatal(err)
	}
	handler := findPythonTestNode(nodes, "Handler")
	if handler == nil || len(handler.Methods) != 2 {
		t.Fatalf("expected two methods, got %#v", handler)
	}
	if got := len(handler.Methods[0].Parameters); got != 2 {
		t.Fatalf("classmethod receiver must be omitted; got %d parameters", got)
	}
	if !hasPythonTestDependency(handler.Dependencies, "Repository") || !hasPythonTestDependency(handler.Dependencies, "Callback") {
		t.Fatalf("expected unwrapped type-hint dependencies, got %#v", handler.Dependencies)
	}
}

func findPythonTestNode(nodes []*ExtractedNode, id string) *ExtractedNode {
	for _, node := range nodes {
		if node != nil && node.ID == id {
			return node
		}
	}
	return nil
}

func hasPythonTestMethod(methods []ExtractedMethod, name string) bool {
	for _, method := range methods {
		if method.Name == name {
			return true
		}
	}
	return false
}

func hasPythonTestDependency(dependencies []ExtractedDependency, target string) bool {
	for _, dependency := range dependencies {
		if dependency.Target == target {
			return true
		}
	}
	return false
}
