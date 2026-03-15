package extract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileSystemCodeLoader loads implementation code from the file system.
// This is the default implementation that works without any code index.
type FileSystemCodeLoader struct {
	projectRoot string
}

// NewFileSystemCodeLoader creates a new file system based code loader.
func NewFileSystemCodeLoader(projectRoot string) *FileSystemCodeLoader {
	return &FileSystemCodeLoader{
		projectRoot: projectRoot,
	}
}

// IsAvailable always returns true for file system loader.
func (l *FileSystemCodeLoader) IsAvailable() bool {
	return true
}

// LoadImplementation retrieves implementation code for a node.
func (l *FileSystemCodeLoader) LoadImplementation(ctx context.Context, spec *NodeSpec) (*CodeLoadResult, error) {
	// If spec has explicit implementation paths, use those
	if len(spec.Implementations) > 0 {
		return l.loadFromExplicitPaths(ctx, spec)
	}

	// Otherwise, try to find by convention
	return l.loadByConvention(ctx, spec)
}

// LoadFunction retrieves a specific function implementation.
func (l *FileSystemCodeLoader) LoadFunction(ctx context.Context, spec *NodeSpec, functionName string) (*FunctionCode, error) {
	// First load the full implementation
	result, err := l.LoadImplementation(ctx, spec)
	if err != nil {
		return nil, err
	}

	// Search for the function in the loaded code
	if result.PrimaryFile != nil {
		fn, err := extractFunction(result.PrimaryFile.Content, functionName)
		if err == nil {
			fn.File = result.PrimaryFile.Path
			return fn, nil
		}
	}

	return nil, fmt.Errorf("function %s not found in %s", functionName, spec.ID)
}

func (l *FileSystemCodeLoader) loadFromExplicitPaths(ctx context.Context, spec *NodeSpec) (*CodeLoadResult, error) {
	result := &CodeLoadResult{
		Language: detectLanguage(spec.ID, spec.Implementations),
		FoundBy:  "explicit",
	}

	for i, path := range spec.Implementations {
		// Resolve relative paths
		if !filepath.IsAbs(path) {
			path = filepath.Join(l.projectRoot, path)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			if i == 0 {
				return nil, fmt.Errorf("%w: %v", ErrCodeNotFound, err)
			}
			// Continue loading additional files even if some fail
			continue
		}

		file := &SourceFile{
			Path:     path,
			Content:  string(content),
			Lines:    strings.Count(string(content), "\n") + 1,
			Language: detectLanguageFromPath(path),
		}

		if i == 0 {
			result.PrimaryFile = file
		} else {
			result.AdditionalFiles = append(result.AdditionalFiles, file)
		}
		result.TotalLines += file.Lines
	}

	return result, nil
}

func (l *FileSystemCodeLoader) loadByConvention(ctx context.Context, spec *NodeSpec) (*CodeLoadResult, error) {
	// Common patterns to search for implementation files
	patterns := generateSearchPatterns(spec.ID, spec.Type)

	for _, pattern := range patterns {
		path := filepath.Join(l.projectRoot, pattern)
		if _, err := os.Stat(path); err == nil {
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			return &CodeLoadResult{
				PrimaryFile: &SourceFile{
					Path:     path,
					Content:  string(content),
					Lines:    strings.Count(string(content), "\n") + 1,
					Language: detectLanguageFromPath(path),
				},
				TotalLines: strings.Count(string(content), "\n") + 1,
				Language:   detectLanguageFromPath(path),
				FoundBy:    "convention",
			}, nil
		}
	}

	return nil, fmt.Errorf("%w: no implementation found for %s", ErrCodeNotFound, spec.ID)
}

// generateSearchPatterns generates possible file paths based on node ID and type.
func generateSearchPatterns(nodeID, nodeType string) []string {
	patterns := []string{
		// Go patterns
		fmt.Sprintf("%s.go", strings.ToLower(nodeID)),
		filepath.Join("internal", fmt.Sprintf("%s.go", strings.ToLower(nodeID))),
		filepath.Join("pkg", fmt.Sprintf("%s.go", strings.ToLower(nodeID))),

		// C# patterns
		fmt.Sprintf("%s.cs", nodeID),
		filepath.Join("src", fmt.Sprintf("%s.cs", nodeID)),

		// TypeScript patterns
		fmt.Sprintf("%s.ts", nodeID),
		fmt.Sprintf("%s.js", nodeID),
		filepath.Join("src", fmt.Sprintf("%s.ts", nodeID)),

		// Common directories
		filepath.Join("cmd", strings.ToLower(nodeID), "main.go"),
	}

	return patterns
}

// detectLanguage detects language from file paths.
func detectLanguage(nodeID string, paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return detectLanguageFromPath(paths[0])
}

// detectLanguageFromPath detects language from file extension.
func detectLanguageFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".cs":
		return "csharp"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	default:
		return ""
	}
}

// extractFunction attempts to extract a specific function from source code.
// This is a simplified implementation - in production, this would use AST parsing.
func extractFunction(content, functionName string) (*FunctionCode, error) {
	// For now, return a placeholder
	// Full implementation would parse the AST to find the exact function boundaries
	return &FunctionCode{
		Name:      functionName,
		Signature: fmt.Sprintf("func %s(...)", functionName),
		Body:      "// Function extraction not fully implemented\n// See full file content",
	}, nil
}
