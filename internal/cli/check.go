package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/parser"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	checkFix           bool
	checkCategory      string
	checkSeverity      string
	checkVerifyImpl    bool
	checkFailOnMissing bool
	checkNoOrphanInfo  bool
	checkOrphanFilter  string
	checkLayerStrict   bool
	checkCIMode        bool
	checkExitOnWarning bool
	checkMaxErrors     int
	checkMaxWarnings   int
	checkMaxInfo       int
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate graph integrity",
	Long: `Check the graph for consistency and potential issues.

Validation categories:
  - missing_ref     - References to non-existent nodes
  - hash_mismatch   - Contract hash mismatches
  - cycle           - Circular dependencies
  - orphan          - Nodes not referenced anywhere
  - impl_missing    - file_path missing or symbol not found in code
  - impl_mismatch   - spec members do not match implementation
  - layer_violation - Architecture layer violations
  - srp_violation   - Too many dependencies (SRP)

Examples:
  $ gdc check
  $ gdc check --verify-impl
  $ gdc check --ci-mode --max-warnings 5
  $ gdc check --category hash_mismatch
  $ gdc check --severity error`,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().BoolVar(&checkFix, "fix", false, "automatically fix issues where possible")
	checkCmd.Flags().StringVar(&checkCategory, "category", "", "filter by category")
	checkCmd.Flags().StringVar(&checkSeverity, "severity", "", "filter by severity (error, warning, info)")
	checkCmd.Flags().BoolVar(&checkVerifyImpl, "verify-impl", false, "verify that node file_path and interface are implemented in code")
	checkCmd.Flags().BoolVar(&checkFailOnMissing, "fail-on-missing", false, "treat implementation mismatches as errors when verifying")
	checkCmd.Flags().BoolVar(&checkNoOrphanInfo, "no-orphan-info", false, "suppress orphan info messages")
	checkCmd.Flags().StringVar(&checkOrphanFilter, "orphan-filter", "", "suppress orphan info for entry points or matching node patterns")
	checkCmd.Flags().BoolVar(&checkLayerStrict, "layer-strict", false, "treat layer violations as errors")
	checkCmd.Flags().BoolVar(&checkCIMode, "ci-mode", false, "use concise CI-friendly output and explicit exit policy")
	checkCmd.Flags().BoolVar(&checkExitOnWarning, "exit-on-warning", false, "return a failing exit code when warnings are present")
	checkCmd.Flags().IntVar(&checkMaxErrors, "max-errors", -1, "fail when error count exceeds this threshold (-1 uses default policy)")
	checkCmd.Flags().IntVar(&checkMaxWarnings, "max-warnings", -1, "fail when warning count exceeds this threshold")
	checkCmd.Flags().IntVar(&checkMaxInfo, "max-info", -1, "fail when info count exceeds this threshold")
}

type Issue struct {
	Severity   string
	Category   string
	SourceNode string
	TargetNode string
	Message    string
	Suggestion string
}

