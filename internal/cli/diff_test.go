package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/parser"
)

func TestResolveDiffNodeSpecAcceptsCanonicalID(t *testing.T) {
	nodesDir := t.TempDir()
	spec := &node.Spec{
		Node: node.NodeInfo{ID: "runQuery", Namespace: "cli", Type: "function", FilePath: "query.go"},
	}
	if err := node.Save(filepath.Join(nodesDir, "runQuery.yaml"), spec); err != nil {
		t.Fatalf("save node: %v", err)
	}

	resolved, canonical, err := resolveDiffNodeSpec(nodesDir, "cli.runQuery")
	if err != nil {
		t.Fatalf("resolve canonical diff node: %v", err)
	}
	if resolved.Node.ID != "runQuery" || canonical != "cli.runQuery" {
		t.Fatalf("unexpected resolution: spec=%+v canonical=%q", resolved.Node, canonical)
	}
}

func TestResolveDiffNodeSpecRejectsAmbiguousShortID(t *testing.T) {
	nodesDir := t.TempDir()
	for fileName, namespace := range map[string]string{"one.yaml": "one", "two.yaml": "two"} {
		spec := &node.Spec{Node: node.NodeInfo{ID: "Widget", Namespace: namespace, Type: "class", FilePath: namespace + ".go"}}
		if err := node.Save(filepath.Join(nodesDir, fileName), spec); err != nil {
			t.Fatalf("save %s: %v", fileName, err)
		}
	}

	_, _, err := resolveDiffNodeSpec(nodesDir, "Widget")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected explicit ambiguity error, got %v", err)
	}
}

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

func TestFindExtractedNodeInFileMatchesTerminalSymbolForDottedIDs(t *testing.T) {
	parser := fakeMultiNodeParser{
		nodes: []*parser.ExtractedNode{
			{ID: "Runtime"},
			{ID: "Other"},
		},
	}

	match, err := findExtractedNodeInFile(parser, "ignored.go", "tools.Runtime")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if match == nil || match.ID != "Runtime" {
		t.Fatalf("expected Runtime match, got %+v", match)
	}
}

func TestFindExtractedNodeInFilePrefersExactIDBeforeTerminalFallback(t *testing.T) {
	parser := fakeMultiNodeParser{
		nodes: []*parser.ExtractedNode{
			{ID: "Runtime"},
			{ID: "tools.Runtime"},
		},
	}

	match, err := findExtractedNodeInFile(parser, "ignored.go", "tools.Runtime")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if match == nil || match.ID != "tools.Runtime" {
		t.Fatalf("expected exact tools.Runtime match, got %+v", match)
	}
}

type fakeMultiNodeParser struct {
	nodes []*parser.ExtractedNode
	err   error
}

func (f fakeMultiNodeParser) ParseFile(path string) (*parser.ExtractedNode, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.nodes) == 0 {
		return nil, nil
	}
	return f.nodes[0], nil
}

func (f fakeMultiNodeParser) ParseFileNodes(path string) ([]*parser.ExtractedNode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.nodes, nil
}

func (f fakeMultiNodeParser) Language() string {
	return "go"
}
