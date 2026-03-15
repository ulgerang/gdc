package extract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NamingConventionTestMatcher finds tests using language-aware naming conventions.
type NamingConventionTestMatcher struct {
	projectRoot string
}

// NewNamingConventionTestMatcher creates a new test matcher.
func NewNamingConventionTestMatcher(projectRoot string) *NamingConventionTestMatcher {
	return &NamingConventionTestMatcher{
		projectRoot: projectRoot,
	}
}

// FindTests locates test files for a given node using naming conventions.
func (m *NamingConventionTestMatcher) FindTests(ctx context.Context, spec *NodeSpec) ([]*TestFile, error) {
	var tests []*TestFile

	// Generate search patterns based on language
	patterns := m.generateTestPatterns(spec)

	for _, pattern := range patterns {
		matches, err := m.findFilesByPattern(pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			test := &TestFile{
				Path:        match,
				Name:        filepath.Base(match),
				Framework:   m.detectFramework(match),
				MatchScore:  m.calculateMatchScore(spec.ID, match),
				MatchReason: fmt.Sprintf("Naming convention match: %s", pattern),
			}
			tests = append(tests, test)
		}
	}

	// Remove duplicates
	tests = m.deduplicate(tests)

	// Sort by match score (highest first)
	tests = m.sortByScore(tests)

	return tests, nil
}

// LoadTestContent loads the content of test files.
func (m *NamingConventionTestMatcher) LoadTestContent(ctx context.Context, tests []*TestFile) ([]*TestFileContent, error) {
	var contents []*TestFileContent

	for _, test := range tests {
		content, err := os.ReadFile(test.Path)
		if err != nil {
			continue
		}

		contents = append(contents, &TestFileContent{
			TestFile: test,
			Content:  string(content),
			Lines:    strings.Count(string(content), "\n") + 1,
		})
	}

	return contents, nil
}

// GetTestCoverage returns coverage information if available.
func (m *NamingConventionTestMatcher) GetTestCoverage(ctx context.Context, spec *NodeSpec) (*TestCoverage, error) {
	// Coverage data would typically come from a coverage report file
	// For now, return nil indicating no coverage data available
	return nil, nil
}

// generateTestPatterns generates file search patterns based on node spec.
func (m *NamingConventionTestMatcher) generateTestPatterns(spec *NodeSpec) []string {
	var patterns []string
	lang := detectLanguageFromPath(spec.SourcePath)

	switch lang {
	case "go":
		// Go test patterns
		patterns = append(patterns,
			fmt.Sprintf("*_test.go"),
			fmt.Sprintf("%s_test.go", strings.ToLower(spec.ID)),
		)
		// Check for test files with the node name
		patterns = append(patterns,
			fmt.Sprintf("*%s*_test.go", spec.ID),
		)

	case "csharp":
		// C# test patterns
		patterns = append(patterns,
			fmt.Sprintf("%sTests.cs", spec.ID),
			fmt.Sprintf("%sTest.cs", spec.ID),
			fmt.Sprintf("*%s*Tests.cs", spec.ID),
		)

	case "typescript", "javascript":
		// TypeScript/JavaScript patterns
		patterns = append(patterns,
			fmt.Sprintf("%s.spec.ts", spec.ID),
			fmt.Sprintf("%s.test.ts", spec.ID),
			fmt.Sprintf("%s.spec.js", spec.ID),
			fmt.Sprintf("%s.test.js", spec.ID),
			fmt.Sprintf("*%s*.spec.ts", spec.ID),
		)

	case "python":
		// Python test patterns
		patterns = append(patterns,
			fmt.Sprintf("test_%s.py", strings.ToLower(spec.ID)),
			fmt.Sprintf("%s_test.py", strings.ToLower(spec.ID)),
			fmt.Sprintf("*%s*_test.py", spec.ID),
		)

	case "java":
		// Java test patterns
		patterns = append(patterns,
			fmt.Sprintf("%sTest.java", spec.ID),
			fmt.Sprintf("%sTests.java", spec.ID),
			fmt.Sprintf("Test%s.java", spec.ID),
		)

	default:
		// Generic patterns
		patterns = append(patterns,
			fmt.Sprintf("*%s*test*", spec.ID),
			fmt.Sprintf("*%s*spec*", spec.ID),
		)
	}

	return patterns
}