func runCheck(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.NodesDir()

	if !quiet {
		fmt.Println("Running validation checks...")
		fmt.Println()
	}

	nodes, err := loadAllNodes(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}

	nodeMap := buildSpecLookup(nodes)

	var issues []Issue
	issues = append(issues, checkMissingRefs(nodes, nodeMap)...)
	issues = append(issues, checkHashMismatch(nodes, nodeMap)...)
	issues = append(issues, checkCycles(nodes, nodeMap)...)
	issues = append(issues, checkOrphans(nodes, nodeMap, cfg.Validation.Orphan, checkNoOrphanInfo, checkOrphanFilter)...)
	issues = append(issues, checkSRPViolations(nodes, cfg.Validation.SRPThreshold)...)
	if !isDisabled(cfg.Validation.Disabled, "layer_violation") {
		issues = append(issues, checkLayerViolations(nodes, cfg.Architecture.Layers, resolveLayerViolationSeverity(cfg.Architecture.ViolationLevel, checkLayerStrict))...)
	}
	if checkVerifyImpl {
		issues = append(issues, checkImplementationConsistency(nodes, cfg, checkFailOnMissing)...)
	}

	issues = filterIssues(issues, checkCategory, checkSeverity)

	for i, issue := range issues {
		if isWarnOnly(cfg.Validation.WarnOnly, issue.Category) && issue.Severity == "error" {
			issues[i].Severity = "warning"
		}
	}

	errors := countBySeverity(issues, "error")
	warnings := countBySeverity(issues, "warning")
	infos := countBySeverity(issues, "info")

	if len(issues) == 0 {
		if checkCIMode {
			fmt.Println(formatCISummary(errors, warnings, infos, nil, false))
		} else {
			printSuccess("No issues found!")
		}
		return nil
	}

	if !checkCIMode || verbose {
		displayIssues(issues)
	}

	breaches, exitCode := evaluateCheckExitPolicy(errors, warnings, infos)
	if checkCIMode {
		fmt.Println(formatCISummary(errors, warnings, infos, breaches, exitCode != 0))
	} else {
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("Summary: %s, %s, %s\n",
			color.RedString("%d errors", errors),
			color.YellowString("%d warnings", warnings),
			color.CyanString("%d info", infos),
		)
		if len(breaches) > 0 {
			for _, breach := range breaches {
				fmt.Printf("  %s\n", color.HiBlackString(breach))
			}
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

func evaluateCheckExitPolicy(errors, warnings, infos int) ([]string, int) {
	var breaches []string

	maxErrors := checkMaxErrors
	if maxErrors < 0 {
		maxErrors = 0
	}
	if errors > maxErrors {
		if checkMaxErrors >= 0 {
			breaches = append(breaches, fmt.Sprintf("errors %d exceeded max-errors %d", errors, checkMaxErrors))
		} else {
			breaches = append(breaches, fmt.Sprintf("errors present (%d)", errors))
		}
	}
	if checkExitOnWarning && warnings > 0 {
		breaches = append(breaches, fmt.Sprintf("warnings present (%d)", warnings))
	}
	if checkMaxWarnings >= 0 && warnings > checkMaxWarnings {
		breaches = append(breaches, fmt.Sprintf("warnings %d exceeded max-warnings %d", warnings, checkMaxWarnings))
	}
	if checkMaxInfo >= 0 && infos > checkMaxInfo {
		breaches = append(breaches, fmt.Sprintf("info %d exceeded max-info %d", infos, checkMaxInfo))
	}

	if len(breaches) > 0 {
		return breaches, 1
	}
	return nil, 0
}

func formatCISummary(errors, warnings, infos int, breaches []string, failed bool) string {
	status := "PASS"
	if failed {
		status = "FAIL"
	}

	summary := fmt.Sprintf("CI Summary: errors=%d warnings=%d info=%d result=%s", errors, warnings, infos, status)
	if len(breaches) == 0 {
		return summary
	}
	return summary + " | " + strings.Join(breaches, "; ")
}

func checkMissingRefs(nodes []*node.Spec, nodeMap map[string]*node.Spec) []Issue {
	var issues []Issue

	for _, n := range nodes {
		for _, dep := range n.Dependencies {
			if _, exists := nodeMap[dep.Target]; !exists {
				issues = append(issues, Issue{
					Severity:   "error",
					Category:   "missing_ref",
					SourceNode: n.QualifiedID(),
					TargetNode: resolveNodeAlias(dep.Target, nodeMap),
					Message:    fmt.Sprintf("%s.yaml not found", dep.Target),
					Suggestion: fmt.Sprintf("Create the node: gdc node create %s --type %s", resolveNodeAlias(dep.Target, nodeMap), dep.Type),
				})
			}
		}
	}

	return issues
}

func checkHashMismatch(nodes []*node.Spec, nodeMap map[string]*node.Spec) []Issue {
	var issues []Issue

	for _, n := range nodes {
		for _, dep := range n.Dependencies {
			if dep.ContractHash == "" {
				continue
			}

			target, exists := nodeMap[dep.Target]
			if !exists {
				continue
			}

			currentHash := calculateSpecHash(target)
			if dep.ContractHash != currentHash {
				issues = append(issues, Issue{
					Severity:   "warning",
					Category:   "hash_mismatch",
					SourceNode: n.QualifiedID(),
					TargetNode: resolveNodeAlias(dep.Target, nodeMap),
					Message:    fmt.Sprintf("Expected hash %s, got %s", dep.ContractHash, currentHash),
					Suggestion: fmt.Sprintf("%s interface has changed since last sync. Review and update contract_hash.", resolveNodeAlias(dep.Target, nodeMap)),
				})
			}
		}
	}

	return issues
}

func checkCycles(nodes []*node.Spec, nodeMap map[string]*node.Spec) []Issue {
	var issues []Issue

	for _, n := range nodes {
		visited := make(map[string]bool)
		path := []string{n.QualifiedID()}

		if cycle := detectCycle(n.QualifiedID(), nodeMap, visited, path); cycle != nil {
			issues = append(issues, Issue{
				Severity:   "error",
				Category:   "cycle",
				SourceNode: n.QualifiedID(),
				Message:    fmt.Sprintf("Circular dependency: %s", strings.Join(cycle, " -> ")),
				Suggestion: "Refactor to break the circular dependency",
			})
		}
	}

	return issues
}

func detectCycle(nodeID string, nodeMap map[string]*node.Spec, visiting map[string]bool, path []string) []string {
	if visiting[nodeID] {
		for i, p := range path {
			if p == nodeID {
				return append(path[i:], nodeID)
			}
		}
		return path
	}

	visiting[nodeID] = true
	defer func() { visiting[nodeID] = false }()

	spec, exists := nodeMap[nodeID]
	if !exists {
		return nil
	}

	for _, dep := range spec.Dependencies {
		newPath := append(path, dep.Target)
		if cycle := detectCycle(dep.Target, nodeMap, visiting, newPath); cycle != nil {
			return cycle
		}
	}

	return nil
}

func checkOrphans(nodes []*node.Spec, nodeMap map[string]*node.Spec, rules config.OrphanRules, suppress bool, filter string) []Issue {
	if suppress {
		return nil
	}

	var issues []Issue
	referenced := make(map[string]bool)
	for _, n := range nodes {
		for _, dep := range n.Dependencies {
			referenced[resolveNodeAlias(dep.Target, nodeMap)] = true
		}
	}

	for _, n := range nodes {
		if n.Node.Type == "interface" {
			continue
		}
		if shouldIgnoreOrphan(n, rules, filter) {
			continue
		}
		if !referenced[n.QualifiedID()] {
			issues = append(issues, Issue{
				Severity:   "info",
				Category:   "orphan",
				SourceNode: n.QualifiedID(),
				Message:    "Not referenced by any other node",
				Suggestion: "This node may be unused or is an entry point",
			})
		}
	}

	return issues
}

func shouldIgnoreOrphan(spec *node.Spec, rules config.OrphanRules, filter string) bool {
	if spec == nil {
		return false
	}

	nodeID := spec.QualifiedID()
	for _, entryPoint := range rules.EntryPoints {
		if strings.TrimSpace(entryPoint) == nodeID {
			return true
		}
	}
	for _, pattern := range rules.IgnorePatterns {
		if matchesPattern(nodeID, pattern) {
			return true
		}
	}

	filter = strings.TrimSpace(filter)
	if filter == "" {
		return hasAllowedOrphanTag(spec)
	}
	if strings.EqualFold(filter, "entry-point") {
		for _, entryPoint := range rules.EntryPoints {
			if strings.TrimSpace(entryPoint) == nodeID {
				return true
			}
		}
		return hasAllowedOrphanTag(spec)
	}
	return matchesPattern(nodeID, filter) || hasAllowedOrphanTag(spec)
}

func hasAllowedOrphanTag(spec *node.Spec) bool {
	if spec == nil {
		return false
	}
	for _, tag := range spec.Metadata.Tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "entrypoint", "entry-point", "root", "schema", "internal-support":
			return true
		}
	}
	return inferredAllowedOrphan(spec) != ""
}

func inferredAllowedOrphan(spec *node.Spec) string {
	if spec == nil {
		return ""
	}

	id := strings.TrimSpace(spec.Node.ID)
	switch {
	case id == "":
		return ""
	case strings.HasSuffix(id, "Info"):
		return "internal-support"
	case strings.HasPrefix(id, "Extracted"):
		return "internal-support"
	}

	switch id {
	case "NodeRecord", "EdgeRecord", "InterfaceMember", "queryNodeJSON", "codeSyncPlan",
		"goDependencyContext", "cli.DependencyInfo", "cli.SearchResult", "codegen.InterfaceInfo",
		"extract.DependencyInfo", "extract.InterfaceInfo", "search.SearchResult":
		return "internal-support"
	case "Algorithm", "Architecture", "AssemblerConfig", "AssemblyError", "AssemblyResult",
		"Constructor", "Dependency", "DependencyRef", "Edge", "Event", "FormatOptions",
		"FunctionCode", "Interface", "Issue", "LanguageSpec", "LayerRule", "Logic", "Metadata",
		"NodeInfo", "NodeReference", "NodeSpec", "Output", "Parameter", "Project", "Property",
		"Range", "RecoverableError", "Responsibility", "Returns", "SearchOptions", "SourceFile",
		"Spec", "SpecLoadResult", "State", "StateMachine", "Storage", "Symbol", "TestCoverage",
		"TestFile", "TestFileContent", "Throws", "Transition", "Validation":
		return "schema"
	default:
		return ""
	}
}

func matchesPattern(value, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		return false
	}
	return matched
}

