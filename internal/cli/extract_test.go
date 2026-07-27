package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/config"
	extractctx "github.com/gdc-tools/gdc/internal/extract"
	"github.com/gdc-tools/gdc/internal/node"
)

func TestCollectExtractEvidenceLoadsRequestedArtifacts(t *testing.T) {
	projectRoot := t.TempDir()
	sourcePath := filepath.Join(projectRoot, "service.go")
	testPath := filepath.Join(projectRoot, "service_test.go")

	if err := os.WriteFile(sourcePath, []byte(`package sample

func Service() {}

func UseService() {
	Service()
}
`), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}
	if err := os.WriteFile(testPath, []byte(`package sample

import "testing"

func TestService(t *testing.T) {
	Service()
}
`), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	prevImpl := extractWithImpl
	prevTests := extractWithTests
	prevCallers := extractWithCallers
	t.Cleanup(func() {
		extractWithImpl = prevImpl
		extractWithTests = prevTests
		extractWithCallers = prevCallers
	})
	extractWithImpl = true
	extractWithTests = true
	extractWithCallers = true

	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:       "Service",
			Type:     "function",
			FilePath: sourcePath,
		},
		Interface: node.Interface{
			Methods: []node.Method{
				{Name: "Service", Signature: "Service()"},
			},
		},
		Metadata: node.Metadata{Status: "implemented"},
	}
	cfg := &config.Config{ProjectRoot: projectRoot}

	evidence, err := collectExtractEvidence(context.Background(), spec, cfg)
	if err != nil {
		t.Fatalf("failed to collect evidence: %v", err)
	}

	if evidence.Implementation == nil || evidence.Implementation.PrimaryFile == nil {
		t.Fatal("expected implementation evidence to be loaded")
	}
	if len(evidence.Tests) == 0 {
		t.Fatal("expected test evidence to be loaded")
	}
	if len(evidence.Callers) == 0 {
		t.Fatal("expected caller evidence to be loaded")
	}
	if len(evidence.References) == 0 {
		t.Fatal("expected reference evidence to be loaded")
	}
	if len(evidence.Warnings) == 0 {
		t.Fatal("expected caller fallback warning to be included")
	}
}

func TestResolveExtractNodeSpecPreservesFileStemAndAcceptsKebabCaseID(t *testing.T) {
	nodesDir := t.TempDir()
	spec := &node.Spec{
		Node: node.NodeInfo{ID: "DeterministicPropertyGraphAdapter", Type: "class", FilePath: "adapter.cs"},
	}
	if err := node.Save(filepath.Join(nodesDir, "deterministic-property-adapter.yaml"), spec); err != nil {
		t.Fatalf("save node: %v", err)
	}

	allNodes, err := loadAllNodes(nodesDir)
	if err != nil {
		t.Fatalf("load nodes: %v", err)
	}
	lookup := buildSpecLookup(allNodes)

	for _, input := range []string{"deterministic-property-adapter", "DeterministicPropertyGraphAdapter", "deterministic-property-graph-adapter"} {
		resolved, resolveErr := resolveExtractNodeSpec(nodesDir, input, lookup)
		if resolveErr != nil {
			t.Fatalf("resolve %q: %v", input, resolveErr)
		}
		if resolved.Node.ID != "DeterministicPropertyGraphAdapter" {
			t.Fatalf("resolve %q returned %q", input, resolved.Node.ID)
		}
	}
}

func TestGeneratePromptIncludesEvidenceSections(t *testing.T) {
	spec := &node.Spec{
		Node:           node.NodeInfo{ID: "Service", Type: "function", Layer: "application"},
		Responsibility: node.Responsibility{Summary: "Provide service behavior."},
		Interface: node.Interface{
			Methods: []node.Method{
				{Name: "Service", Signature: "Service()", Description: "Run the service."},
			},
		},
		Metadata: node.Metadata{Status: "implemented"},
	}
	cfg := &config.Config{}
	cfg.Project.Language = "go"

	evidence := extractEvidence{
		Warnings: []string{"Caller evidence collected via code search fallback."},
	}
	evidence.Implementation = &extractctx.CodeLoadResult{
		Language: "go",
		PrimaryFile: &extractctx.SourceFile{
			Path:     "service.go",
			Content:  "func Service() {}",
			Language: "go",
		},
	}
	evidence.Tests = []*extractctx.TestFileContent{
		{
			TestFile: &extractctx.TestFile{
				Path:      "service_test.go",
				Name:      "service_test.go",
				Framework: "go test",
			},
			Content: "func TestService(t *testing.T) {}",
			Lines:   1,
		},
	}
	evidence.Callers = []*extractctx.CallerInfo{
		{File: "main.go", Line: 10, Function: "UseService", CallSnippet: "Service()"},
	}
	evidence.References = []*extractctx.ReferenceInfo{
		{File: "main.go", Line: 10, Type: "type_reference", Snippet: "Service()"},
	}

	prompt, err := generatePrompt(spec, nil, cfg, false, evidence, false)
	if err != nil {
		t.Fatalf("failed to generate prompt: %v", err)
	}

	expectedSnippets := []string{
		"## Implementation Code Evidence",
		"## Test Code Evidence",
		"## Usage Evidence (Callers)",
		"## Usage Evidence (References)",
		"## Warnings",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("expected prompt to include %q", snippet)
		}
	}
}

