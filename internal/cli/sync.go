package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/db"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/parser"
	"github.com/spf13/cobra"
)

var (
	syncDryRun        bool
	syncForce         bool
	syncDirection     string
	syncSource        string
	syncAutoStatus    bool
	syncMerge         bool
	syncStrategy      string
	syncConflictLog   string
	syncNoDocWarnings bool
	syncDocThreshold  int
	syncLogMapping    string
	syncTiming        bool
	syncProfile       bool
	syncProfileOutput string
	syncFiles         []string
	syncDirs          []string
	syncSymbols       []string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize YAML specs with database or code",
	Long: `Synchronize node specifications with the SQLite database.

Direction modes:
  yaml  - Sync YAML specs to database (default)
  code  - Extract interface from source code and update YAML specs

This command scans all YAML files and updates the database index
for fast querying. With --direction code, it extracts interfaces
from source files and updates the corresponding YAML specs.

Examples:
  $ gdc sync                           # Sync YAML → DB
  $ gdc sync --dry-run                 # Preview changes
  $ gdc sync --force                   # Force full resync
  $ gdc sync --direction code          # Extract from code → YAML
  $ gdc sync --direction code --source src/  # Specify source directory`,
	RunE: runSync,
}

func init() {
	syncCmd.Long = `Synchronize node specifications with the SQLite database.

Direction modes:
  yaml  - Sync YAML specs to the database index (default)
  code  - Extract interfaces from source code and update YAML specs
  both  - Run code sync and then refresh the database index
  spec  - Reserved for future spec-to-code generation

Examples:
  $ gdc sync
  $ gdc sync --dry-run
  $ gdc sync --direction code --source src/
  $ gdc sync --direction both --strategy merge
  $ gdc sync --direction both --conflict-log .gdc/conflicts.log
  $ gdc sync --timing --profile --profile-output .gdc/sync-profile.json`
	syncCmd.Flags().BoolVarP(&syncDryRun, "dry-run", "n", false, "preview changes without applying")
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "force full resync")
	syncCmd.Flags().StringVarP(&syncDirection, "direction", "d", "yaml", "sync direction (yaml, code, both, spec)")
	syncCmd.Flags().StringVarP(&syncSource, "source", "s", "", "source directory for code extraction")
	syncCmd.Flags().BoolVar(&syncAutoStatus, "auto-status", false, "set code-synced nodes with file paths to implemented")
	syncCmd.Flags().BoolVar(&syncMerge, "merge", true, "merge extracted signatures into existing specs, preserving authored descriptions")
	syncCmd.Flags().StringVar(&syncStrategy, "strategy", "code-first", "sync strategy for --direction both (code-first, spec-first, merge)")
	syncCmd.Flags().StringVar(&syncConflictLog, "conflict-log", "", "write drift/conflict summaries detected during code sync to a log file")
	syncCmd.Flags().BoolVar(&syncNoDocWarnings, "no-doc-warnings", false, "suppress missing documentation warnings during code sync")
	syncCmd.Flags().IntVar(&syncDocThreshold, "doc-threshold", 1, "only show documentation warnings when missing member count reaches this threshold")
	syncCmd.Flags().StringVar(&syncLogMapping, "log-mapping", "", "write source-to-node mapping details to a log file")
	syncCmd.Flags().BoolVar(&syncTiming, "timing", false, "print sync timing metrics")
	syncCmd.Flags().BoolVar(&syncProfile, "profile", false, "write a JSON sync profile report")
	syncCmd.Flags().StringVar(&syncProfileOutput, "profile-output", "", "output path for --profile (default: .gdc/sync-profile.json)")
	syncCmd.Flags().StringSliceVar(&syncFiles, "files", nil, "limit sync to specific files")
	syncCmd.Flags().StringSliceVar(&syncDirs, "dirs", nil, "limit sync to specific directories")
	syncCmd.Flags().StringSliceVar(&syncSymbols, "symbols", nil, "limit sync to specific symbols")
}

func runSync(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.NodesDir()
	dbPath := cfg.DatabasePath()
	scope := newSyncScope(cfg, syncFiles, syncDirs, syncSymbols)

	switch strings.ToLower(strings.TrimSpace(syncDirection)) {
	case "code":
		return runSyncFromCode(cfg, nodesDir, scope)
	case "both":
		return runSyncBoth(cfg, nodesDir, dbPath, scope)
	case "yaml", "":
		return runSyncToDB(cfg, nodesDir, dbPath, scope)
	case "spec":
		return fmt.Errorf("sync --direction spec is not implemented yet")
	default:
		return fmt.Errorf("unknown sync direction: %s", syncDirection)
	}
}

