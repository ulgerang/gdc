package cli

import (
	"testing"

	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/parser"
)

func TestDriftOwnedTypeIsNotAnExtraDependency(t *testing.T) {
	spec := &node.Spec{
		Interface: node.Interface{
			Types:   []node.TypeContract{{Name: "Node", Signature: "type Node struct { Value int }"}},
			Methods: []node.Method{{Name: "Parse", Signature: "Parse(input string) (*Node, error)"}},
		},
		Dependencies: []node.Dependency{{Target: "Lexer"}},
	}
	extracted := &parser.ExtractedNode{
		Methods:      []parser.ExtractedMethod{{Name: "Parse", Signature: "Parse(input string) (*Node, error)"}},
		Dependencies: []parser.ExtractedDependency{{Target: "Lexer"}, {Target: "Node"}},
	}
	report := buildDriftReport(spec, extracted)
	if len(report.ExtraDeps) != 0 || len(report.MissingDeps) != 0 || len(report.MethodMismatches) != 0 {
		t.Fatalf("owned type reported as drift: %+v", report)
	}
}

func TestDriftUndeclaredCollaboratorStillDrifts(t *testing.T) {
	spec := &node.Spec{
		Interface: node.Interface{Types: []node.TypeContract{{Name: "Node"}}},
	}
	extracted := &parser.ExtractedNode{
		Dependencies: []parser.ExtractedDependency{{Target: "Node"}, {Target: "Lexer"}},
	}
	report := buildDriftReport(spec, extracted)
	if len(report.ExtraDeps) != 1 || report.ExtraDeps[0] != "Lexer" {
		t.Fatalf("undeclared collaborator must remain extra drift: %+v", report.ExtraDeps)
	}
}