func checkSRPViolations(nodes []*node.Spec, threshold int) []Issue {
	var issues []Issue

	if threshold <= 0 {
		threshold = 5
	}

	for _, n := range nodes {
		depCount := len(n.Dependencies)
		nodeThreshold := threshold
		if n.Metadata.SRPThreshold > 0 {
			nodeThreshold = n.Metadata.SRPThreshold
		}

		if depCount > nodeThreshold {
			issues = append(issues, Issue{
				Severity:   "warning",
				Category:   "srp_violation",
				SourceNode: n.Node.ID,
				Message:    fmt.Sprintf("Node has %d dependencies (threshold: %d)", depCount, nodeThreshold),
				Suggestion: "Consider splitting responsibilities into smaller classes",
			})
		}
	}

	return issues
}

func resolveLayerViolationSeverity(configured string, strict bool) string {
	if strict {
		return "error"
	}
	switch strings.ToLower(strings.TrimSpace(configured)) {
	case "error", "info":
		return strings.ToLower(strings.TrimSpace(configured))
	default:
		return "warning"
	}
}

func checkLayerViolations(nodes []*node.Spec, layers []config.LayerRule, severity string) []Issue {
	var issues []Issue

	allowed := make(map[string]map[string]bool)
	for _, layer := range layers {
		allowed[layer.Name] = make(map[string]bool)
		for _, dep := range layer.CanDependOn {
			allowed[layer.Name][dep] = true
		}
	}

	nodeMap := buildSpecLookup(nodes)

	for _, n := range nodes {
		srcLayer := n.Node.Layer
		if srcLayer == "" {
			continue
		}

		for _, dep := range n.Dependencies {
			target, exists := nodeMap[dep.Target]
			if !exists {
				continue
			}

			dstLayer := target.Node.Layer
			if dstLayer == "" {
				continue
			}

			if srcLayer != dstLayer {
				if allowedDeps, ok := allowed[srcLayer]; ok && !allowedDeps[dstLayer] {
					issues = append(issues, Issue{
						Severity:   severity,
						Category:   "layer_violation",
						SourceNode: n.QualifiedID(),
						TargetNode: resolveNodeAlias(dep.Target, nodeMap),
						Message:    fmt.Sprintf("%s layer cannot depend on %s layer", srcLayer, dstLayer),
						Suggestion: "Restructure dependencies to follow layered architecture",
					})
				}
			}
		}
	}

	return issues
}