func runSyncToDB(cfg *config.Config, nodesDir, dbPath string, scope *syncScope) error {
	startedAt := time.Now()
	if !quiet {
		fmt.Println("Scanning for changes...")
	}

	// Load all nodes
	nodes, err := loadAllNodes(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}
	if scope.active() {
		nodes = filterSpecsByScope(nodes, scope)
	}

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Ensure schema exists
	if err := database.InitSchema(); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Get existing nodes from DB
	existingNodes, err := database.GetAllNodes()
	if err != nil {
		return fmt.Errorf("failed to get existing nodes: %w", err)
	}

	existingMap := make(map[string]*db.NodeRecord)
	for _, n := range existingNodes {
		if scope.active() && !scope.matchesStoredNode(n) {
			continue
		}
		existingMap[n.QualifiedID] = n
	}

	// Track changes
	var created, updated, deleted int

	// Process each node
	for _, spec := range nodes {
		specHash := calculateSpecHash(spec)
		qualifiedID := spec.QualifiedID()
		existing, exists := existingMap[qualifiedID]

		if !exists {
			// New node
			if syncDryRun {
				color.Green("  + Added: %s", spec.Node.ID)
			} else {
				if err := syncNodeToDB(database, spec, specHash); err != nil {
					printWarning("Failed to sync %s: %v", qualifiedID, err)
					continue
				}
				color.Green("  + Added: %s", qualifiedID)
			}
			created++
		} else if syncForce || existing.SpecHash != specHash {
			// Modified node
			if syncDryRun {
				color.Yellow("  ⟳ Modified: %s (spec_hash changed)", qualifiedID)
			} else {
				if err := syncNodeToDB(database, spec, specHash); err != nil {
					printWarning("Failed to sync %s: %v", qualifiedID, err)
					continue
				}
				color.Yellow("  ⟳ Modified: %s", qualifiedID)
			}
			updated++
			delete(existingMap, qualifiedID)
		} else {
			delete(existingMap, qualifiedID)
		}
	}

	// Remove deleted nodes
	for id := range existingMap {
		if syncDryRun {
			color.Red("  - Deleted: %s", id)
		} else {
			if err := database.DeleteNode(id); err != nil {
				printWarning("Failed to delete %s: %v", id, err)
				continue
			}
			color.Red("  - Deleted: %s", id)
		}
		deleted++
	}

	// Summary
	fmt.Println()
	if created == 0 && updated == 0 && deleted == 0 {
		printInfo("No changes detected")
	} else {
		action := "Synced"
		if syncDryRun {
			action = "Would sync"
		}
		printSuccess("%s: %d created, %d updated, %d deleted",
			action, created, updated, deleted)
	}

	report := syncProfileReport{
		Direction:    "yaml",
		Strategy:     normalizedSyncStrategy(syncStrategy),
		StartedAt:    startedAt,
		FinishedAt:   time.Now(),
		NodesScanned: len(nodes),
		Created:      created,
		Updated:      updated,
		Deleted:      deleted,
		Phases: map[string]time.Duration{
			"total": time.Since(startedAt),
		},
	}
	printSyncTiming(report)
	if err := writeSyncProfileReport(cfg, report); err != nil && !quiet {
		printWarning("Failed to write sync profile: %v", err)
	}

	return nil
}

func syncNodeToDB(database *db.Database, spec *node.Spec, specHash string) error {
	qualifiedID := spec.QualifiedID()

	// Update node record
	record := &db.NodeRecord{
		QualifiedID:    qualifiedID,
		ID:             spec.Node.ID,
		Type:           spec.Node.Type,
		Layer:          spec.Node.Layer,
		Namespace:      spec.Node.Namespace,
		SpecPath:       getSpecPath(spec),
		ImplPath:       spec.Node.FilePath,
		Responsibility: spec.Responsibility.Summary,
		Status:         spec.Metadata.Status,
		SpecHash:       specHash,
		ImplHash:       spec.Metadata.ImplHash,
		UpdatedAt:      time.Now(),
	}

	if err := database.UpsertNode(record); err != nil {
		return err
	}

	// Update edges
	if err := database.DeleteEdgesFrom(qualifiedID); err != nil {
		return err
	}

	for _, dep := range spec.Dependencies {
		edge := &db.EdgeRecord{
			FromNode:       qualifiedID,
			ToNode:         dep.Target,
			DependencyType: dep.Type,
			InjectionType:  dep.Injection,
			IsOptional:     dep.Optional,
			ContractHash:   dep.ContractHash,
			UsageSummary:   dep.Usage,
		}
		if err := database.InsertEdge(edge); err != nil {
			return err
		}
	}

	// Update interface members
	if err := database.DeleteInterfaceMembers(qualifiedID); err != nil {
		return err
	}

	for _, ctor := range spec.Interface.Constructors {
		member := &db.InterfaceMember{
			NodeID:      qualifiedID,
			MemberType:  "constructor",
			Name:        "constructor",
			Signature:   ctor.Signature,
			Description: ctor.Description,
		}
		if err := database.InsertInterfaceMember(member); err != nil {
			return err
		}
	}

	for _, method := range spec.Interface.Methods {
		member := &db.InterfaceMember{
			NodeID:      qualifiedID,
			MemberType:  "method",
			Name:        method.Name,
			Signature:   method.Signature,
			Description: method.Description,
			ReturnType:  method.Returns.Type,
		}
		if err := database.InsertInterfaceMember(member); err != nil {
			return err
		}
	}

	for _, prop := range spec.Interface.Properties {
		member := &db.InterfaceMember{
			NodeID:      qualifiedID,
			MemberType:  "property",
			Name:        prop.Name,
			Signature:   fmt.Sprintf("%s { %s; }", prop.Type, prop.Access),
			Description: prop.Description,
			ReturnType:  prop.Type,
		}
		if err := database.InsertInterfaceMember(member); err != nil {
			return err
		}
	}

	// Update tags
	if err := database.DeleteTags(qualifiedID); err != nil {
		return err
	}

	for _, tag := range spec.Metadata.Tags {
		if err := database.InsertTag(qualifiedID, tag); err != nil {
			return err
		}
	}

	return nil
}

func calculateSpecHash(spec *node.Spec) string {
	// Hash based on interface signature (not documentation)
	var content string

	for _, ctor := range spec.Interface.Constructors {
		content += ctor.Signature + "|"
	}
	for _, method := range spec.Interface.Methods {
		content += method.Signature + "|"
	}
	for _, prop := range spec.Interface.Properties {
		content += fmt.Sprintf("%s:%s:%s|", prop.Name, prop.Type, prop.Access)
	}
	for _, event := range spec.Interface.Events {
		content += event.Signature + "|"
	}

	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:4]) // First 8 hex chars
}

