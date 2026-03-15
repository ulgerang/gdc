package extract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SimpleCallerResolver finds callers using basic text analysis.
// This implementation doesn't require a code index or language server.
type SimpleCallerResolver struct {
	projectRoot string
	maxFileSize int64 // Maximum file size to scan (bytes)
}

// NewSimpleCallerResolver creates a new simple caller resolver.
func NewSimpleCallerResolver(projectRoot string) *SimpleCallerResolver {
	return &SimpleCallerResolver{
		projectRoot: projectRoot,
		maxFileSize: 1024 * 1024, // 1MB max file size
	}
}

// IsAvailable always returns true as this resolver doesn't require index.
func (r *SimpleCallerResolver) IsAvailable() bool {
	return true
}

// FindCallers finds all call sites for a node's methods.
func (r *SimpleCallerResolver) FindCallers(ctx context.Context, spec *NodeSpec, maxCallers int) ([]*CallerInfo, error) {
	var callers []*CallerInfo

	// Generate patterns to search for
	patterns := r.generateCallPatterns(spec)
	if len(patterns) == 0 {
		return callers, nil
	}

	// Find all source files to scan
	sourceFiles, err := r.findSourceFiles()
	if err != nil {
		return nil, err
	}

	// Scan each file for call patterns
	for _, file := range sourceFiles {
		if ctx.Err() != nil {
			return callers, ctx.Err()
		}

		fileCallers, err := r.scanFileForCalls(file, spec.ID, patterns)
		if err != nil {
			continue
		}

		callers = append(callers, fileCallers...)

		if len(callers) >= maxCallers {
			break
		}
	}

	// Sort by relevance and limit results
	callers = r.sortByRelevance(callers)
	if len(callers) > maxCallers {
		callers = callers[:maxCallers]
	}

	return callers, nil
}

// FindReferences finds all references to the node.
func (r *SimpleCallerResolver) FindReferences(ctx context.Context, spec *NodeSpec, maxRefs int) ([]*ReferenceInfo, error) {
	var refs []*ReferenceInfo

	// Find import/type references
	sourceFiles, err := r.findSourceFiles()
	if err != nil {
		return nil, err
	}

	for _, file := range sourceFiles {
		if ctx.Err() != nil {
			return refs, ctx.Err()
		}

		fileRefs, err := r.scanFileForReferences(file, spec.ID)
		if err != nil {
			continue
		}

		refs = append(refs, fileRefs...)

		if len(refs) >= maxRefs {
			break
		}
	}

	if len(refs) > maxRefs {
		refs = refs[:maxRefs]
	}

	return refs, nil
}

// FindFunctionCallers finds callers of a specific function.
func (r *SimpleCallerResolver) FindFunctionCallers(ctx context.Context, spec *NodeSpec, functionName string, maxCallers int) ([]*CallerInfo, error) {
	// Find all callers first
	allCallers, err := r.FindCallers(ctx, spec, maxCallers*3) // Get more to filter
	if err != nil {
		return nil, err
	}

	// Filter by function name
	var filtered []*CallerInfo
	for _, caller := range allCallers {
		if strings.Contains(caller.CallSnippet, functionName) {
			filtered = append(filtered, caller)
		}
	}

	if len(filtered) > maxCallers {
		filtered = filtered[:maxCallers]
	}

	return filtered, nil
}

// generateCallPatterns generates regex patterns to search for calls.
func (r *SimpleCallerResolver) generateCallPatterns(spec *NodeSpec) []string {
	var patterns []string

	// Direct calls: nodeName.MethodName or nodeName()
	for _, method := range spec.Interface.Methods {
		// Pattern: NodeName.MethodName
		patterns = append(patterns,
			fmt.Sprintf(`%s\.%s\s*\(`, regexp.QuoteMeta(spec.ID), regexp.QuoteMeta(method.Name)),
		)

		// Pattern: methodName( (for top-level functions)
		if spec.Type != "class" && spec.Type != "interface" {
			patterns = append(patterns,
				fmt.Sprintf(`\b%s\s*\(`, regexp.QuoteMeta(method.Name)),
			)
		}
	}

	// Constructor calls
	for range spec.Interface.Constructors {
		// Pattern: new NodeName( or NodeName(
		patterns = append(patterns,
			fmt.Sprintf(`(?:new\s+)?%s\s*\(`, regexp.QuoteMeta(spec.ID)),
		)
	}

	// Type references
	patterns = append(patterns, fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(spec.ID)))

	return patterns
}