func checkImplementationConsistency(nodes []*node.Spec, cfg *config.Config, failOnMissing bool) []Issue {
	lang := strings.TrimSpace(cfg.Project.Language)
	if lang == "" {
		return nil
	}

	p, err := parser.GetParser(lang)
	if err != nil {
		return []Issue{{
			Severity: "warning",
			Category: "impl_mismatch",
			Message:  fmt.Sprintf("Implementation verification unavailable for language %s", lang),
		}}
	}

	cache := make(map[string]implVerificationResult)
	var issues []Issue

	for _, spec := range nodes {
		if strings.TrimSpace(spec.Node.FilePath) == "" {
			continue
		}

		resolvedPath := cfg.ResolvePath(spec.Node.FilePath)
		result := verifyNodeImplementation(spec, resolvedPath, lang, p, cache)

		if result.err != nil {
			issues = append(issues, Issue{
				Severity:   "error",
				Category:   "impl_missing",
				SourceNode: spec.Node.ID,
				Message:    result.err.Error(),
				Suggestion: "Update node.file_path or fix the source file, then rerun gdc check --verify-impl",
			})
			continue
		}

		if result.extracted == nil {
			continue
		}

		matched, total, missing := compareSpecToImplementation(spec, result.extracted)
		if total == 0 || len(missing) == 0 {
			continue
		}

		severity := "warning"
		if failOnMissing {
			severity = "error"
		}
		issues = append(issues, Issue{
			Severity:   severity,
			Category:   "impl_mismatch",
			SourceNode: spec.Node.ID,
			Message:    fmt.Sprintf("%d/%d interface members matched (missing: %s)", matched, total, strings.Join(missing, ", ")),
			Suggestion: "Update the implementation or refresh the YAML spec so the public contract matches the code",
		})
	}

	return issues
}