func getSpecPath(spec *node.Spec) string {
	if spec != nil && strings.TrimSpace(spec.SourcePath) != "" {
		return spec.SourcePath
	}
	cfg, _ := config.Load("")
	nodesDir := ".gdc/nodes"
	if cfg != nil && cfg.Storage.NodesDir != "" {
		nodesDir = cfg.Storage.NodesDir
	}
	if spec == nil {
		return nodesDir
	}
	return filepath.Join(nodesDir, spec.QualifiedID()+".yaml")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type syncProfileReport struct {
	Direction     string                   `json:"direction"`
	Strategy      string                   `json:"strategy,omitempty"`
	StartedAt     time.Time                `json:"started_at"`
	FinishedAt    time.Time                `json:"finished_at"`
	NodesScanned  int                      `json:"nodes_scanned,omitempty"`
	SourceFiles   int                      `json:"source_files,omitempty"`
	Extracted     int                      `json:"extracted,omitempty"`
	Created       int                      `json:"created"`
	Updated       int                      `json:"updated"`
	Deleted       int                      `json:"deleted"`
	Skipped       int                      `json:"skipped,omitempty"`
	Conflicts     int                      `json:"conflicts,omitempty"`
	Phases        map[string]time.Duration `json:"-"`
	PhaseMillis   map[string]int64         `json:"phases_ms,omitempty"`
	DurationMilli int64                    `json:"duration_ms"`
}

func runSyncBoth(cfg *config.Config, nodesDir, dbPath string, scope *syncScope) error {
	strategy := normalizedSyncStrategy(syncStrategy)
	switch strategy {
	case "code-first", "spec-first", "merge":
	default:
		return fmt.Errorf("unknown sync strategy: %s", syncStrategy)
	}

	originalMerge := syncMerge
	if strategy == "merge" {
		syncMerge = true
		defer func() { syncMerge = originalMerge }()
	}

	switch strategy {
	case "spec-first":
		if err := runSyncToDB(cfg, nodesDir, dbPath, scope); err != nil {
			return err
		}
		if err := runSyncFromCode(cfg, nodesDir, scope); err != nil {
			return err
		}
		return runSyncToDB(cfg, nodesDir, dbPath, scope)
	default:
		if err := runSyncFromCode(cfg, nodesDir, scope); err != nil {
			return err
		}
		return runSyncToDB(cfg, nodesDir, dbPath, scope)
	}
}

func normalizedSyncStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "", "code-first", "code-wins", "code":
		return "code-first"
	case "spec-first", "spec-wins", "spec":
		return "spec-first"
	case "merge", "merge-prompt":
		return "merge"
	default:
		return strings.ToLower(strings.TrimSpace(strategy))
	}
}

