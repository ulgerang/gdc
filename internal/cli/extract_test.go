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

	prompt, err := generatePrompt(spec, nil, cfg, false, evidence)
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