type implVerificationResult struct {
	extracted *parser.ExtractedNode
	err       error
}

func verifyNodeImplementation(spec *node.Spec, resolvedPath, lang string, p parser.Parser, cache map[string]implVerificationResult) implVerificationResult {
	cacheKey := resolvedPath + "::" + spec.Node.ID
	if cached, ok := cache[cacheKey]; ok {
		return cached
	}

	info, err := os.Stat(resolvedPath)
	if err != nil || info.IsDir() {
		result := implVerificationResult{
			err: fmt.Errorf("file_path does not exist for %s: %s", spec.Node.ID, spec.Node.FilePath),
		}
		cache[cacheKey] = result
		return result
	}

	extracted, err := findExtractedNodeInFile(p, resolvedPath, spec.Node.ID)
	if err != nil {
		result := implVerificationResult{
			err: fmt.Errorf("failed to parse %s for %s: %v", spec.Node.FilePath, spec.Node.ID, err),
		}
		cache[cacheKey] = result
		return result
	}

	if extracted == nil {
		content, readErr := os.ReadFile(resolvedPath)
		if readErr == nil && symbolExistsInSource(lang, string(content), spec.Node.ID, spec.Node.Type) {
			result := implVerificationResult{}
			cache[cacheKey] = result
			return result
		}

		result := implVerificationResult{
			err: fmt.Errorf("file_path exists but no %s '%s' found in %s", spec.Node.Type, spec.Node.ID, spec.Node.FilePath),
		}
		cache[cacheKey] = result
		return result
	}

	result := implVerificationResult{extracted: extracted}
	cache[cacheKey] = result
	return result
}

func findExtractedNodeInFile(p parser.Parser, path, nodeID string) (*parser.ExtractedNode, error) {
	if multi, ok := p.(parser.MultiNodeParser); ok {
		extractedNodes, err := multi.ParseFileNodes(path)
		if err != nil {
			return nil, err
		}
		for _, extracted := range extractedNodes {
			if extracted != nil && extracted.ID == nodeID {
				return extracted, nil
			}
		}
		return nil, nil
	}

	extracted, err := p.ParseFile(path)
	if err != nil {
		return nil, err
	}
	if extracted != nil && extracted.ID == nodeID {
		return extracted, nil
	}
	return nil, nil
}

func symbolExistsInSource(lang, content, nodeID, nodeType string) bool {
	patterns := make([]string, 0, 2)

	switch strings.ToLower(lang) {
	case "go", "golang":
		if nodeType == "function" {
			patterns = append(patterns, fmt.Sprintf(`\bfunc\s+(?:\([^)]+\)\s*)?%s\s*\(`, regexp.QuoteMeta(nodeID)))
		} else {
			patterns = append(patterns, fmt.Sprintf(`\btype\s+%s\s+(?:struct|interface)\b`, regexp.QuoteMeta(nodeID)))
		}
	case "csharp", "cs", "c#":
		if nodeType == "function" {
			patterns = append(patterns, fmt.Sprintf(`\b%s\s*\(`, regexp.QuoteMeta(nodeID)))
		} else {
			patterns = append(patterns, fmt.Sprintf(`\b(?:class|interface|enum|record)\s+%s\b`, regexp.QuoteMeta(nodeID)))
		}
	case "typescript", "ts", "javascript", "js":
		if nodeType == "function" {
			patterns = append(patterns, fmt.Sprintf(`\bfunction\s+%s\b`, regexp.QuoteMeta(nodeID)))
		} else {
			patterns = append(patterns, fmt.Sprintf(`\b(?:class|interface|type|enum)\s+%s\b`, regexp.QuoteMeta(nodeID)))
		}
	default:
		patterns = append(patterns, fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(nodeID)))
	}

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err == nil && re.FindStringIndex(content) != nil {
			return true
		}
	}
	return false
}