// runSyncFromCode extracts interfaces from source code and updates YAML specs
func runSyncFromCode(cfg *config.Config, nodesDir string, scope *syncScope) error {
	startedAt := time.Now()
	phaseStarted := startedAt
	if !quiet {
		fmt.Println("Extracting interfaces from source code...")
	}

	// Determine language and source directory
	lang := cfg.Project.Language
	if lang == "" {
		return fmt.Errorf("project.language not set in config")
	}

	sourceDir := syncSource
	if sourceDir == "" {
		sourceDir = cfg.Project.SourceDir
	}
	if sourceDir == "" {
		return fmt.Errorf("source directory not specified. Use --source or set project.source_dir in config")
	}

	// Resolve relative path to absolute path based on project root
	sourceDir = cfg.ResolvePath(sourceDir)

	// Get the appropriate parser
	p, err := parser.GetParser(lang)
	if err != nil {
		return fmt.Errorf("failed to get parser: %w", err)
	}

	// Find source files
	var sourceFiles []string
	var extensions []string
	switch strings.ToLower(lang) {
	case "go", "golang":
		extensions = []string{".go"}
	case "csharp", "cs", "c#":
		extensions = []string{".cs"}
	case "typescript", "ts":
		extensions = []string{".ts", ".tsx"}
	default:
		return fmt.Errorf("unsupported language: %s", lang)
	}

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip common non-source directories
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "bin" || name == "obj" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, validExt := range extensions {
			if ext == validExt {
				// Skip test files
				if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".spec.ts") {
					break
				}
				sourceFiles = append(sourceFiles, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan source directory: %w", err)
	}
	scanDuration := time.Since(phaseStarted)

	sourceFiles = append(sourceFiles, collectExplicitSourceScopeFiles(cfg, sourceDir, extensions)...)
	sourceFiles = dedupeStrings(sourceFiles)

	if len(sourceFiles) == 0 {
		printWarning("No source files found in %s", sourceDir)
		return nil
	}

	if scope.active() {
		sourceFiles = filterPathsByScope(sourceFiles, scope)
		if len(sourceFiles) == 0 {
			printWarning("No source files matched the requested sync scope")
			return nil
		}
	}

	if !quiet {
		fmt.Printf("Found %d source files\n\n", len(sourceFiles))
	}

	// Load existing specs for merging
	existingNodes, _ := loadAllNodes(nodesDir)

	var skipped int
	allExtracted := make([]*parser.ExtractedNode, 0)
	phaseStarted = time.Now()

	// Process each source file
	for _, filePath := range sourceFiles {
		extractedNodes, err := extractNodesFromSourceFile(p, filePath)
		if err != nil {
			if !quiet {
				printWarning("Failed to parse %s: %v", filePath, err)
			}
			continue
		}

		if len(extractedNodes) == 0 {
			skipped++
			continue
		}

		allExtracted = append(allExtracted, extractedNodes...)
	}
	parseDuration := time.Since(phaseStarted)

	if scope.active() {
		allExtracted = filterExtractedNodesByScope(allExtracted, scope)
		if len(allExtracted) == 0 {
			printWarning("No symbols matched the requested sync scope")
			return nil
		}
	}

	phaseStarted = time.Now()
	plans := buildCodeSyncPlans(sourceDir, nodesDir, existingNodes, allExtracted)
	dependencyAliasMap := buildCanonicalDependencyAliasMap(existingNodes, plans)
	conflictLines := collectCodeSyncConflicts(plans, dependencyAliasMap)
	planDuration := time.Since(phaseStarted)

	var created, updated, deleted int
	deletedPaths := make(map[string]bool)
	var mappingLines []string
	phaseStarted = time.Now()

	for _, plan := range plans {
		baseSpec := plan.ExistingSpec
		if !syncMerge {
			baseSpec = nil
		}
		newSpec := plan.Extracted.ToNodeSpec(baseSpec)
		newSpec.Node.ID = plan.FinalID
		newSpec.Node.FilePath = plan.Extracted.FilePath
		canonicalizeSpecDependencies(newSpec, dependencyAliasMap)
		applyCodeSyncMetadata(newSpec, baseSpec, syncAutoStatus, time.Now())

		specPath := filepath.Join(nodesDir, plan.FinalID+".yaml")
		exists := plan.ExistingSpec != nil
		actionLabel := "create"
		if exists {
			actionLabel = "update"
		}
		mappingLines = append(mappingLines, formatSyncMappingLine(plan, specPath, actionLabel))
		if verbose && !quiet {
			fmt.Printf("     map: %s\n", formatSyncMappingPreview(plan, actionLabel))
		}

		if syncDryRun {
			if exists {
				color.Yellow("  ⟳ Would update: %s → %s", plan.Extracted.FilePath, specPath)
				updated++
			} else {
				color.Green("  + Would create: %s → %s", plan.Extracted.FilePath, specPath)
				created++
			}
		} else {
			if err := node.Save(specPath, newSpec); err != nil {
				printWarning("Failed to save %s: %v", specPath, err)
				continue
			}

			if exists {
				color.Yellow("  ⟳ Updated: %s", plan.FinalID)
				updated++
			} else {
				color.Green("  + Created: %s", plan.FinalID)
				created++
			}
		}

		if stalePath := plan.StaleSpecPath; stalePath != "" {
			normalizedStale := normalizeSyncPath(stalePath)
			if !deletedPaths[normalizedStale] && !sameSyncPath(stalePath, specPath) {
				if syncDryRun {
					color.Red("  - Would delete stale: %s", stalePath)
					deleted++
					deletedPaths[normalizedStale] = true
				} else {
					if err := os.Remove(stalePath); err != nil && !os.IsNotExist(err) {
						printWarning("Failed to delete stale spec %s: %v", stalePath, err)
					} else {
						color.Red("  - Deleted stale: %s", stalePath)
						deleted++
						deletedPaths[normalizedStale] = true
					}
				}
			}
		}

		missingCount := countMissingDescriptions(newSpec)
		if !shouldShowDocWarning(missingCount) {
			missingCount = 0
		}
		if missingCount > 0 && !quiet {
			color.HiBlack("     └─ ⚠️ %d members need documentation", missingCount)
		}
	}

	writeDuration := time.Since(phaseStarted)

	if err := writeSyncMappingLog(cfg, mappingLines); err != nil && !quiet {
		printWarning("Failed to write sync mapping log: %v", err)
	}
	if err := writeSyncConflictLog(cfg, conflictLines); err != nil && !quiet {
		printWarning("Failed to write sync conflict log: %v", err)
	}

	// Summary
	fmt.Println()
	if created == 0 && updated == 0 && deleted == 0 {
		printInfo("No changes detected")
	} else {
		action := "Synced"
		if syncDryRun {
			action = "Would sync"
		}
		printSuccess("%s: %d created, %d updated, %d deleted, %d skipped", action, created, updated, deleted, skipped)
	}

	report := syncProfileReport{
		Direction:    "code",
		Strategy:     normalizedSyncStrategy(syncStrategy),
		StartedAt:    startedAt,
		FinishedAt:   time.Now(),
		NodesScanned: len(plans),
		SourceFiles:  len(sourceFiles),
		Extracted:    len(allExtracted),
		Created:      created,
		Updated:      updated,
		Deleted:      deleted,
		Skipped:      skipped,
		Conflicts:    len(conflictLines),
		Phases: map[string]time.Duration{
			"scan":  scanDuration,
			"parse": parseDuration,
			"plan":  planDuration,
			"write": writeDuration,
			"total": time.Since(startedAt),
		},
	}
	printSyncTiming(report)
	if err := writeSyncProfileReport(cfg, report); err != nil && !quiet {
		printWarning("Failed to write sync profile: %v", err)
	}

	return nil
}

func collectExplicitSourceScopeFiles(cfg *config.Config, sourceDir string, extensions []string) []string {
	files := make([]string, 0)
	seen := make(map[string]bool)
	addFile := func(path string) {
		normalized := normalizeComparablePath(path)
		if normalized == "" || seen[normalized] {
			return
		}
		if !hasAllowedSourceExtension(path, extensions) {
			return
		}
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".spec.ts") {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		seen[normalized] = true
		files = append(files, path)
	}

	sourceDir = normalizeComparablePath(sourceDir)
	for _, requested := range syncFiles {
		resolved := cfg.ResolvePath(requested)
		if normalized := normalizeComparablePath(resolved); normalized != "" && normalized != sourceDir && !pathWithinScope(normalized, sourceDir) {
			addFile(resolved)
		}
	}

	for _, requested := range syncDirs {
		resolved := cfg.ResolvePath(requested)
		normalized := normalizeComparablePath(resolved)
		if normalized == "" || normalized == sourceDir || pathWithinScope(normalized, sourceDir) {
			continue
		}
		_ = filepath.Walk(resolved, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			addFile(path)
			return nil
		})
	}

	return files
}