// findFilesByPattern finds files matching a pattern.
func (m *NamingConventionTestMatcher) findFilesByPattern(pattern string) ([]string, error) {
	var matches []string

	// Common test directories to search
	testDirs := []string{
		m.projectRoot,
		filepath.Join(m.projectRoot, "tests"),
		filepath.Join(m.projectRoot, "test"),
		filepath.Join(m.projectRoot, "__tests__"),
		filepath.Join(m.projectRoot, "spec"),
		filepath.Join(m.projectRoot, "internal", "tests"),
	}

	for _, dir := range testDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		searchPath := filepath.Join(dir, pattern)
		found, err := filepath.Glob(searchPath)
		if err != nil {
			continue
		}
		matches = append(matches, found...)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no files found for pattern: %s", pattern)
	}

	return matches, nil
}

// detectFramework detects the test framework from file content/path.
func (m *NamingConventionTestMatcher) detectFramework(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}

	contentStr := string(content)
	lowerPath := strings.ToLower(path)

	// Go
	if strings.Contains(lowerPath, "_test.go") {
		if strings.Contains(contentStr, "testing") {
			return "go test"
		}
		if strings.Contains(contentStr, "ginkgo") || strings.Contains(contentStr, "Ginkgo") {
			return "ginkgo"
		}
		if strings.Contains(contentStr, "testify") {
			return "testify"
		}
		return "go test"
	}

	// TypeScript/JavaScript
	if strings.Contains(lowerPath, ".spec.ts") || strings.Contains(lowerPath, ".test.ts") ||
		strings.Contains(lowerPath, ".spec.js") || strings.Contains(lowerPath, ".test.js") {
		if strings.Contains(contentStr, "describe") {
			if strings.Contains(contentStr, "jest") {
				return "jest"
			}
			if strings.Contains(contentStr, "mocha") {
				return "mocha"
			}
			if strings.Contains(contentStr, "vitest") {
				return "vitest"
			}
			return "jest" // default assumption
		}
	}

	// C#
	if strings.Contains(lowerPath, ".cs") {
		if strings.Contains(contentStr, "[Test]") || strings.Contains(contentStr, "[Fact]") {
			if strings.Contains(contentStr, "Xunit") || strings.Contains(contentStr, "xunit") {
				return "xUnit"
			}
			if strings.Contains(contentStr, "NUnit") || strings.Contains(contentStr, "nunit") {
				return "NUnit"
			}
			if strings.Contains(contentStr, "TestClass") {
				return "MSTest"
			}
		}
	}

	// Python
	if strings.Contains(lowerPath, ".py") {
		if strings.Contains(contentStr, "pytest") || strings.Contains(contentStr, "import pytest") {
			return "pytest"
		}
		if strings.Contains(contentStr, "unittest") {
			return "unittest"
		}
	}

	return "unknown"
}

// calculateMatchScore calculates relevance score for a test file.
func (m *NamingConventionTestMatcher) calculateMatchScore(nodeID, path string) float64 {
	score := 0.5 // Base score

	base := strings.ToLower(filepath.Base(path))
	nodeLower := strings.ToLower(nodeID)

	// Exact match in filename
	if strings.Contains(base, nodeLower) {
		score += 0.3
	}

	// Direct test file (e.g., Node_test.go vs test_node.go)
	if strings.HasPrefix(base, nodeLower) {
		score += 0.1
	}

	// In test directory
	if strings.Contains(path, "/test/") || strings.Contains(path, "/tests/") ||
		strings.Contains(path, "\\test\\") || strings.Contains(path, "\\tests\\") {
		score += 0.1
	}

	return score
}

// deduplicate removes duplicate test files.
func (m *NamingConventionTestMatcher) deduplicate(tests []*TestFile) []*TestFile {
	seen := make(map[string]bool)
	var unique []*TestFile

	for _, test := range tests {
		if !seen[test.Path] {
			seen[test.Path] = true
			unique = append(unique, test)
		}
	}

	return unique
}

// sortByScore sorts test files by match score (highest first).
func (m *NamingConventionTestMatcher) sortByScore(tests []*TestFile) []*TestFile {
	// Simple bubble sort for now
	// In production, use sort.Slice
	n := len(tests)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if tests[j].MatchScore > tests[i].MatchScore {
				tests[i], tests[j] = tests[j], tests[i]
			}
		}
	}
	return tests
}
