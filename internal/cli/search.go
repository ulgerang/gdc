package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/search"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	searchCaseSensitive bool
	searchFilePattern   string
	searchMaxResults    int
	searchRegex         bool
	searchContext       int
)

// SearchResult represents a single search match
type SearchResult struct {
	File          string
	Line          int
	Content       string
	ContextBefore []string // lines before the match (for --context)
	ContextAfter  []string // lines after the match (for --context)
}

var searchCmd = &cobra.Command{
	Use:   "search <pattern>",
	Short: "Search for pattern in codebase",
	Long: `Search for a text pattern in source files.

Examples:
  $ gdc search "PlayerController"
  $ gdc search "TODO" --file-pattern "*.go"
  $ gdc search "func.*Handler" --regex
  $ gdc search "UserService" --case-sensitive
  $ gdc search "error" --max-results 50
  $ gdc search "class" --context 2`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().BoolVar(&searchCaseSensitive, "case-sensitive", false, "case-sensitive search")
	searchCmd.Flags().StringVarP(&searchFilePattern, "file-pattern", "f", "", "file pattern to search (e.g., \"*.go\", \"*.cs\")")
	searchCmd.Flags().IntVarP(&searchMaxResults, "max-results", "m", 100, "maximum number of results")
	searchCmd.Flags().BoolVarP(&searchRegex, "regex", "r", false, "treat pattern as regular expression")
	searchCmd.Flags().IntVar(&searchContext, "context", 0, "number of context lines to show")
}

func runSearch(cmd *cobra.Command, args []string) error {
	pattern := args[0]

	// Determine search root
	var searchRoot string
	var hasProject bool

	cfg, err := config.Load("")
	if err != nil {
		// No project initialized - search from current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		searchRoot = cwd
		hasProject = false
		printInfo("No GDC project found, searching from current directory")
	} else {
		searchRoot = cfg.ProjectRoot
		hasProject = true

		// Graceful degradation: check project readiness and provide helpful guidance
		if checkErr := search.CheckAndSuggest(cfg.ProjectRoot); checkErr != nil {
			if search.IsGracefulError(checkErr) {
				printWarning("%v", checkErr)
			}
		}
	}

	// Build the pattern matcher
	var patternMatcher func(string) bool
	if searchRegex {
		regexFlags := ""
		if !searchCaseSensitive {
			regexFlags = "(?i)"
		}
		re, err := regexp.Compile(regexFlags + pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
		patternMatcher = func(s string) bool {
			return re.MatchString(s)
		}
	} else {
		searchPattern := pattern
		if !searchCaseSensitive {
			searchPattern = strings.ToLower(pattern)
		}
		patternMatcher = func(s string) bool {
			if !searchCaseSensitive {
				return strings.Contains(strings.ToLower(s), searchPattern)
			}
			return strings.Contains(s, searchPattern)
		}
	}

	// Determine file pattern
	filePattern := searchFilePattern

	// Validate --max-results: 0 means unlimited, negative is invalid
	if searchMaxResults < 0 {
		return fmt.Errorf("--max-results must be 0 (unlimited) or a positive integer")
	}

	// Collect results
	var results []SearchResult
	resultCount := 0

	// Directories to skip
	skipDirs := map[string]bool{
		".git":         true,
		".gdc":         true,
		"node_modules": true,
		"bin":          true,
		"obj":          true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		".idea":        true,
		".vscode":      true,
		"__pycache__":  true,
		".venv":        true,
		"venv":         true,
	}

	err = filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Check file pattern using relative path for proper glob matching
		if filePattern != "" {
			relPath, relErr := filepath.Rel(searchRoot, path)
			if relErr != nil {
				relPath = info.Name()
			}
			// Normalize to forward slashes for cross-platform glob matching
			relPath = filepath.ToSlash(relPath)
			normalizedPattern := filepath.ToSlash(filePattern)

			matched, matchErr := filepath.Match(normalizedPattern, relPath)
			if matchErr != nil {
				matched = false
			}
			// Also try matching against just the filename for simple patterns like "*.go"
			if !matched {
				matched, _ = filepath.Match(filePattern, info.Name())
			}
			if !matched {
				return nil
			}
		}

		// Skip binary files and non-text files
		if isBinaryFile(path) {
			return nil
		}

		// Search in file
		fileResults, err := searchInFile(path, patternMatcher, searchContext)
		if err != nil {
			return nil // Skip files we can't read
		}

		for _, r := range fileResults {
			// Make path relative to search root
			relPath, err := filepath.Rel(searchRoot, r.File)
			if err != nil {
				relPath = r.File
			}
			r.File = filepath.ToSlash(relPath)

			results = append(results, r)
			resultCount++
			if searchMaxResults > 0 && resultCount >= searchMaxResults {
				return fmt.Errorf("max_results_reached")
			}
		}

		return nil
	})

	// Handle max results reached
	if err != nil && err.Error() != "max_results_reached" {
		return fmt.Errorf("search failed: %w", err)
	}

	// Output results
	if len(results) == 0 {
		printInfo("No matches found for pattern: %s", pattern)
		if !hasProject {
			printInfo("Tip: Initialize a GDC project with 'gdc init' for better search scope")
		}
		return nil
	}

	outputSearchResults(pattern, results, hasProject)

	return nil
}