func hasAllowedSourceExtension(path string, extensions []string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range extensions {
		if ext == allowed {
			return true
		}
	}
	return false
}

func extractNodesFromSourceFile(p parser.Parser, filePath string) ([]*parser.ExtractedNode, error) {
	if multi, ok := p.(parser.MultiNodeParser); ok {
		return multi.ParseFileNodes(filePath)
	}

	extracted, err := p.ParseFile(filePath)
	if err != nil {
		return nil, err
	}
	if extracted == nil || extracted.ID == "" {
		return nil, nil
	}
	return []*parser.ExtractedNode{extracted}, nil
}

type codeSyncPlan struct {
	Extracted     *parser.ExtractedNode
	BareID        string
	FinalID       string
	ExistingSpec  *node.Spec
	StaleSpecPath string
}

func buildCodeSyncPlans(sourceDir, nodesDir string, existingNodes []*node.Spec, extractedNodes []*parser.ExtractedNode) []*codeSyncPlan {
	existingByID := make(map[string]*node.Spec, len(existingNodes))
	existingBySource := make(map[string][]*node.Spec)
	for _, spec := range existingNodes {
		existingByID[spec.Node.ID] = spec
		if spec.Node.FilePath != "" {
			key := normalizeSyncPath(spec.Node.FilePath)
			existingBySource[key] = append(existingBySource[key], spec)
		}
	}

	resolvedIDs, duplicateCounts := resolveExtractedNodeIDs(sourceDir, extractedNodes)
	dependencyLookup := buildDependencyLookup(extractedNodes, resolvedIDs)

	plans := make([]*codeSyncPlan, 0, len(extractedNodes))
	for _, extracted := range extractedNodes {
		if extracted == nil || extracted.ID == "" {
			continue
		}

		bareID := extracted.ID
		cloned := cloneExtractedNode(extracted)
		cloned.ID = resolvedIDs[extractedNodeKey(extracted.FilePath, bareID)]
		remapExtractedDependencies(cloned, duplicateCounts, dependencyLookup)

		existingSpec, staleSpecPath := findExistingSpecForPlan(existingByID, existingBySource, nodesDir, cloned, bareID)
		plans = append(plans, &codeSyncPlan{
			Extracted:     cloned,
			BareID:        bareID,
			FinalID:       cloned.ID,
			ExistingSpec:  existingSpec,
			StaleSpecPath: staleSpecPath,
		})
	}

	return plans
}

func resolveExtractedNodeIDs(sourceDir string, extractedNodes []*parser.ExtractedNode) (map[string]string, map[string]int) {
	duplicateCounts := make(map[string]int)
	groups := make(map[string][]*parser.ExtractedNode)

	for _, extracted := range extractedNodes {
		if extracted == nil || extracted.ID == "" {
			continue
		}
		duplicateCounts[extracted.ID]++
		groups[extracted.ID] = append(groups[extracted.ID], extracted)
	}

	resolved := make(map[string]string, len(extractedNodes))
	for bareID, group := range groups {
		if len(group) == 1 {
			resolved[extractedNodeKey(group[0].FilePath, bareID)] = bareID
			continue
		}

		if canUseNamespaceQualifiedIDs(group) {
			for _, extracted := range group {
				resolved[extractedNodeKey(extracted.FilePath, bareID)] = extracted.Namespace + "." + bareID
			}
			continue
		}

		prefixes := buildUniquePathPrefixes(sourceDir, group)
		for _, extracted := range group {
			key := extractedNodeKey(extracted.FilePath, bareID)
			prefix := prefixes[key]
			if prefix == "" {
				prefix = fallbackIDPrefix(extracted)
			}
			resolved[key] = prefix + "." + bareID
		}
	}

	return resolved, duplicateCounts
}

func canUseNamespaceQualifiedIDs(group []*parser.ExtractedNode) bool {
	seen := make(map[string]bool, len(group))
	for _, extracted := range group {
		namespace := strings.TrimSpace(extracted.Namespace)
		if namespace == "" || seen[namespace] {
			return false
		}
		seen[namespace] = true
	}
	return true
}

func buildUniquePathPrefixes(sourceDir string, group []*parser.ExtractedNode) map[string]string {
	segmentsByNode := make(map[string][]string, len(group))
	maxSegments := 0

	for _, extracted := range group {
		key := extractedNodeKey(extracted.FilePath, extracted.ID)
		segments := relativePathSegments(sourceDir, extracted.FilePath)
		if len(segments) == 0 {
			segments = []string{fallbackIDPrefix(extracted)}
		}
		segmentsByNode[key] = segments
		if len(segments) > maxSegments {
			maxSegments = len(segments)
		}
	}

	prefixes := make(map[string]string, len(group))
	for suffixLen := 1; suffixLen <= maxSegments; suffixLen++ {
		candidateOwners := make(map[string]string, len(group))
		collisions := make(map[string]bool)
		for nodeKey, segments := range segmentsByNode {
			candidate := joinPathSuffix(segments, suffixLen)
			if owner, exists := candidateOwners[candidate]; exists && owner != nodeKey {
				collisions[candidate] = true
			} else {
				candidateOwners[candidate] = nodeKey
			}
			prefixes[nodeKey] = candidate
		}

		if len(collisions) == 0 {
			return prefixes
		}
	}

	for _, extracted := range group {
		key := extractedNodeKey(extracted.FilePath, extracted.ID)
		segments := append([]string{}, segmentsByNode[key]...)
		fileStem := strings.TrimSuffix(filepath.Base(extracted.FilePath), filepath.Ext(extracted.FilePath))
		segments = append(segments, fileStem)
		prefixes[key] = strings.Join(segments, ".")
	}

	return prefixes
}