func TestValidateImplementationExtractOptionsRejectsSourceEvidence(t *testing.T) {
	previousImpl := extractWithImpl
	previousTests := extractWithTests
	previousCallers := extractWithCallers
	t.Cleanup(func() {
		extractWithImpl = previousImpl
		extractWithTests = previousTests
		extractWithCallers = previousCallers
	})

	for _, setup := range []func(){
		func() { extractWithImpl = true },
		func() { extractWithTests = true },
		func() { extractWithCallers = true },
	} {
		extractWithImpl, extractWithTests, extractWithCallers = false, false, false
		setup()
		if err := validateImplementationExtractOptions(); err == nil || !strings.Contains(err.Error(), "source-free") {
			t.Fatalf("expected source-free incompatibility error, got %v", err)
		}
	}
}

func TestGeneratePromptIncludesCompleteSourceFreeContractClosure(t *testing.T) {
	dependency := readyImplementationSpec("Validator", "Validate")
	target := readyImplementationSpec("Worker", "Execute")
	target.Dependencies = []node.Dependency{{
		Target:       dependency.Node.ID,
		Type:         "interface",
		Injection:    "constructor",
		Usage:        "Validate each request.",
		ContractHash: calculateSpecHash(dependency),
		Requires:     []string{"Validate"},
	}}
	nodeMap := buildSpecLookup([]*node.Spec{target, dependency})
	deps := gatherImplementationDependencies(target, nodeMap, "go")
	cfg := &config.Config{}
	cfg.Project.Language = "go"

	prompt, err := generatePrompt(target, deps, cfg, true, extractEvidence{}, true)
	if err != nil {
		t.Fatalf("generate implementation prompt: %v", err)
	}
	for _, expected := range []string{
		"Implementation Ready: true",
		"Source Free: true",
		"WORKER-SUCCESS",
		"Validator",
		"Required Members: Validate",
		"output is canonical",
		"No external implementation source is required",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("implementation prompt omitted %q:\n%s", expected, prompt)
		}
	}
	if strings.Contains(prompt, "Implementation Code Evidence") || strings.Contains(prompt, "Test Code Evidence") {
		t.Fatalf("source-free packet contained repository evidence:\n%s", prompt)
	}
}

func TestBuildDependenciesJSONPreservesFullBehaviorContract(t *testing.T) {
	dependency := readyImplementationSpec("Validator", "Validate")
	dependency.Interface.Types = []node.TypeContract{{
		Name: "ValidationResult", Signature: "type ValidationResult struct", Description: "Immutable validation result.",
		Fields: []node.ContractField{{Name: "Valid", Type: "bool", Description: "Whether validation succeeded."}},
	}}
	deps := []DependencyInfo{{
		Target:       dependency.Node.ID,
		ContractHash: calculateSpecHash(dependency),
		Requires:     []string{"Validate"},
		Spec:         dependency,
	}}

	result := buildDependenciesJSON(deps)
	if len(result) != 1 || result[0].Spec == nil {
		t.Fatalf("dependency contract missing from JSON: %+v", result)
	}
	method := result[0].Spec.Interface.Methods[0]
	if len(method.Postconditions) != 1 || method.Postconditions[0] != "output is canonical" {
		t.Fatalf("dependency behavior was reduced or lost: %+v", method)
	}
	if len(result[0].Spec.Interface.Types) != 1 || result[0].Spec.Interface.Types[0].Fields[0].Name != "Valid" {
		t.Fatalf("dependency type contracts were lost: %+v", result[0].Spec.Interface.Types)
	}
	if result[0].ContractHash == "" || len(result[0].Requires) != 1 {
		t.Fatalf("dependency edge contract was lost: %+v", result[0])
	}
}