func compareSpecToImplementation(spec *node.Spec, extracted *parser.ExtractedNode) (matched int, total int, missing []string) {
	if spec == nil || extracted == nil {
		return 0, 0, nil
	}

	methods := make(map[string]parser.ExtractedMethod, len(extracted.Methods))
	for _, method := range extracted.Methods {
		methods[method.Name] = method
	}
	properties := make(map[string]parser.ExtractedProperty, len(extracted.Properties))
	for _, prop := range extracted.Properties {
		properties[prop.Name] = prop
	}
	events := make(map[string]parser.ExtractedEvent, len(extracted.Events))
	for _, event := range extracted.Events {
		events[event.Name] = event
	}

	for _, method := range spec.Interface.Methods {
		total++
		if extractedMethod, ok := methods[method.Name]; ok && signaturesCompatible(method.Signature, extractedMethod.Signature) {
			matched++
			continue
		}
		missing = append(missing, method.Name)
	}

	for _, prop := range spec.Interface.Properties {
		total++
		if extractedProp, ok := properties[prop.Name]; ok && typesCompatible(prop.Type, extractedProp.Type) {
			matched++
			continue
		}
		missing = append(missing, prop.Name)
	}

	for _, event := range spec.Interface.Events {
		total++
		if _, ok := events[event.Name]; ok {
			matched++
			continue
		}
		missing = append(missing, event.Name)
	}

	for _, ctor := range spec.Interface.Constructors {
		total++
		found := false
		for _, extractedCtor := range extracted.Constructors {
			if signaturesCompatible(ctor.Signature, extractedCtor.Signature) {
				found = true
				break
			}
		}
		if found {
			matched++
			continue
		}
		missing = append(missing, "constructor")
	}

	sort.Strings(missing)
	return matched, total, missing
}

func signaturesCompatible(specSig, extractedSig string) bool {
	return normalizeSignature(specSig) == normalizeSignature(extractedSig)
}

func typesCompatible(specType, extractedType string) bool {
	return normalizeSignature(specType) == normalizeSignature(extractedType)
}

func normalizeSignature(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func filterIssues(issues []Issue, category, severity string) []Issue {
	if category == "" && severity == "" {
		return issues
	}

	var filtered []Issue
	for _, issue := range issues {
		if category != "" && issue.Category != category {
			continue
		}
		if severity != "" && issue.Severity != severity {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

func displayIssues(issues []Issue) {
	for _, issue := range issues {
		var prefix string
		switch issue.Severity {
		case "error":
			prefix = color.RedString("[ERROR]")
		case "warning":
			prefix = color.YellowString("[WARNING]")
		case "info":
			prefix = color.CyanString("[INFO]")
		}

		fmt.Printf("%s %s\n", prefix, issue.Category)

		if issue.TargetNode != "" {
			fmt.Printf("  - %s -> %s\n", issue.SourceNode, issue.TargetNode)
		} else if issue.SourceNode != "" {
			fmt.Printf("  - %s\n", issue.SourceNode)
		}

		fmt.Printf("    %s\n", issue.Message)

		if issue.Suggestion != "" && !quiet {
			fmt.Printf("    %s\n", color.HiBlackString(issue.Suggestion))
		}
		fmt.Println()
	}
}

func countBySeverity(issues []Issue, severity string) int {
	count := 0
	for _, issue := range issues {
		if issue.Severity == severity {
			count++
		}
	}
	return count
}

func isWarnOnly(warnOnly []string, category string) bool {
	for _, c := range warnOnly {
		if c == category {
			return true
		}
	}
	return false
}

func isDisabled(disabled []string, category string) bool {
	for _, c := range disabled {
		if c == category {
			return true
		}
	}
	return false
}

// Helper for stats command.
func printStatsTable(headers []string, rows [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetBorder(false)
	table.AppendBulk(rows)
	table.Render()
}