func searchInFile(filePath string, patternMatcher func(string) bool, contextLines int) ([]SearchResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	// Remove trailing empty line from Split (if file ends with \n)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var results []SearchResult

	for i, line := range lines {
		if patternMatcher(line) {
			// Truncate long lines
			content := line
			if len(content) > 100 {
				content = content[:97] + "..."
			}
			content = strings.TrimSpace(content)

			r := SearchResult{
				File:    filePath,
				Line:    i + 1,
				Content: content,
			}

			// Collect context lines
			if contextLines > 0 {
				// Before context
				start := i - contextLines
				if start < 0 {
					start = 0
				}
				for j := start; j < i; j++ {
					ctx := lines[j]
					if len(ctx) > 100 {
						ctx = ctx[:97] + "..."
					}
					r.ContextBefore = append(r.ContextBefore, strings.TrimSpace(ctx))
				}

				// After context
				end := i + contextLines + 1
				if end > len(lines) {
					end = len(lines)
				}
				for j := i + 1; j < end; j++ {
					ctx := lines[j]
					if len(ctx) > 100 {
						ctx = ctx[:97] + "..."
					}
					r.ContextAfter = append(r.ContextAfter, strings.TrimSpace(ctx))
				}
			}

			results = append(results, r)
		}
	}

	return results, nil
}

func isBinaryFile(path string) bool {
	// Check by extension first
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := map[string]bool{
		".exe":    true,
		".dll":    true,
		".so":     true,
		".dylib":  true,
		".bin":    true,
		".png":    true,
		".jpg":    true,
		".jpeg":   true,
		".gif":    true,
		".ico":    true,
		".pdf":    true,
		".zip":    true,
		".tar":    true,
		".gz":     true,
		".db":     true,
		".sqlite": true,
		".lock":   true,
		".sum":    true,
	}
	if binaryExts[ext] {
		return true
	}

	// Check file name patterns
	base := filepath.Base(path)
	lockedFiles := map[string]bool{
		"go.sum":            true,
		"go.mod":            false, // go.mod is text
		"package-lock.json": true,
		"yarn.lock":         true,
		"pnpm-lock.yaml":    true,
	}
	if lockedFiles[base] {
		return true
	}

	return false
}

func outputSearchResults(pattern string, results []SearchResult, hasProject bool) {
	// Print header
	fmt.Println()
	patternColor := color.New(color.Bold, color.FgYellow).SprintFunc()
	fmt.Printf("Search results for %s:\n", patternColor(pattern))
	fmt.Println()

	highlightColor := color.New(color.FgYellow).SprintFunc()
	dimColor := color.New(color.FgHiBlack).SprintFunc()

	if searchContext > 0 {
		// Context mode: grep-like output with surrounding lines
		for i, r := range results {
			filePath := r.File
			if len(filePath) > 60 {
				filePath = "..." + filePath[len(filePath)-57:]
			}

			// Print context-before lines
			for j, ctxLine := range r.ContextBefore {
				beforeLineNum := r.Line - len(r.ContextBefore) + j
				fmt.Printf("  %s %s\n",
					dimColor(fmt.Sprintf("%s:%d:", filePath, beforeLineNum)),
					dimColor(ctxLine))
			}

			// Print the matching line (highlighted)
			content := r.Content
			if !searchRegex {
				content = highlightMatch(content, pattern, searchCaseSensitive, highlightColor)
			}
			fmt.Printf("  %s:%d: %s\n",
				color.CyanString(filePath),
				r.Line,
				content)

			// Print context-after lines
			for j, ctxLine := range r.ContextAfter {
				afterLineNum := r.Line + j + 1
				fmt.Printf("  %s %s\n",
					dimColor(fmt.Sprintf("%s:%d:", filePath, afterLineNum)),
					dimColor(ctxLine))
			}

			// Separator between result groups
			if i < len(results)-1 {
				fmt.Println("  --")
			}
		}
	} else {
		// Table mode (original behavior)
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"File", "Line", "Content"})
		table.SetBorder(false)
		table.SetAutoWrapText(false)
		table.SetHeaderLine(false)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		// Set header colors
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		)

		// Add rows
		for _, r := range results {
			// Truncate file path if too long
			filePath := r.File
			if len(filePath) > 40 {
				// Keep the end of the path which is usually more relevant
				filePath = "..." + filePath[len(filePath)-37:]
			}

			// Highlight the matching pattern in content
			content := r.Content
			if !searchRegex {
				content = highlightMatch(content, pattern, searchCaseSensitive, highlightColor)
			}

			table.Append([]string{
				filePath,
				fmt.Sprintf("%d", r.Line),
				content,
			})
		}

		table.Render()
	}

	// Print summary
	fmt.Println()
	resultStr := "result"
	if len(results) != 1 {
		resultStr = "results"
	}
	fmt.Printf("Found %d %s", len(results), resultStr)
	if searchMaxResults > 0 && len(results) >= searchMaxResults {
		fmt.Printf(" (max results reached)")
	}
	fmt.Println()
}

// highlightMatch highlights the matching pattern in the content
func highlightMatch(content, pattern string, caseSensitive bool, highlightFunc func(...interface{}) string) string {
	if pattern == "" {
		return content
	}

	searchContent := content
	searchPattern := pattern
	if !caseSensitive {
		searchContent = strings.ToLower(content)
		searchPattern = strings.ToLower(pattern)
	}

	idx := strings.Index(searchContent, searchPattern)
	if idx == -1 {
		return content
	}

	// Find the actual text to highlight (preserving original case)
	actualMatch := content[idx : idx+len(pattern)]
	before := content[:idx]
	after := content[idx+len(pattern):]

	return before + highlightFunc(actualMatch) + after
}
