package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/search"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	queryVerbose bool
	queryFormat  string
)

var queryCmd = &cobra.Command{
	Use:   "query <symbol>",
	Short: "Query node information by symbol name",
	Long: `Look up a node specification by its name or ID.

Supports exact matching first, then partial/fuzzy matching.
Use --format to change output format.

Examples:
  $ gdc query PlayerController
  $ gdc query IInputManager
  $ gdc query UserService --verbose
  $ gdc query Player --format json
  $ gdc query Controller --format yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runQuery,
}

func init() {
	queryCmd.Flags().BoolVarP(&queryVerbose, "verbose", "v", false, "show verbose output with full details")
	queryCmd.Flags().StringVarP(&queryFormat, "format", "f", "text", "output format (text, json, yaml)")
}

func runQuery(cmd *cobra.Command, args []string) error {
	symbol := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config (is GDC initialized?): %w", err)
	}

	// Graceful degradation: check project readiness and provide helpful guidance
	if checkErr := search.CheckAndSuggest(cfg.ProjectRoot); checkErr != nil {
		if search.IsGracefulError(checkErr) {
			printWarning("%v", checkErr)
		}
	}

	nodesDir := cfg.NodesDir()

	// Load all nodes
	allNodes, err := loadAllNodes(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}

	if len(allNodes) == 0 {
		return fmt.Errorf("no nodes found in %s. Create one with: gdc node create <name>", nodesDir)
	}

	// Find matching node(s)
	matches := findMatchingNodes(symbol, allNodes, cfg.ProjectRoot, nodesDir)

	if len(matches) == 0 {
		// No match found - suggest similar names and probe source for missing graph nodes
		suggestions := findSimilarNodes(symbol, allNodes, cfg.ProjectRoot, nodesDir)
		sourceHints := findSourceHints(cfg, symbol)
		printQueryNotFound(symbol, suggestions, sourceHints)
		return fmt.Errorf("node '%s' not found", symbol)
	}

	// If multiple matches, show all matching names
	if len(matches) > 1 {
		printMultipleMatches(symbol, matches)
		// Use the first (best) match for output
	}

	// Use the best match
	match := matches[0]
	match.ReferenceCount = len(findReferences(match.CanonicalID, allNodes))

	// Output based on format
	switch queryFormat {
	case "json":
		return outputQueryJSON(match)
	case "yaml":
		return outputQueryYAML(match)
	default:
		outputQueryText(match, allNodes, queryVerbose)
	}

	return nil
}

type queryMatch struct {
	Spec           *node.Spec
	CanonicalID    string
	QualifiedName  string
	SpecPath       string
	ImplPath       string
	MatchedBy      string
	MatchedValue   string
	Aliases        []string
	ReferenceCount int
	score          int
}

type querySuggestion struct {
	CanonicalID string
	MatchedBy   string
	MatchedValue string
	score       int
}

type queryAlias struct {
	value string
	label string
}

// findMatchingNodes finds nodes matching the symbol across IDs, qualified names, and paths.
func findMatchingNodes(symbol string, nodes []*node.Spec, projectRoot, nodesDir string) []*queryMatch {
	normalizedSymbol := strings.ToLower(strings.TrimSpace(symbol))
	normalizedPathSymbol := normalizeComparablePath(symbol)
	if normalizedSymbol == "" {
		return nil
	}

	bestByID := make(map[string]*queryMatch)
	for _, spec := range nodes {
		match := evaluateQueryMatch(symbol, normalizedSymbol, normalizedPathSymbol, spec, projectRoot, nodesDir)
		if match == nil {
			continue
		}
		if existing, ok := bestByID[match.CanonicalID]; !ok || match.score > existing.score {
			bestByID[match.CanonicalID] = match
		}
	}

	matches := make([]*queryMatch, 0, len(bestByID))
	for _, match := range bestByID {
		matches = append(matches, match)
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			if len(matches[i].CanonicalID) == len(matches[j].CanonicalID) {
				return matches[i].CanonicalID < matches[j].CanonicalID
			}
			return len(matches[i].CanonicalID) < len(matches[j].CanonicalID)
		}
		return matches[i].score > matches[j].score
	})

	return matches
}

// findSimilarNodes finds nodes with similar names or aliases.
func findSimilarNodes(symbol string, nodes []*node.Spec, projectRoot, nodesDir string) []querySuggestion {
	symbolLower := strings.ToLower(symbol)
	bestByID := make(map[string]querySuggestion)
	for _, spec := range nodes {
		for _, alias := range buildQueryAliases(spec, projectRoot, nodesDir) {
			score := similarityScore(symbolLower, strings.ToLower(alias.value))
			if score == 0 {
				continue
			}

			suggestion := querySuggestion{
				CanonicalID: spec.Node.ID,
				MatchedBy:   alias.label,
				MatchedValue: alias.value,
				score:       score,
			}
			if existing, ok := bestByID[spec.Node.ID]; !ok || suggestion.score > existing.score {
				bestByID[spec.Node.ID] = suggestion
			}
		}
	}

	scored := make([]querySuggestion, 0, len(bestByID))
	for _, suggestion := range bestByID {
		scored = append(scored, suggestion)
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].CanonicalID < scored[j].CanonicalID
		}
		return scored[i].score > scored[j].score
	})

	if len(scored) > 5 {
		scored = scored[:5]
	}

	return scored
}

// similarityScore returns a similarity score between 0-100 for two strings
func similarityScore(a, b string) int {
	// Simple similarity based on common characters and length
	if a == b {
		return 100
	}

	// Count common characters
	aChars := make(map[rune]int)
	for _, c := range a {
		aChars[c]++
	}

	common := 0
	for _, c := range b {
		if aChars[c] > 0 {
			common++
			aChars[c]--
		}
	}

	// Calculate score based on overlap
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	if maxLen == 0 {
		return 0
	}

	score := (common * 100) / maxLen

	// Bonus for matching prefix
	if strings.HasPrefix(b, a) || strings.HasPrefix(a, b) {
		score += 20
	}

	if score > 100 {
		score = 100
	}

	// Only return if score is reasonable (at least 30% similar)
	if score < 30 {
		return 0
	}

	return score
}

func printQueryNotFound(symbol string, suggestions []querySuggestion, sourceHints []string) {
	bold := color.New(color.Bold)
	red := color.New(color.FgRed)

	bold.Printf("\n  Node '%s' ", symbol)
	red.Println("not found")

	if len(suggestions) > 0 {
		yellow := color.New(color.FgYellow)
		yellow.Println("\n  Closest graph matches:")
		for _, s := range suggestions {
			line := fmt.Sprintf("    • %s", color.CyanString(s.CanonicalID))
			if s.MatchedValue != "" && s.MatchedValue != s.CanonicalID {
				line += fmt.Sprintf(" (%s: %s)", s.MatchedBy, s.MatchedValue)
			}
			fmt.Println(line)
		}
	}

	if len(sourceHints) > 0 {
		yellow := color.New(color.FgYellow)
		yellow.Println("\n  Found in source but missing from graph:")
		for _, hint := range sourceHints {
			fmt.Printf("    • %s\n", hint)
		}
		fmt.Println("\n  Suggested next step:")
		fmt.Printf("    %s\n", color.CyanString("gdc sync --direction code --symbols "+symbol))
	}

	fmt.Println()
}

func printMultipleMatches(symbol string, matches []*queryMatch) {
	yellow := color.New(color.FgYellow)
	yellow.Printf("\n  Multiple nodes match '%s':\n", symbol)

	for _, m := range matches {
		label := m.CanonicalID
		if m.QualifiedName != "" && m.QualifiedName != m.CanonicalID {
			label = fmt.Sprintf("%s (%s)", label, m.QualifiedName)
		}
		fmt.Printf("    • %s [%s] via %s\n",
			color.CyanString(label),
			color.HiBlackString(m.Spec.Node.Type),
			color.HiBlackString(m.MatchedBy))
	}

	fmt.Printf("\n  Showing details for: %s\n\n", color.GreenString(matches[0].CanonicalID))
}

func outputQueryText(match *queryMatch, allNodes []*node.Spec, verbose bool) {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	spec := match.Spec

	// Header
	fmt.Println()
	fmt.Printf("Node: %s\n", bold.Sprint(spec.Node.ID))
	fmt.Printf("Type: %s\n", cyan.Sprint(spec.Node.Type))
	fmt.Printf("Layer: %s\n", cyan.Sprint(spec.Node.Layer))
	fmt.Printf("Status: %s\n", formatStatus(spec.Metadata.Status))
	fmt.Printf("Matched By: %s\n", cyan.Sprint(match.MatchedBy))

	if spec.Node.Namespace != "" {
		fmt.Printf("Namespace: %s\n", spec.Node.Namespace)
	}
	if match.QualifiedName != "" {
		fmt.Printf("Qualified Name: %s\n", match.QualifiedName)
	}

	fmt.Println()

	bold.Println("Provenance:")
	fmt.Printf("  Canonical ID: %s\n", match.CanonicalID)
	if match.ImplPath != "" {
		fmt.Printf("  Implementation File: %s\n", match.ImplPath)
	}
	if match.SpecPath != "" {
		fmt.Printf("  Spec File: %s\n", match.SpecPath)
	}
	fmt.Printf("  Trace Links: %d deps, %d refs\n", len(spec.Dependencies), match.ReferenceCount)
	if verbose && len(match.Aliases) > 0 {
		fmt.Printf("  Aliases: %s\n", strings.Join(match.Aliases, ", "))
	}

	fmt.Println()

	// Responsibility
	bold.Println("Responsibility:")
	fmt.Printf("  %s\n", spec.Responsibility.Summary)

	if verbose && spec.Responsibility.Details != "" {
		fmt.Println()
		lines := strings.Split(spec.Responsibility.Details, "\n")
		for _, line := range lines {
			fmt.Printf("  %s\n", strings.TrimSpace(line))
		}
	}

	fmt.Println()

	// Dependencies
	bold.Println("Dependencies:")
	if len(spec.Dependencies) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, dep := range spec.Dependencies {
			depType := dep.Type
			if depType == "" {
				depType = "interface"
			}
			fmt.Printf("  → %s (%s)\n",
				color.CyanString(dep.Target),
				color.HiBlackString(depType))
		}
	}

	fmt.Println()

	// Methods
	if len(spec.Interface.Methods) > 0 {
		bold.Println("Methods:")
		for _, method := range spec.Interface.Methods {
			fmt.Printf("  - %s\n", method.Signature)
			if verbose && method.Description != "" {
				fmt.Printf("    %s\n", color.HiBlackString(method.Description))
			}
		}
		fmt.Println()
	}

	// Properties
	if len(spec.Interface.Properties) > 0 {
		bold.Println("Properties:")
		for _, prop := range spec.Interface.Properties {
			access := prop.Access
			if access == "" {
				access = "get/set"
			}
			fmt.Printf("  - %s: %s {%s}\n", prop.Name, prop.Type, access)
		}
		fmt.Println()
	}

	// Verbose additional info
	if verbose {
		// Implementations (for interfaces)
		if len(spec.Implementations) > 0 {
			bold.Println("Implementations:")
			for _, impl := range spec.Implementations {
				fmt.Printf("  ← %s\n", color.CyanString(impl))
			}
			fmt.Println()
		}

		// Metadata
		bold.Println("Metadata:")
		fmt.Printf("  Created: %s\n", spec.Metadata.Created)
		fmt.Printf("  Updated: %s\n", spec.Metadata.Updated)
		if refNames := findReferences(spec.Node.ID, allNodes); len(refNames) > 0 {
			fmt.Printf("  Referenced By: %s\n", strings.Join(refNames, ", "))
		}
		if len(spec.Metadata.Tags) > 0 {
			fmt.Printf("  Tags: %s\n", strings.Join(spec.Metadata.Tags, ", "))
		}
		if spec.Metadata.Author != "" {
			fmt.Printf("  Author: %s\n", spec.Metadata.Author)
		}
		fmt.Println()
	}
}

// queryNodeJSON is a simplified JSON output structure
type queryNodeJSON struct {
	ID             string   `json:"id"`
	CanonicalID    string   `json:"canonical_id"`
	MatchedBy      string   `json:"matched_by"`
	Type           string   `json:"type"`
	Layer          string   `json:"layer"`
	Status         string   `json:"status"`
	Namespace      string   `json:"namespace,omitempty"`
	QualifiedName  string   `json:"qualified_name,omitempty"`
	SpecPath       string   `json:"spec_path,omitempty"`
	ImplPath       string   `json:"impl_path,omitempty"`
	Aliases        []string `json:"aliases,omitempty"`
	Responsibility string   `json:"responsibility"`
	Dependencies   []string `json:"dependencies,omitempty"`
	Methods        []string `json:"methods,omitempty"`
	Properties     []string `json:"properties,omitempty"`
}

func outputQueryJSON(match *queryMatch) error {
	spec := match.Spec
	// Build dependencies list
	var deps []string
	for _, d := range spec.Dependencies {
		deps = append(deps, d.Target)
	}

	// Build methods list
	var methods []string
	for _, m := range spec.Interface.Methods {
		methods = append(methods, m.Signature)
	}

	// Build properties list
	var props []string
	for _, p := range spec.Interface.Properties {
		props = append(props, fmt.Sprintf("%s: %s", p.Name, p.Type))
	}

	output := queryNodeJSON{
		ID:             spec.Node.ID,
		CanonicalID:    match.CanonicalID,
		MatchedBy:      match.MatchedBy,
		Type:           spec.Node.Type,
		Layer:          spec.Node.Layer,
		Status:         spec.Metadata.Status,
		Namespace:      spec.Node.Namespace,
		QualifiedName:  match.QualifiedName,
		SpecPath:       match.SpecPath,
		ImplPath:       match.ImplPath,
		Aliases:        match.Aliases,
		Responsibility: spec.Responsibility.Summary,
		Dependencies:   deps,
		Methods:        methods,
		Properties:     props,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputQueryYAML(match *queryMatch) error {
	spec := match.Spec
	// Build dependencies list
	var deps []string
	for _, d := range spec.Dependencies {
		deps = append(deps, d.Target)
	}

	// Build methods list
	var methods []string
	for _, m := range spec.Interface.Methods {
		methods = append(methods, m.Signature)
	}

	// Build properties list
	var props []string
	for _, p := range spec.Interface.Properties {
		props = append(props, fmt.Sprintf("%s: %s", p.Name, p.Type))
	}

	output := queryNodeJSON{
		ID:             spec.Node.ID,
		CanonicalID:    match.CanonicalID,
		MatchedBy:      match.MatchedBy,
		Type:           spec.Node.Type,
		Layer:          spec.Node.Layer,
		Status:         spec.Metadata.Status,
		Namespace:      spec.Node.Namespace,
		QualifiedName:  match.QualifiedName,
		SpecPath:       match.SpecPath,
		ImplPath:       match.ImplPath,
		Aliases:        match.Aliases,
		Responsibility: spec.Responsibility.Summary,
		Dependencies:   deps,
		Methods:        methods,
		Properties:     props,
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

func evaluateQueryMatch(symbol, normalizedSymbol, normalizedPathSymbol string, spec *node.Spec, projectRoot, nodesDir string) *queryMatch {
	aliases := buildQueryAliases(spec, projectRoot, nodesDir)
	var best *queryMatch
	for _, alias := range aliases {
		score, matchedBy := classifyQueryMatch(symbol, normalizedSymbol, normalizedPathSymbol, alias)
		if score == 0 {
			continue
		}

		match := &queryMatch{
			Spec:           spec,
			CanonicalID:    spec.Node.ID,
			QualifiedName:  qualifiedNodeName(spec),
			SpecPath:       filepath.Join(nodesDir, spec.Node.ID+".yaml"),
			ImplPath:       spec.Node.FilePath,
			MatchedBy:      matchedBy,
			MatchedValue:   alias.value,
			Aliases:        queryAliasValues(aliases),
			ReferenceCount: 0,
			score:          score,
		}

		if best == nil || match.score > best.score {
			best = match
		}
	}

	return best
}

func buildQueryAliases(spec *node.Spec, projectRoot, nodesDir string) []queryAlias {
	var aliases []queryAlias
	aliases = append(aliases, queryAlias{value: spec.Node.ID, label: "id"})

	if qualified := qualifiedNodeName(spec); qualified != "" && qualified != spec.Node.ID {
		aliases = append(aliases, queryAlias{value: qualified, label: "qualified name"})
	}

	if spec.Node.FilePath != "" {
		for _, variant := range projectPathVariants(spec.Node.FilePath, projectRoot) {
			label := "implementation file"
			if filepath.Base(variant) == variant {
				label = "implementation file name"
			}
			aliases = append(aliases, queryAlias{value: variant, label: label})
		}
	}

	for _, variant := range projectPathVariants(filepath.Join(nodesDir, spec.Node.ID+".yaml"), projectRoot) {
		label := "spec file"
		if filepath.Base(variant) == variant {
			label = "spec file name"
		}
		aliases = append(aliases, queryAlias{value: variant, label: label})
	}

	return dedupeQueryAliases(aliases)
}

func dedupeQueryAliases(aliases []queryAlias) []queryAlias {
	seen := make(map[string]bool, len(aliases))
	result := make([]queryAlias, 0, len(aliases))
	for _, alias := range aliases {
		key := alias.label + "::" + alias.value
		if alias.value == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, alias)
	}
	return result
}

func queryAliasValues(aliases []queryAlias) []string {
	values := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		values = append(values, alias.value)
	}
	return dedupeStrings(values)
}

func classifyQueryMatch(symbol, normalizedSymbol, normalizedPathSymbol string, alias queryAlias) (int, string) {
	normalizedAlias := strings.ToLower(strings.TrimSpace(alias.value))
	normalizedAliasPath := normalizeComparablePath(alias.value)
	if normalizedAlias == "" {
		return 0, ""
	}

	weight := queryAliasWeight(alias.label)
	switch {
	case alias.value == symbol || (normalizedPathSymbol != "" && normalizedAliasPath == normalizedPathSymbol):
		return 1000 + weight, "exact " + alias.label
	case normalizedAlias == normalizedSymbol:
		return 950 + weight, "case-insensitive " + alias.label
	case normalizedPathSymbol != "" && strings.HasPrefix(normalizedAliasPath, normalizedPathSymbol):
		return 720 + weight - len(normalizedAliasPath), "prefix " + alias.label
	case strings.HasPrefix(normalizedAlias, normalizedSymbol):
		return 700 + weight - len(normalizedAlias), "prefix " + alias.label
	case normalizedPathSymbol != "" && strings.Contains(normalizedAliasPath, normalizedPathSymbol):
		return 520 + weight - len(normalizedAliasPath), "partial " + alias.label
	case strings.Contains(normalizedAlias, normalizedSymbol):
		return 500 + weight - len(normalizedAlias), "partial " + alias.label
	default:
		return 0, ""
	}
}

func queryAliasWeight(label string) int {
	switch label {
	case "id":
		return 80
	case "qualified name":
		return 70
	case "implementation file":
		return 60
	case "implementation file name":
		return 55
	case "spec file":
		return 50
	case "spec file name":
		return 45
	default:
		return 10
	}
}

func qualifiedNodeName(spec *node.Spec) string {
	if spec == nil || spec.Node.Namespace == "" {
		return ""
	}
	return spec.Node.Namespace + "." + spec.Node.ID
}

func findSourceHints(cfg *config.Config, symbol string) []string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil
	}

	if strings.ContainsAny(symbol, `/\`) {
		resolved := cfg.ResolvePath(symbol)
		if _, err := os.Stat(resolved); err == nil {
			rel, relErr := filepath.Rel(cfg.ProjectRoot, resolved)
			if relErr != nil {
				return []string{resolved}
			}
			return []string{filepath.ToSlash(rel)}
		}
	}

	sourceRoot := cfg.ProjectRoot
	if cfg.Project.SourceDir != "" {
		sourceRoot = cfg.ResolvePath(cfg.Project.SourceDir)
	}

	pattern := regexp.QuoteMeta(symbol)
	re, err := regexp.Compile(`(?i)\b` + pattern + `\b`)
	if err != nil {
		return nil
	}

	extensions := sourceExtensionsForLanguage(cfg.Project.Language)
	var hints []string
	_ = filepath.Walk(sourceRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == ".gdc" || name == "node_modules" || name == "vendor" || name == "bin" || name == "obj" {
				return filepath.SkipDir
			}
			return nil
		}
		if len(extensions) > 0 && !hasAllowedExtension(path, extensions) {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil || !re.Match(data) {
			return nil
		}

		rel, relErr := filepath.Rel(cfg.ProjectRoot, path)
		if relErr != nil {
			hints = append(hints, path)
		} else {
			hints = append(hints, filepath.ToSlash(rel))
		}
		if len(hints) >= 3 {
			return fmt.Errorf("enough-hints")
		}
		return nil
	})

	return hints
}

func sourceExtensionsForLanguage(language string) []string {
	switch strings.ToLower(language) {
	case "go", "golang":
		return []string{".go"}
	case "csharp", "cs", "c#":
		return []string{".cs"}
	case "typescript", "ts":
		return []string{".ts", ".tsx"}
	default:
		return nil
	}
}

func hasAllowedExtension(path string, extensions []string) bool {
	if len(extensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range extensions {
		if ext == allowed {
			return true
		}
	}
	return false
}
