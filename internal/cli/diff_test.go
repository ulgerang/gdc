package cli

import (
	"testing"

	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/parser"
)

func TestBuildDriftReportDetectsSignatureAndDependencyChanges(t *testing.T) {
	spec := &node.Spec{
		Node: node.NodeInfo{ID: "Agent", Type: "class"},
		Interface: node.Interface{
			Constructors: []node.Constructor{
				{Signature: "NewAgent(cfg Config) *Agent"},
			},
			Methods: []node.Method{
				{Name: "RegisterTools", Signature: "RegisterTools(tools unknown) error"},
				{Name: "CloneForRun", Signature: "CloneForRun(ctx RunContext) *Agent"},
			},
			Properties: []node.Property{
				{Name: "Garden", Type: "ContextInjector"},
			},
		},
		Dependencies: []node.Dependency{
			{Target: "Config"},
			{Target: "RunContext"},
		},
	}

	extracted := &parser.ExtractedNode{
		ID:   "Agent",
		Type: "class",
		Constructors: []parser.ExtractedConstructor{
			{Signature: "NewAgent(cfg Config, llm LLM) *Agent"},
		},
		Methods: []parser.ExtractedMethod{
			{Name: "RegisterTools", Signature: "RegisterTools(tools []Tool) error"},
			{Name: "Execute", Signature: "Execute() error"},
		},
		Properties: []parser.ExtractedProperty{
			{Name: "Garden", Type: "*garden.Gardener"},
		},
		Dependencies: []parser.ExtractedDependency{
			{Target: "Config"},
			{Target: "LLM"},
		},
	}

	report := buildDriftReport(spec, extracted)

	if len(report.MethodMismatches) != 1 || report.MethodMismatches[0].Name != "RegisterTools" {
		t.Fatalf("expected RegisterTools mismatch, got %+v", report.MethodMismatches)
	}
	if len(report.PropertyMismatches) != 1 || report.PropertyMismatches[0].Name != "Garden" {
		t.Fatalf("expected Garden property mismatch, got %+v", report.PropertyMismatches)
	}
	if len(report.MissingMethods) != 1 || report.MissingMethods[0] != "CloneForRun" {
		t.Fatalf("expected CloneForRun missing, got %+v", report.MissingMethods)
	}
	if len(report.ExtraMethods) != 1 || report.ExtraMethods[0] != "Execute" {
		t.Fatalf("expected Execute extra, got %+v", report.ExtraMethods)
	}
	if len(report.MissingConstructors) != 1 || len(report.ExtraConstructors) != 1 {
		t.Fatalf("expected constructor drift, got missing=%+v extra=%+v", report.MissingConstructors, report.ExtraConstructors)
	}
	if len(report.MissingDeps) != 1 || report.MissingDeps[0] != "RunContext" {
		t.Fatalf("expected RunContext missing dependency, got %+v", report.MissingDeps)
	}
	if len(report.ExtraDeps) != 1 || report.ExtraDeps[0] != "LLM" {
		t.Fatalf("expected LLM extra dependency, got %+v", report.ExtraDeps)
	}
}

func TestBuildDriftReportEmptyWhenSpecMatchesCode(t *testing.T) {
	spec := &node.Spec{
		Node: node.NodeInfo{ID: "Agent", Type: "class"},
		Interface: node.Interface{
			Methods: []node.Method{
				{Name: "Execute", Signature: "Execute() error"},
			},
		},
		Dependencies: []node.Dependency{
			{Target: "Logger"},
		},
	}
	extracted := &parser.ExtractedNode{
		ID:   "Agent",
		Type: "class",
		Methods: []parser.ExtractedMethod{
			{Name: "Execute", Signature: "Execute() error"},
		},
		Dependencies: []parser.ExtractedDependency{
			{Target: "Logger"},
		},
	}

	report := buildDriftReport(spec, extracted)
	if !report.isEmpty() {
		t.Fatalf("expected no drift, got %+v", report)
	}
}