func TestBuildExtractResultJSONMarksSourceFreePacketAndPreservesLiteralContracts(t *testing.T) {
	dependency := readyImplementationSpec("Validator", "Validate")
	target := readyImplementationSpec("Worker", "Execute")
	deps := []DependencyInfo{{
		Target:       dependency.Node.ID,
		ContractHash: calculateSpecHash(dependency),
		Requires:     []string{"Validate"},
		Spec:         dependency,
	}}

	result := buildExtractResultJSON(target, deps, extractEvidence{}, true)
	if !result.ImplementationReady || !result.SourceFree {
		t.Fatalf("implementation packet was not marked ready and source-free: %+v", result)
	}
	if result.Implementation != nil || len(result.Tests) != 0 || len(result.Callers) != 0 || len(result.References) != 0 {
		t.Fatalf("source-free packet contained repository evidence: %+v", result)
	}
	if _, ok := result.Contract["implementation_contract"]; !ok {
		t.Fatalf("target literal contract missing implementation metadata: %+v", result.Contract)
	}
	if len(result.Dependencies) != 1 || result.Dependencies[0].Spec == nil {
		t.Fatalf("dependency closure missing: %+v", result.Dependencies)
	}
	if _, ok := result.Dependencies[0].Spec.Contract["implementation_contract"]; !ok {
		t.Fatalf("dependency literal contract missing implementation metadata: %+v", result.Dependencies[0].Spec.Contract)
	}
}

func TestRunExtractForImplementationUsesOnlyGDCContracts(t *testing.T) {
	projectRoot := t.TempDir()
	nodesDir := filepath.Join(projectRoot, ".gdc", "nodes")
	if err := os.MkdirAll(nodesDir, 0o755); err != nil {
		t.Fatalf("create nodes directory: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Project.Name = "source-free-fixture"
	cfg.Project.Language = "go"
	if err := config.Save(filepath.Join(projectRoot, ".gdc", "config.yaml"), cfg); err != nil {
		t.Fatalf("save fixture config: %v", err)
	}

	dependency := readyImplementationSpec("Validator", "Validate")
	target := readyImplementationSpec("Worker", "Execute")
	target.Dependencies = []node.Dependency{{
		Target:       dependency.Node.ID,
		Type:         "interface",
		Injection:    "constructor",
		Usage:        "Validate each request.",
		ContractHash: calculateSpecHash(dependency),
		Requires:     []string{"Validate"},
	}}
	if err := node.Save(filepath.Join(nodesDir, "Validator.yaml"), dependency); err != nil {
		t.Fatalf("save dependency contract: %v", err)
	}
	if err := node.Save(filepath.Join(nodesDir, "Worker.yaml"), target); err != nil {
		t.Fatalf("save target contract: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("enter fixture project: %v", err)
	}
	previousOutput := extractOutput
	previousFormat := extractFormat
	previousForImplementation := extractForImplementation
	previousImpl := extractWithImpl
	previousTests := extractWithTests
	previousCallers := extractWithCallers
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
		extractOutput = previousOutput
		extractFormat = previousFormat
		extractForImplementation = previousForImplementation
		extractWithImpl = previousImpl
		extractWithTests = previousTests
		extractWithCallers = previousCallers
	})

	packetPath := filepath.Join(projectRoot, "packet.md")
	extractOutput = packetPath
	extractFormat = "text"
	extractForImplementation = true
	extractWithImpl, extractWithTests, extractWithCallers = false, false, false
	if err := runExtract(nil, []string{"Worker"}); err != nil {
		t.Fatalf("source-free implementation extract failed: %v", err)
	}
	packet, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatalf("read implementation packet: %v", err)
	}
	if !strings.Contains(string(packet), "Implementation Ready: true") || !strings.Contains(string(packet), "Validator") {
		t.Fatalf("implementation packet omitted ready closure:\n%s", packet)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "worker.go")); !os.IsNotExist(err) {
		t.Fatalf("fixture unexpectedly required or created implementation source: %v", err)
	}

	target.Interface.Methods[0].Description = "REQUIRES_APPROVAL"
	if err := node.Save(filepath.Join(nodesDir, "Worker.yaml"), target); err != nil {
		t.Fatalf("save incomplete target contract: %v", err)
	}
	if err := runExtract(nil, []string{"Worker"}); err == nil || !strings.Contains(err.Error(), "unresolved placeholder") {
		t.Fatalf("incomplete source-free contract was not rejected: %v", err)
	}
}