func relativePathSegments(sourceDir, filePath string) []string {
	relPath, err := filepath.Rel(sourceDir, filePath)
	if err != nil {
		relPath = filePath
	}
	relDir := filepath.Dir(relPath)
	if relDir == "." || relDir == "" {
		return nil
	}

	parts := strings.Split(filepath.ToSlash(relDir), "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		segments = append(segments, part)
	}
	return segments
}

func joinPathSuffix(segments []string, suffixLen int) string {
	if len(segments) == 0 {
		return ""
	}
	if suffixLen > len(segments) {
		suffixLen = len(segments)
	}
	return strings.Join(segments[len(segments)-suffixLen:], ".")
}

func fallbackIDPrefix(extracted *parser.ExtractedNode) string {
	if extracted != nil {
		if namespace := strings.TrimSpace(extracted.Namespace); namespace != "" {
			return namespace
		}
		if filePath := strings.TrimSpace(extracted.FilePath); filePath != "" {
			return strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		}
	}
	return "node"
}

func buildDependencyLookup(extractedNodes []*parser.ExtractedNode, resolvedIDs map[string]string) map[string]string {
	lookup := make(map[string]string)
	ambiguous := make(map[string]bool)

	for _, extracted := range extractedNodes {
		if extracted == nil || extracted.ID == "" || strings.TrimSpace(extracted.Namespace) == "" {
			continue
		}

		key := dependencyLookupKey(extracted.Namespace, extracted.ID)
		resolvedID := resolvedIDs[extractedNodeKey(extracted.FilePath, extracted.ID)]
		if existing, exists := lookup[key]; exists && existing != resolvedID {
			ambiguous[key] = true
			continue
		}
		lookup[key] = resolvedID
	}

	for key := range ambiguous {
		delete(lookup, key)
	}

	return lookup
}

func remapExtractedDependencies(extracted *parser.ExtractedNode, duplicateCounts map[string]int, dependencyLookup map[string]string) {
	if extracted == nil {
		return
	}

	for i := range extracted.Dependencies {
		dep := &extracted.Dependencies[i]
		if dep.Target == "" || duplicateCounts[dep.Target] <= 1 {
			continue
		}

		namespace := dep.Namespace
		if namespace == "" {
			namespace = extracted.Namespace
		}
		if namespace == "" {
			continue
		}

		if resolvedID, ok := dependencyLookup[dependencyLookupKey(namespace, dep.Target)]; ok {
			dep.Target = resolvedID
		}
	}
}

func findExistingSpecForPlan(existingByID map[string]*node.Spec, existingBySource map[string][]*node.Spec, nodesDir string, extracted *parser.ExtractedNode, bareID string) (*node.Spec, string) {
	if extracted == nil {
		return nil, ""
	}

	finalID := extracted.ID
	existingSpec := existingByID[finalID]
	staleSpecPath := ""
	if finalID == bareID {
		return existingSpec, staleSpecPath
	}

	for _, spec := range existingBySource[normalizeSyncPath(extracted.FilePath)] {
		if !existingSpecMatchesExtracted(spec, extracted, bareID) {
			continue
		}
		if existingSpec == nil {
			existingSpec = spec
		}
		staleSpecPath = filepath.Join(nodesDir, spec.Node.ID+".yaml")
		break
	}

	return existingSpec, staleSpecPath
}

func existingSpecMatchesExtracted(spec *node.Spec, extracted *parser.ExtractedNode, bareID string) bool {
	if spec == nil || extracted == nil || spec.Node.ID != bareID {
		return false
	}
	if spec.Node.FilePath != "" && extracted.FilePath != "" && !sameSyncPath(spec.Node.FilePath, extracted.FilePath) {
		return false
	}
	if spec.Node.Namespace != "" && extracted.Namespace != "" && spec.Node.Namespace != extracted.Namespace {
		return false
	}
	return true
}

func cloneExtractedNode(src *parser.ExtractedNode) *parser.ExtractedNode {
	if src == nil {
		return nil
	}

	dst := *src
	dst.Attributes = append([]string(nil), src.Attributes...)
	dst.Constructors = make([]parser.ExtractedConstructor, len(src.Constructors))
	for i, ctor := range src.Constructors {
		dst.Constructors[i] = ctor
		dst.Constructors[i].Parameters = append([]parser.ExtractedParameter(nil), ctor.Parameters...)
	}
	dst.Methods = make([]parser.ExtractedMethod, len(src.Methods))
	for i, method := range src.Methods {
		dst.Methods[i] = method
		dst.Methods[i].Parameters = append([]parser.ExtractedParameter(nil), method.Parameters...)
		dst.Methods[i].Attributes = append([]string(nil), method.Attributes...)
	}
	dst.Properties = append([]parser.ExtractedProperty(nil), src.Properties...)
	dst.Events = append([]parser.ExtractedEvent(nil), src.Events...)
	dst.Dependencies = append([]parser.ExtractedDependency(nil), src.Dependencies...)

	return &dst
}

