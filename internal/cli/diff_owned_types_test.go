package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/parser"
)

func writeGoFile(t *testing.T, name, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDriftOwnedTypeIsNotAnExtraDependency(t *testing.T) {
	path := writeGoFile(t, "parser.go", "package calc\n\ntype Node struct{ Value int }\n\ntype Parser struct{ lexer Lexer }\n")
	spec := &node.Spec{
		Interface: node.Interface{
			Types:   []node.TypeContract{{Name: "Node", Signature: "type Node struct { Value int }"}},
			Methods: []node.Method{{Name: "Parse", Signature: "Parse(input string) (*Node, error)"}},
		},
		Dependencies: []node.Dependency{{Target: "Lexer"}},
	}
	extracted := &parser.ExtractedNode{
		ID: "Parser", Language: "go", FilePath: path,
		Methods:      []parser.ExtractedMethod{{Name: "Parse", Signature: "Parse(input string) (*Node, error)"}},
		Dependencies: []parser.ExtractedDependency{{Target: "Lexer"}, {Target: "Node"}},
	}
	report := buildDriftReport(spec, extracted)
	if len(report.ExtraDeps) != 0 || len(report.MissingDeps) != 0 || len(report.MethodMismatches) != 0 {
		t.Fatalf("owned type reported as drift: %+v", report)
	}
}

func TestDriftUndeclaredCollaboratorStillDrifts(t *testing.T) {
	path := writeGoFile(t, "parser.go", "package calc\n\ntype Node struct{}\n")
	spec := &node.Spec{Interface: node.Interface{Types: []node.TypeContract{{Name: "Node"}}}}
	extracted := &parser.ExtractedNode{
		ID: "Parser", Language: "go", FilePath: path,
		Dependencies: []parser.ExtractedDependency{{Target: "Node"}, {Target: "Lexer"}},
	}
	report := buildDriftReport(spec, extracted)
	if len(report.ExtraDeps) != 1 || report.ExtraDeps[0] != "Lexer" {
		t.Fatalf("undeclared collaborator must remain extra drift: %+v", report.ExtraDeps)
	}
}

// A spec cannot hide a collaborator by listing the collaborator's type as its
// own: the type is declared in another file, so it stays an extra dependency.
func TestDriftCollaboratorListedAsOwnedTypeStillDrifts(t *testing.T) {
	path := writeGoFile(t, "compose.go", "package portrait\n\ntype Composer struct{ registry Registry }\n")
	spec := &node.Spec{Interface: node.Interface{Types: []node.TypeContract{{Name: "Registry"}}}}
	extracted := &parser.ExtractedNode{
		ID: "Composer", Language: "go", FilePath: path,
		Dependencies: []parser.ExtractedDependency{{Target: "Registry"}},
	}
	report := buildDriftReport(spec, extracted)
	if len(report.ExtraDeps) != 1 || report.ExtraDeps[0] != "Registry" {
		t.Fatalf("collaborator hidden by interface.types: %+v", report.ExtraDeps)
	}
}

func TestDriftOwnedTypeNeedsReadableGoSource(t *testing.T) {
	spec := &node.Spec{Interface: node.Interface{Types: []node.TypeContract{{Name: "Node"}}}}
	for _, extracted := range []*parser.ExtractedNode{
		{ID: "Parser", Language: "go", FilePath: filepath.Join(t.TempDir(), "missing.go"), Dependencies: []parser.ExtractedDependency{{Target: "Node"}}},
		{ID: "Parser", Language: "typescript", FilePath: "parser.ts", Dependencies: []parser.ExtractedDependency{{Target: "Node"}}},
	} {
		if report := buildDriftReport(spec, extracted); len(report.ExtraDeps) != 1 {
			t.Fatalf("ownership without Go source evidence was trusted: %+v", report.ExtraDeps)
		}
	}
}