// findSourceFiles finds all source files in the project.
func (r *SimpleCallerResolver) findSourceFiles() ([]string, error) {
	var files []string

	extensions := []string{".go", ".cs", ".ts", ".tsx", ".js", ".jsx", ".py", ".java", ".rs"}

	err := filepath.Walk(r.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking
		}

		if info.IsDir() {
			// Skip common non-source directories
			name := info.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" ||
				name == "dist" || name == "build" || name == "bin" || name == "obj" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check file extension
		ext := strings.ToLower(filepath.Ext(path))
		for _, validExt := range extensions {
			if ext == validExt {
				files = append(files, path)
				break
			}
		}

		return nil
	})

	return files, err
}

// scanFileForCalls scans a file for call patterns.
func (r *SimpleCallerResolver) scanFileForCalls(filePath, nodeID string, patterns []string) ([]*CallerInfo, error) {
	// Check file size
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if info.Size() > r.maxFileSize {
		return nil, fmt.Errorf("file too large: %s", filePath)
	}

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var callers []*CallerInfo

	// Get file metadata
	pkg := extractPackage(lines)

	// Scan each line
	for lineNum, line := range lines {
		for _, pattern := range patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}

			if re.MatchString(line) {
				// Extract function name from surrounding context
				funcName := extractFunctionName(lines, lineNum)

				caller := &CallerInfo{
					File:         filePath,
					Line:         lineNum + 1,
					Column:       1,
					Function:     funcName,
					Package:      pkg,
					CallSnippet:  strings.TrimSpace(line),
					ContextLines: getContextLines(lines, lineNum, 2),
					Relevance:    calculateRelevance(nodeID, line),
				}

				callers = append(callers, caller)
				break // Only count first match per line
			}
		}
	}

	return callers, nil
}

// scanFileForReferences scans a file for type references.
func (r *SimpleCallerResolver) scanFileForReferences(filePath, nodeID string) ([]*ReferenceInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var refs []*ReferenceInfo

	// Pattern for type references
	refPattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(nodeID))
	re, _ := regexp.Compile(refPattern)

	for lineNum, line := range lines {
		if re.MatchString(line) {
			refType := "type_reference"
			if strings.Contains(line, "import") {
				refType = "import"
			}

			refs = append(refs, &ReferenceInfo{
				File:    filePath,
				Line:    lineNum + 1,
				Column:  1,
				Type:    refType,
				Snippet: strings.TrimSpace(line),
			})
		}
	}

	return refs, nil
}

// extractPackage extracts package name from file content.
func extractPackage(lines []string) string {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "package "))
		}
		if strings.HasPrefix(line, "namespace ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "namespace "))
		}
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// extractFunctionName extracts function name from context.
func extractFunctionName(lines []string, callLine int) string {
	// Look backwards for function/method definition
	for i := callLine; i >= 0 && i > callLine-20; i-- {
		line := lines[i]

		// Go: func Name(
		if match := regexp.MustCompile(`func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(`).FindStringSubmatch(line); match != nil {
			return match[1]
		}

		// C#: void Name( or public Type Name(
		if match := regexp.MustCompile(`(?:public|private|protected|internal)?\s+\w+\s+(\w+)\s*\(`).FindStringSubmatch(line); match != nil {
			return match[1]
		}

		// TypeScript/JavaScript: function Name( or Name: function(
		if match := regexp.MustCompile(`(?:function|async\s+function)\s+(\w+)\s*\(`).FindStringSubmatch(line); match != nil {
			return match[1]
		}
	}

	return "unknown"
}

// getContextLines returns surrounding lines for context.
func getContextLines(lines []string, centerLine, radius int) []string {
	var result []string
	start := centerLine - radius
	if start < 0 {
		start = 0
	}
	end := centerLine + radius + 1
	if end > len(lines) {
		end = len(lines)
	}

	for i := start; i < end; i++ {
		prefix := "  "
		if i == centerLine {
			prefix = "> "
		}
		result = append(result, prefix+lines[i])
	}

	return result
}

// calculateRelevance calculates relevance score for a call.
func calculateRelevance(nodeID, line string) float64 {
	score := 0.5

	// Direct method call is more relevant
	if strings.Contains(line, nodeID+".") {
		score += 0.3
	}

	// Assignment or initialization is relevant
	if strings.Contains(line, "=") && strings.Contains(line, nodeID) {
		score += 0.1
	}

	return score
}

// sortByRelevance sorts callers by relevance (highest first).
func (r *SimpleCallerResolver) sortByRelevance(callers []*CallerInfo) []*CallerInfo {
	n := len(callers)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if callers[j].Relevance > callers[i].Relevance {
				callers[i], callers[j] = callers[j], callers[i]
			}
		}
	}
	return callers
}