func applyCodeSyncMetadata(spec *node.Spec, existing *node.Spec, autoStatus bool, now time.Time) {
	if spec == nil {
		return
	}

	if existing == nil && strings.TrimSpace(spec.Metadata.Origin) == "" {
		spec.Metadata.Origin = "code_extracted"
	}
	if strings.TrimSpace(spec.Metadata.Origin) == "code_extracted" {
		spec.Metadata.ExtractedAt = now.Format("2006-01-02")
	}
	if autoStatus && spec.Node.FilePath != "" && shouldPromoteCodeSyncedStatus(spec.Metadata.Status) {
		spec.Metadata.Status = "implemented"
	}
}

func shouldShowDocWarning(missingCount int) bool {
	if syncNoDocWarnings {
		return false
	}

	threshold := syncDocThreshold
	if threshold <= 0 {
		threshold = 1
	}

	return missingCount >= threshold
}

func formatSyncMappingPreview(plan *codeSyncPlan, actionLabel string) string {
	if plan == nil || plan.Extracted == nil {
		return ""
	}

	source := filepath.Base(plan.Extracted.FilePath)
	if source == "" {
		source = plan.Extracted.FilePath
	}

	if plan.FinalID == plan.BareID {
		return fmt.Sprintf("%s -> %s (%s)", source, plan.FinalID, actionLabel)
	}
	return fmt.Sprintf("%s -> %s [from %s] (%s)", source, plan.FinalID, plan.BareID, actionLabel)
}

func formatSyncMappingLine(plan *codeSyncPlan, specPath, actionLabel string) string {
	if plan == nil || plan.Extracted == nil {
		return ""
	}

	source := filepath.ToSlash(plan.Extracted.FilePath)
	specTarget := filepath.ToSlash(specPath)
	if plan.FinalID == plan.BareID {
		return fmt.Sprintf("%s -> %s (%s) [%s]", source, plan.FinalID, actionLabel, specTarget)
	}
	return fmt.Sprintf("%s -> %s (bare: %s, %s) [%s]", source, plan.FinalID, plan.BareID, actionLabel, specTarget)
}

func writeSyncMappingLog(cfg *config.Config, lines []string) error {
	if strings.TrimSpace(syncLogMapping) == "" || len(lines) == 0 {
		return nil
	}

	targetPath := cfg.ResolvePath(syncLogMapping)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(targetPath, []byte(content), 0o644)
}

func collectCodeSyncConflicts(plans []*codeSyncPlan, dependencyAliasMap map[string]string) []string {
	lines := make([]string, 0)
	for _, plan := range plans {
		if plan == nil || plan.ExistingSpec == nil || plan.Extracted == nil {
			continue
		}

		extracted := cloneExtractedNode(plan.Extracted)
		for i := range extracted.Dependencies {
			if canonical, ok := dependencyAliasMap[strings.TrimSpace(extracted.Dependencies[i].Target)]; ok {
				extracted.Dependencies[i].Target = canonical
			}
		}

		report := buildDriftReport(plan.ExistingSpec, extracted)
		if report.isEmpty() {
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", normalizedSyncStrategy(syncStrategy), formatSyncConflictSummary(plan.FinalID, report)))
	}
	return lines
}

func formatSyncConflictSummary(nodeID string, report driftReport) string {
	parts := make([]string, 0)
	if len(report.MethodMismatches) > 0 {
		parts = append(parts, fmt.Sprintf("method drift=%d", len(report.MethodMismatches)))
	}
	if len(report.PropertyMismatches) > 0 {
		parts = append(parts, fmt.Sprintf("property drift=%d", len(report.PropertyMismatches)))
	}
	if len(report.EventMismatches) > 0 {
		parts = append(parts, fmt.Sprintf("event drift=%d", len(report.EventMismatches)))
	}
	if len(report.MissingMethods)+len(report.ExtraMethods) > 0 {
		parts = append(parts, fmt.Sprintf("methods +/-=%d/%d", len(report.MissingMethods), len(report.ExtraMethods)))
	}
	if len(report.MissingProperties)+len(report.ExtraProperties) > 0 {
		parts = append(parts, fmt.Sprintf("properties +/-=%d/%d", len(report.MissingProperties), len(report.ExtraProperties)))
	}
	if len(report.MissingEvents)+len(report.ExtraEvents) > 0 {
		parts = append(parts, fmt.Sprintf("events +/-=%d/%d", len(report.MissingEvents), len(report.ExtraEvents)))
	}
	if len(report.MissingConstructors)+len(report.ExtraConstructors) > 0 {
		parts = append(parts, fmt.Sprintf("constructors +/-=%d/%d", len(report.MissingConstructors), len(report.ExtraConstructors)))
	}
	if len(report.MissingDeps)+len(report.ExtraDeps) > 0 {
		parts = append(parts, fmt.Sprintf("deps +/-=%d/%d", len(report.MissingDeps), len(report.ExtraDeps)))
	}
	if len(parts) == 0 {
		parts = append(parts, "signature drift detected")
	}
	return fmt.Sprintf("%s: %s", nodeID, strings.Join(parts, ", "))
}

func writeSyncConflictLog(cfg *config.Config, lines []string) error {
	if strings.TrimSpace(syncConflictLog) == "" || len(lines) == 0 {
		return nil
	}

	targetPath := cfg.ResolvePath(syncConflictLog)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(targetPath, []byte(content), 0o644)
}

func printSyncTiming(report syncProfileReport) {
	if !syncTiming || quiet {
		return
	}

	total := report.FinishedAt.Sub(report.StartedAt)
	if total < 0 {
		total = 0
	}
	rate := 0.0
	if total > 0 && report.NodesScanned > 0 {
		rate = float64(report.NodesScanned) / total.Seconds()
	}

	fmt.Printf("Timing: %s sync completed in %.1fs", report.Direction, total.Seconds())
	if rate > 0 {
		fmt.Printf(" (%.1f nodes/sec)", rate)
	}
	fmt.Println()
	if len(report.Phases) > 0 {
		order := []string{"scan", "parse", "plan", "write", "total"}
		parts := make([]string, 0, len(order))
		for _, key := range order {
			if duration, ok := report.Phases[key]; ok {
				parts = append(parts, fmt.Sprintf("%s: %.1fs", strings.Title(key), duration.Seconds()))
			}
		}
		if len(parts) > 0 {
			fmt.Printf("  %s\n", strings.Join(parts, " | "))
		}
	}
}

func writeSyncProfileReport(cfg *config.Config, report syncProfileReport) error {
	if !syncProfile {
		return nil
	}

	report.DurationMilli = report.FinishedAt.Sub(report.StartedAt).Milliseconds()
	if len(report.Phases) > 0 {
		report.PhaseMillis = make(map[string]int64, len(report.Phases))
		for phase, duration := range report.Phases {
			report.PhaseMillis[phase] = duration.Milliseconds()
		}
	}

	targetPath := strings.TrimSpace(syncProfileOutput)
	if targetPath == "" {
		targetPath = ".gdc/sync-profile.json"
	}
	targetPath = cfg.ResolvePath(targetPath)

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(targetPath, data, 0o644)
}

func shouldPromoteCodeSyncedStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "", "draft", "specified":
		return true
	default:
		return false
	}
}

func extractedNodeKey(filePath, bareID string) string {
	return normalizeSyncPath(filePath) + "::" + bareID
}

func dependencyLookupKey(namespace, bareID string) string {
	return strings.TrimSpace(namespace) + "::" + bareID
}

func buildCanonicalDependencyAliasMap(existingNodes []*node.Spec, plans []*codeSyncPlan) map[string]string {
	counts := make(map[string]int)
	owners := make(map[string]string)
	addAlias := func(alias, canonical string) {
		alias = strings.TrimSpace(alias)
		canonical = strings.TrimSpace(canonical)
		if alias == "" || canonical == "" {
			return
		}
		counts[alias]++
		if _, exists := owners[alias]; !exists {
			owners[alias] = canonical
		}
	}

	for _, spec := range existingNodes {
		if spec == nil {
			continue
		}
		canonical := spec.QualifiedID()
		addAlias(canonical, canonical)
		addAlias(spec.Node.ID, canonical)
	}

	for _, plan := range plans {
		if plan == nil {
			continue
		}
		addAlias(plan.FinalID, plan.FinalID)
		addAlias(plan.BareID, plan.FinalID)
		if plan.Extracted != nil && strings.TrimSpace(plan.Extracted.Namespace) != "" {
			addAlias(plan.Extracted.Namespace+"."+plan.BareID, plan.FinalID)
		}
	}

	result := make(map[string]string)
	for alias, canonical := range owners {
		if counts[alias] == 1 {
			result[alias] = canonical
		}
	}
	return result
}

func canonicalizeSpecDependencies(spec *node.Spec, aliasMap map[string]string) {
	if spec == nil {
		return
	}
	for i := range spec.Dependencies {
		target := strings.TrimSpace(spec.Dependencies[i].Target)
		if canonical, ok := aliasMap[target]; ok {
			spec.Dependencies[i].Target = canonical
		}
	}
}

func normalizeSyncPath(path string) string {
	return strings.ToLower(filepath.Clean(path))
}

func sameSyncPath(a, b string) bool {
	return normalizeSyncPath(a) == normalizeSyncPath(b)
}

// countMissingDescriptions counts members without descriptions
func countMissingDescriptions(spec *node.Spec) int {
	count := 0

	for _, ctor := range spec.Interface.Constructors {
		if strings.TrimSpace(ctor.Description) == "" {
			count++
		}
	}
	for _, method := range spec.Interface.Methods {
		if strings.TrimSpace(method.Description) == "" {
			count++
		}
	}
	for _, prop := range spec.Interface.Properties {
		if strings.TrimSpace(prop.Description) == "" {
			count++
		}
	}
	for _, event := range spec.Interface.Events {
		if strings.TrimSpace(event.Description) == "" {
			count++
		}
	}

	return count
}

func filterSpecsByScope(nodes []*node.Spec, scope *syncScope) []*node.Spec {
	if scope == nil || !scope.active() {
		return nodes
	}

	filtered := make([]*node.Spec, 0, len(nodes))
	for _, spec := range nodes {
		if scope.matchesNode(spec) {
			filtered = append(filtered, spec)
		}
	}
	return filtered
}

func filterPathsByScope(paths []string, scope *syncScope) []string {
	if scope == nil || !scope.active() || !scope.hasFileScope() {
		return paths
	}

	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if scope.matchesSourceFile(path) {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func filterExtractedNodesByScope(extractedNodes []*parser.ExtractedNode, scope *syncScope) []*parser.ExtractedNode {
	if scope == nil || !scope.active() {
		return extractedNodes
	}

	filtered := make([]*parser.ExtractedNode, 0, len(extractedNodes))
	for _, extracted := range extractedNodes {
		fileMatched := !scope.hasFileScope() || scope.matchesSourceFile(extracted.FilePath)
		symbolMatched := !scope.hasSymbolScope() || scope.matchesExtractedNode(extracted)
		if fileMatched && symbolMatched {
			filtered = append(filtered, extracted)
		}
	}
	return filtered
}
