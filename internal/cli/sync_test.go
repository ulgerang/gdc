package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/parser"
)

func TestBuildCodeSyncPlansQualifiesDuplicateIDsAndRemapsDependencies(t *testing.T) {
	sourceDir := filepath.Join("E:", "repo", "src")
	nodesDir := filepath.Join("E:", "repo", ".gdc", "nodes")

	commandRegistryPath := filepath.Join(sourceDir, "pkg", "command", "registry.go")
	agentMessagePath := filepath.Join(sourceDir, "pkg", "agent", "message.go")
	channelMessagePath := filepath.Join(sourceDir, "pkg", "channel", "message.go")
	commandToolPath := filepath.Join(sourceDir, "pkg", "command", "tool.go")
	channelNotifierPath := filepath.Join(sourceDir, "pkg", "channel", "notifier.go")

	existingNodes := []*node.Spec{
		{
			Node: node.NodeInfo{
				ID:        "Registry",
				Namespace: "command",
				FilePath:  commandRegistryPath,
			},
		},
	}

	extractedNodes := []*parser.ExtractedNode{
		{ID: "Registry", Namespace: "command", FilePath: commandRegistryPath},
		{ID: "Registry", Namespace: "skill", FilePath: filepath.Join(sourceDir, "pkg", "skill", "registry.go")},
		{ID: "Message", Namespace: "agent", FilePath: agentMessagePath},
		{ID: "Message", Namespace: "channel", FilePath: channelMessagePath},
		{
			ID:        "CommandTool",
			Namespace: "command",
			FilePath:  commandToolPath,
			Dependencies: []parser.ExtractedDependency{
				{Target: "Registry", Namespace: "command", Injection: "constructor"},
			},
		},
		{
			ID:        "Notifier",
			Namespace: "channel",
			FilePath:  channelNotifierPath,
			Dependencies: []parser.ExtractedDependency{
				{Target: "Message", Namespace: "channel", Injection: "constructor"},
			},
		},
	}

	plans := buildCodeSyncPlans(sourceDir, nodesDir, existingNodes, extractedNodes)
	planByID := make(map[string]*codeSyncPlan, len(plans))
	for _, plan := range plans {
		planByID[plan.FinalID] = plan
	}

	if _, ok := planByID["command.Registry"]; !ok {
		t.Fatal("expected command.Registry plan to exist")
	}
	if _, ok := planByID["skill.Registry"]; !ok {
		t.Fatal("expected skill.Registry plan to exist")
	}
	if _, ok := planByID["agent.Message"]; !ok {
		t.Fatal("expected agent.Message plan to exist")
	}
	if _, ok := planByID["channel.Message"]; !ok {
		t.Fatal("expected channel.Message plan to exist")
	}

	commandRegistryPlan := planByID["command.Registry"]
	if commandRegistryPlan.ExistingSpec == nil {
		t.Fatal("expected command.Registry to reuse the existing bare Registry spec")
	}
	expectedStalePath := filepath.Join(nodesDir, "Registry.yaml")
	if !sameSyncPath(commandRegistryPlan.StaleSpecPath, expectedStalePath) {
		t.Fatalf("expected stale path %s, got %s", expectedStalePath, commandRegistryPlan.StaleSpecPath)
	}

	commandToolPlan := planByID["CommandTool"]
	if commandToolPlan == nil {
		t.Fatal("expected CommandTool plan to exist")
	}
	if len(commandToolPlan.Extracted.Dependencies) != 1 || commandToolPlan.Extracted.Dependencies[0].Target != "command.Registry" {
		t.Fatalf("expected CommandTool dependency to remap to command.Registry, got %+v", commandToolPlan.Extracted.Dependencies)
	}

	notifierPlan := planByID["Notifier"]
	if notifierPlan == nil {
		t.Fatal("expected Notifier plan to exist")
	}
	if len(notifierPlan.Extracted.Dependencies) != 1 || notifierPlan.Extracted.Dependencies[0].Target != "channel.Message" {
		t.Fatalf("expected Notifier dependency to remap to channel.Message, got %+v", notifierPlan.Extracted.Dependencies)
	}
}

func TestBuildCodeSyncPlansFallsBackToPathPrefixForSameNamespaceCollisions(t *testing.T) {
	sourceDir := filepath.Join("E:", "repo", "src")
	nodesDir := filepath.Join("E:", "repo", ".gdc", "nodes")

	plans := buildCodeSyncPlans(sourceDir, nodesDir, nil, []*parser.ExtractedNode{
		{ID: "Config", Namespace: "main", FilePath: filepath.Join(sourceDir, "cmd", "alpha", "config.go")},
		{ID: "Config", Namespace: "main", FilePath: filepath.Join(sourceDir, "cmd", "beta", "config.go")},
	})

	planByID := make(map[string]bool, len(plans))
	for _, plan := range plans {
		planByID[plan.FinalID] = true
	}

	if !planByID["alpha.Config"] {
		t.Fatal("expected alpha.Config to be generated for duplicate main.Config")
	}
	if !planByID["beta.Config"] {
		t.Fatal("expected beta.Config to be generated for duplicate main.Config")
	}
}

func TestBuildCodeSyncPlansProtectsExistingPortableFilenameDuringScopedSync(t *testing.T) {
	sourceDir := filepath.Join("E:", "repo")
	nodesDir := filepath.Join("E:", "repo", ".gdc", "nodes")
	cliSummaryPath := filepath.Join(sourceDir, "internal", "cli", "scan.go")
	hookSummaryPath := filepath.Join(sourceDir, "internal", "hook", "event.go")

	existingNodes := []*node.Spec{
		{
			Node: node.NodeInfo{
				ID:        "scanSummary",
				Namespace: "cli",
				FilePath:  cliSummaryPath,
			},
		},
	}
	extractedNodes := []*parser.ExtractedNode{
		{
			ID:        "ScanSummary",
			Namespace: "hook",
			FilePath:  hookSummaryPath,
		},
		{
			ID:        "Dispatcher",
			Namespace: "hook",
			FilePath:  filepath.Join(sourceDir, "internal", "hook", "runner.go"),
			Dependencies: []parser.ExtractedDependency{
				{Target: "ScanSummary", Namespace: "hook", Injection: "constructor"},
			},
		},
	}

	plans := buildCodeSyncPlans(sourceDir, nodesDir, existingNodes, extractedNodes)
	planByID := make(map[string]*codeSyncPlan, len(plans))
	for _, plan := range plans {
		planByID[plan.FinalID] = plan
	}

	summaryPlan := planByID["hook.ScanSummary"]
	if summaryPlan == nil {
		t.Fatalf("expected hook.ScanSummary plan, got %v", planByID)
	}
	if summaryPlan.ExistingSpec != nil {
		t.Fatal("expected scoped hook.ScanSummary to leave the existing cli.scanSummary spec untouched")
	}
	if summaryPlan.StaleSpecPath != "" {
		t.Fatalf("expected no stale path for unrelated existing node, got %s", summaryPlan.StaleSpecPath)
	}

	dispatcherPlan := planByID["Dispatcher"]
	if dispatcherPlan == nil {
		t.Fatal("expected Dispatcher plan to exist")
	}
	if len(dispatcherPlan.Extracted.Dependencies) != 1 || dispatcherPlan.Extracted.Dependencies[0].Target != "hook.ScanSummary" {
		t.Fatalf("expected dependency to remap to hook.ScanSummary, got %+v", dispatcherPlan.Extracted.Dependencies)
	}
}

func TestBuildCodeSyncPlansReusesExistingQualifiedNodeDuringScopedSync(t *testing.T) {
	sourceDir := filepath.Join("E:", "repo")
	nodesDir := filepath.Join("E:", "repo", ".gdc", "nodes")
	hookSummaryPath := filepath.Join(sourceDir, "internal", "hook", "event.go")
	existing := &node.Spec{
		Node: node.NodeInfo{
			ID:        "hook.ScanSummary",
			Namespace: "hook",
			FilePath:  hookSummaryPath,
		},
	}

	plans := buildCodeSyncPlans(sourceDir, nodesDir, []*node.Spec{existing}, []*parser.ExtractedNode{
		{
			ID:        "ScanSummary",
			Namespace: "hook",
			FilePath:  hookSummaryPath,
		},
	})

	if len(plans) != 1 {
		t.Fatalf("expected one plan, got %d", len(plans))
	}
	if plans[0].FinalID != "hook.ScanSummary" {
		t.Fatalf("expected existing qualified ID to remain stable, got %s", plans[0].FinalID)
	}
	if plans[0].ExistingSpec != existing {
		t.Fatal("expected existing qualified spec to be reused")
	}
}

func TestSyncScopeFiltersFilesAndSymbols(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Storage: config.Storage{
			NodesDir: ".gdc/nodes",
		},
	}

	scope := newSyncScope(cfg,
		[]string{"src/services/user_service.go"},
		nil,
		[]string{"UserService"},
	)

	paths := []string{
		filepath.Join(projectRoot, "src", "services", "user_service.go"),
		filepath.Join(projectRoot, "src", "services", "auth_service.go"),
	}
	filteredPaths := filterPathsByScope(paths, scope)
	if len(filteredPaths) != 1 || !sameSyncPath(filteredPaths[0], paths[0]) {
		t.Fatalf("expected only user_service.go to remain, got %v", filteredPaths)
	}

	extracted := []*parser.ExtractedNode{
		{ID: "UserService", Namespace: "services", FilePath: paths[0]},
		{ID: "AuthService", Namespace: "services", FilePath: paths[1]},
	}
	filteredNodes := filterExtractedNodesByScope(extracted, scope)
	if len(filteredNodes) != 1 || filteredNodes[0].ID != "UserService" {
		t.Fatalf("expected only UserService to remain, got %+v", filteredNodes)
	}

	configScope := newSyncScope(cfg, nil, nil, []string{"Config"})
	configNodes := filterExtractedNodesByScope([]*parser.ExtractedNode{
		{ID: "Config", Namespace: "config", FilePath: paths[0]},
		{ID: "getEnv", Namespace: "config", FilePath: paths[0]},
		{ID: "parseFloat", Namespace: "config", FilePath: paths[0]},
	}, configScope)
	if len(configNodes) != 1 || configNodes[0].ID != "Config" {
		t.Fatalf("exact Config symbol scope must not expand to namespace helpers, got %+v", configNodes)
	}
}

func TestSyncScopeMatchesNodesByQualifiedNameAndPath(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Storage: config.Storage{
			NodesDir: ".gdc/nodes",
		},
	}

	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:        "PlayerController",
			Namespace: "Game.Controllers",
			FilePath:  filepath.Join(projectRoot, "src", "Controllers", "PlayerController.cs"),
		},
	}

	qualifiedScope := newSyncScope(cfg, nil, nil, []string{"Game.Controllers.PlayerController"})
	if !qualifiedScope.matchesNode(spec) {
		t.Fatal("expected qualified-name scope to match node")
	}

	pathScope := newSyncScope(cfg, []string{"src/Controllers/PlayerController.cs"}, nil, nil)
	if !pathScope.matchesNode(spec) {
		t.Fatal("expected file scope to match node")
	}
}

func TestCollectExplicitSourceScopeFiles_IncludesFilesOutsideSourceDir(t *testing.T) {
	projectRoot := t.TempDir()
	pkgDir := filepath.Join(projectRoot, "pkg")
	internalDir := filepath.Join(projectRoot, "internal", "app")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}
	if err := os.MkdirAll(internalDir, 0o755); err != nil {
		t.Fatalf("failed to create internal dir: %v", err)
	}

	pkgFile := filepath.Join(pkgDir, "service.go")
	internalFile := filepath.Join(internalDir, "boundary_contracts.go")
	for _, path := range []string{pkgFile, internalFile} {
		if err := os.WriteFile(path, []byte("package sample\n"), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Project: config.Project{
			Language:  "go",
			SourceDir: "./pkg",
		},
	}

	prevFiles := syncFiles
	prevDirs := syncDirs
	t.Cleanup(func() {
		syncFiles = prevFiles
		syncDirs = prevDirs
	})
	syncFiles = []string{"internal/app/boundary_contracts.go"}
	syncDirs = nil

	files, err := collectExplicitSourceScopeFiles(cfg, filepath.Join(projectRoot, "pkg"), []string{".go"})
	if err != nil {
		t.Fatalf("collect explicit source scope: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 explicit scoped file, got %d (%v)", len(files), files)
	}
	if !sameSyncPath(files[0], internalFile) {
		t.Fatalf("expected %s, got %s", internalFile, files[0])
	}
}

func TestCollectExplicitSourceScopeFilesReportsTraversalFailure(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := &config.Config{ProjectRoot: projectRoot}

	prevFiles := syncFiles
	prevDirs := syncDirs
	t.Cleanup(func() {
		syncFiles = prevFiles
		syncDirs = prevDirs
	})
	syncFiles = nil
	syncDirs = []string{"missing-external-source"}

	_, err := collectExplicitSourceScopeFiles(cfg, filepath.Join(projectRoot, "pkg"), []string{".go"})
	if err == nil || !strings.Contains(err.Error(), "missing-external-source") {
		t.Fatalf("expected explicit directory traversal error, got %v", err)
	}
}

func TestApplyCodeSyncMetadataSetsOriginAndOptionalStatus(t *testing.T) {
	now := time.Date(2026, time.March, 15, 9, 0, 0, 0, time.UTC)
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:       "Agent",
			FilePath: "pkg/agent/agent.go",
		},
		Metadata: node.Metadata{
			Status: "draft",
		},
	}

	applyCodeSyncMetadata(spec, nil, true, now)

	if spec.Metadata.Origin != "code_extracted" {
		t.Fatalf("expected origin to be code_extracted, got %q", spec.Metadata.Origin)
	}
	if spec.Metadata.ExtractedAt != "2026-03-15" {
		t.Fatalf("expected extracted_at to be set, got %q", spec.Metadata.ExtractedAt)
	}
	if spec.Metadata.Status != "implemented" {
		t.Fatalf("expected status to be promoted to implemented, got %q", spec.Metadata.Status)
	}
}

func TestApplyCodeSyncMetadataPreservesExistingOrigin(t *testing.T) {
	now := time.Date(2026, time.March, 15, 9, 0, 0, 0, time.UTC)
	existing := &node.Spec{
		Metadata: node.Metadata{
			Origin: "hand_authored",
		},
	}
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:       "Agent",
			FilePath: "pkg/agent/agent.go",
		},
		Metadata: node.Metadata{
			Status: "draft",
			Origin: "hand_authored",
		},
	}

	applyCodeSyncMetadata(spec, existing, true, now)

	if spec.Metadata.Origin != "hand_authored" {
		t.Fatalf("expected origin to be preserved, got %q", spec.Metadata.Origin)
	}
	if spec.Metadata.ExtractedAt != "" {
		t.Fatalf("expected extracted_at to remain empty for hand_authored spec, got %q", spec.Metadata.ExtractedAt)
	}
	if spec.Metadata.Status != "implemented" {
		t.Fatalf("expected status to be promoted to implemented, got %q", spec.Metadata.Status)
	}
}

func TestCodeSyncHonorsMergeFlagWhenBuildingSpecs(t *testing.T) {
	prevMerge := syncMerge
	t.Cleanup(func() {
		syncMerge = prevMerge
	})

	existing := &node.Spec{
		Responsibility: node.Responsibility{Summary: "Existing summary"},
		Metadata:       node.Metadata{Status: "draft"},
		Interface: node.Interface{
			Methods: []node.Method{
				{Name: "Execute", Signature: "Execute(old) error", Description: "Old description"},
			},
		},
	}
	extracted := &parser.ExtractedNode{
		ID:   "Agent",
		Type: "class",
		Methods: []parser.ExtractedMethod{
			{Name: "Execute", Signature: "Execute() error", IsPublic: true},
		},
	}

	syncMerge = true
	merged := extracted.ToNodeSpec(existing)
	if merged.Responsibility.Summary != "Existing summary" || merged.Interface.Methods[0].Description != "Old description" {
		t.Fatalf("expected merge mode to preserve authored content, got %+v", merged)
	}

	syncMerge = false
	replaced := extracted.ToNodeSpec(nil)
	if replaced.Responsibility.Summary != "" || replaced.Interface.Methods[0].Description != "" {
		t.Fatalf("expected replace mode to drop authored content, got %+v", replaced)
	}
}

func TestCodeSyncMergePreservesCuratedImplementationContract(t *testing.T) {
	projectRoot := t.TempDir()
	sourceDir := filepath.Join(projectRoot, "internal", "httpapi")
	nodesDir := filepath.Join(projectRoot, ".gdc", "nodes")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.MkdirAll(nodesDir, 0o755); err != nil {
		t.Fatalf("create nodes dir: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "auth_handler.go")
	source := `package httpapi

import "context"

type LoginResult struct{}

type AuthService interface {
	LoginGuest(ctx context.Context, playerID string) (LoginResult, error)
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	existing := &node.Spec{
		SchemaVersion: "1.1",
		Node: node.NodeInfo{
			ID:        "AuthService",
			Type:      "interface",
			Layer:     "application",
			Namespace: "httpapi",
			FilePath:  "internal/httpapi/auth_handler.go",
		},
		Responsibility: node.Responsibility{
			Summary:    "Bound authentication use cases.",
			Invariants: []string{"Google availability is optional."},
		},
		Interface: node.Interface{Methods: []node.Method{{
			Name:        "LoginGuest",
			Signature:   "LoginGuest(ctx context.Context, playerID string, restoreToken string) (LoginResult, error)",
			Description: "Creates or resumes a guest session.",
			Parameters: []node.Parameter{
				{Name: "ctx", Type: "context.Context", Description: "Request context."},
				{Name: "playerID", Type: "string", Description: "Existing opaque player ID."},
				{Name: "restoreToken", Type: "string", Description: "Obsolete restore input."},
			},
			Returns:        node.Returns{Type: "(LoginResult, error)", Description: "Issued credentials."},
			Preconditions:  []string{"Strict request decoding succeeded."},
			Postconditions: []string{"A successful login returns credentials."},
			Exported:       true,
		}}},
		ImplementationContract: &node.ImplementationContract{
			Status:      "ready",
			Lifecycle:   []string{"Injected before route registration."},
			Constraints: []string{"Provider outages cannot block continuity."},
			Acceptance: []node.AcceptanceScenario{{
				ID: "AUTH-CONTINUITY-001", Given: "An established player.", When: "Google is unavailable.", Then: []string{"Login succeeds."},
			}},
		},
		Metadata: node.Metadata{Status: "implemented", Origin: "hand_authored"},
	}
	if err := node.Save(filepath.Join(nodesDir, "AuthService.yaml"), existing); err != nil {
		t.Fatalf("save existing spec: %v", err)
	}

	previousSource := syncSource
	previousDryRun := syncDryRun
	previousMerge := syncMerge
	previousAutoStatus := syncAutoStatus
	previousConflictLog := syncConflictLog
	previousLogMapping := syncLogMapping
	previousTiming := syncTiming
	previousProfile := syncProfile
	previousQuiet := quiet
	t.Cleanup(func() {
		syncSource = previousSource
		syncDryRun = previousDryRun
		syncMerge = previousMerge
		syncAutoStatus = previousAutoStatus
		syncConflictLog = previousConflictLog
		syncLogMapping = previousLogMapping
		syncTiming = previousTiming
		syncProfile = previousProfile
		quiet = previousQuiet
	})
	syncSource = ""
	syncDryRun = false
	syncMerge = true
	syncAutoStatus = false
	syncConflictLog = ""
	syncLogMapping = ""
	syncTiming = false
	syncProfile = false
	quiet = true

	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Project: config.Project{
			Language:  "go",
			SourceDir: "internal/httpapi",
		},
		Storage: config.Storage{NodesDir: ".gdc/nodes"},
	}
	scope := newSyncScope(cfg, []string{"internal/httpapi/auth_handler.go"}, nil, []string{"AuthService"})
	if err := runSyncFromCode(cfg, nodesDir, scope); err != nil {
		t.Fatalf("code sync: %v", err)
	}

	merged, err := node.Load(filepath.Join(nodesDir, "AuthService.yaml"))
	if err != nil {
		t.Fatalf("load merged spec: %v", err)
	}
	if merged.SchemaVersion != "1.1" || merged.ImplementationContract == nil || len(merged.ImplementationContract.Acceptance) != 1 {
		t.Fatalf("curated implementation contract was not preserved: schema=%q contract=%#v", merged.SchemaVersion, merged.ImplementationContract)
	}
	if len(merged.Interface.Methods) != 1 || len(merged.Interface.Methods[0].Parameters) != 2 {
		t.Fatalf("code-owned parameter shape was not refreshed: %#v", merged.Interface.Methods)
	}
	method := merged.Interface.Methods[0]
	if strings.Contains(method.Signature, "restoreToken") || method.Parameters[1].Description != "Existing opaque player ID." {
		t.Fatalf("stale parameter survived or authored metadata was lost: %#v", method)
	}
	if len(method.Preconditions) != 1 || len(method.Postconditions) != 1 || method.Returns.Description == "" {
		t.Fatalf("authored method behavior was lost: %#v", method)
	}
}

func TestShouldShowDocWarningHonorsFlagsAndThreshold(t *testing.T) {
	prevNoDoc := syncNoDocWarnings
	prevThreshold := syncDocThreshold
	t.Cleanup(func() {
		syncNoDocWarnings = prevNoDoc
		syncDocThreshold = prevThreshold
	})

	syncNoDocWarnings = false
	syncDocThreshold = 3
	if shouldShowDocWarning(2) {
		t.Fatal("expected warning below threshold to be suppressed")
	}
	if !shouldShowDocWarning(3) {
		t.Fatal("expected warning at threshold to be shown")
	}

	syncNoDocWarnings = true
	if shouldShowDocWarning(10) {
		t.Fatal("expected no-doc-warnings to suppress warnings")
	}
}

func TestFormatSyncMappingLineIncludesRenamedNodeContext(t *testing.T) {
	plan := &codeSyncPlan{
		BareID:  "Registry",
		FinalID: "command.Registry",
		Extracted: &parser.ExtractedNode{
			ID:       "Registry",
			FilePath: filepath.Join("E:", "repo", "src", "pkg", "command", "registry.go"),
		},
	}

	line := formatSyncMappingLine(plan, filepath.Join("E:", "repo", ".gdc", "nodes", "command.Registry.yaml"), "create")
	if !strings.Contains(line, "command.Registry") || !strings.Contains(line, "bare: Registry") {
		t.Fatalf("expected mapping line to include remapped id context, got %q", line)
	}
}

func TestWriteSyncMappingLogWritesFileWhenEnabled(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := &config.Config{ProjectRoot: projectRoot}

	prevLogMapping := syncLogMapping
	t.Cleanup(func() {
		syncLogMapping = prevLogMapping
	})
	syncLogMapping = ".gdc/sync-mapping.log"

	if err := writeSyncMappingLog(cfg, []string{"a.go -> A"}); err != nil {
		t.Fatalf("failed to write mapping log: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(projectRoot, ".gdc", "sync-mapping.log"))
	if err != nil {
		t.Fatalf("failed to read mapping log: %v", err)
	}
	if !strings.Contains(string(data), "a.go -> A") {
		t.Fatalf("expected mapping content in log, got %q", string(data))
	}
}

func TestCollectCodeSyncConflictsSummarizesDrift(t *testing.T) {
	plans := []*codeSyncPlan{
		{
			FinalID: "Agent",
			ExistingSpec: &node.Spec{
				Node: node.NodeInfo{ID: "Agent"},
				Interface: node.Interface{
					Methods: []node.Method{
						{Name: "Execute", Signature: "Execute(old) error"},
					},
				},
				Dependencies: []node.Dependency{
					{Target: "Logger"},
				},
			},
			Extracted: &parser.ExtractedNode{
				ID: "Agent",
				Methods: []parser.ExtractedMethod{
					{Name: "Execute", Signature: "Execute() error"},
				},
				Dependencies: []parser.ExtractedDependency{
					{Target: "Tracer"},
				},
			},
		},
	}

	lines := collectCodeSyncConflicts(plans, map[string]string{})
	if len(lines) != 1 {
		t.Fatalf("expected 1 conflict summary, got %d (%v)", len(lines), lines)
	}
	if !strings.Contains(lines[0], "method drift=1") || !strings.Contains(lines[0], "deps +/-=1/1") {
		t.Fatalf("expected method and dependency drift summary, got %q", lines[0])
	}
}

func TestWriteSyncProfileReportWritesJSONWhenEnabled(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := &config.Config{ProjectRoot: projectRoot}

	prevProfile := syncProfile
	prevOutput := syncProfileOutput
	t.Cleanup(func() {
		syncProfile = prevProfile
		syncProfileOutput = prevOutput
	})

	syncProfile = true
	syncProfileOutput = ".gdc/profile.json"

	report := syncProfileReport{
		Direction:  "code",
		StartedAt:  time.Date(2026, time.March, 15, 9, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2026, time.March, 15, 9, 0, 2, 0, time.UTC),
		Phases: map[string]time.Duration{
			"scan": time.Second,
		},
	}

	if err := writeSyncProfileReport(cfg, report); err != nil {
		t.Fatalf("failed to write sync profile report: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(projectRoot, ".gdc", "profile.json"))
	if err != nil {
		t.Fatalf("failed to read sync profile report: %v", err)
	}
	if !strings.Contains(string(data), `"direction": "code"`) || !strings.Contains(string(data), `"scan": 1000`) {
		t.Fatalf("expected sync profile JSON content, got %q", string(data))
	}
}

func TestCanonicalizeSpecDependenciesNormalizesGenericTargets(t *testing.T) {
	spec := &node.Spec{
		Node: node.NodeInfo{
			ID:        "OrderService",
			Namespace: "Example.Services",
		},
		Dependencies: []node.Dependency{
			{Target: "ILogger<OrderService>"},
			{Target: "Example.Repositories.IRepository<Order>"},
		},
	}

	aliasMap := map[string]string{
		"ILogger":                          "ILogger",
		"Example.Repositories.IRepository": "Example.Repositories.IRepository",
	}

	canonicalizeSpecDependencies(spec, aliasMap)

	if spec.Dependencies[0].Target != "ILogger" {
		t.Fatalf("expected ILogger<OrderService> to canonicalize to ILogger, got %q", spec.Dependencies[0].Target)
	}
	if spec.Dependencies[1].Target != "Example.Repositories.IRepository" {
		t.Fatalf("expected namespaced generic target to canonicalize, got %q", spec.Dependencies[1].Target)
	}
}

func TestMakeCodeSyncPathsPortableUsesProjectRelativePaths(t *testing.T) {
	projectRoot := t.TempDir()
	sourceDir := filepath.Join(projectRoot, "crates", "gateway")
	sourcePath := filepath.Join(sourceDir, "src", "gateway.rs")
	existing := []*node.Spec{
		{Node: node.NodeInfo{ID: "Gateway", FilePath: sourcePath}},
	}
	extracted := []*parser.ExtractedNode{
		{ID: "Gateway", FilePath: sourcePath},
	}

	portableSourceDir := makeCodeSyncPathsPortable(projectRoot, sourceDir, existing, extracted)

	if want := "crates/gateway"; portableSourceDir != want {
		t.Fatalf("expected portable source directory %q, got %q", want, portableSourceDir)
	}
	if want := "crates/gateway/src/gateway.rs"; existing[0].Node.FilePath != want {
		t.Fatalf("expected existing node path %q, got %q", want, existing[0].Node.FilePath)
	}
	if want := "crates/gateway/src/gateway.rs"; extracted[0].FilePath != want {
		t.Fatalf("expected extracted node path %q, got %q", want, extracted[0].FilePath)
	}
}

func TestRunSyncFromCodeStoresPortableNodeFilePath(t *testing.T) {
	projectRoot := t.TempDir()
	sourceDir := filepath.Join(projectRoot, "crates", "gateway")
	nodesDir := filepath.Join(projectRoot, ".gdc", "nodes")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}
	if err := os.MkdirAll(nodesDir, 0o755); err != nil {
		t.Fatalf("failed to create nodes directory: %v", err)
	}
	sourcePath := filepath.Join(sourceDir, "gateway.go")
	if err := os.WriteFile(sourcePath, []byte("package gateway\n\ntype Gateway struct{}\n"), 0o644); err != nil {
		t.Fatalf("failed to write source fixture: %v", err)
	}

	previousSource := syncSource
	previousDryRun := syncDryRun
	previousMerge := syncMerge
	previousAutoStatus := syncAutoStatus
	previousConflictLog := syncConflictLog
	previousLogMapping := syncLogMapping
	previousTiming := syncTiming
	previousProfile := syncProfile
	previousQuiet := quiet
	t.Cleanup(func() {
		syncSource = previousSource
		syncDryRun = previousDryRun
		syncMerge = previousMerge
		syncAutoStatus = previousAutoStatus
		syncConflictLog = previousConflictLog
		syncLogMapping = previousLogMapping
		syncTiming = previousTiming
		syncProfile = previousProfile
		quiet = previousQuiet
	})
	syncSource = ""
	syncDryRun = false
	syncMerge = true
	syncAutoStatus = false
	syncConflictLog = ""
	syncLogMapping = ""
	syncTiming = false
	syncProfile = false
	quiet = true

	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Project: config.Project{
			Language:  "go",
			SourceDir: "crates/gateway",
		},
	}
	if err := runSyncFromCode(cfg, nodesDir, newSyncScope(cfg, nil, nil, nil)); err != nil {
		t.Fatalf("code sync failed: %v", err)
	}

	spec, err := node.Load(filepath.Join(nodesDir, "Gateway.yaml"))
	if err != nil {
		t.Fatalf("failed to load generated node: %v", err)
	}
	if want := "crates/gateway/gateway.go"; spec.Node.FilePath != want {
		t.Fatalf("expected generated node path %q, got %q", want, spec.Node.FilePath)
	}
	if filepath.IsAbs(filepath.FromSlash(spec.Node.FilePath)) {
		t.Fatalf("generated node path must be portable, got absolute path %q", spec.Node.FilePath)
	}
}

func TestProjectRelativeSyncPathPreservesExternalAbsolutePath(t *testing.T) {
	projectRoot := t.TempDir()
	externalRoot := t.TempDir()
	externalPath := filepath.Join(externalRoot, "shared", "contract.go")

	got := projectRelativeSyncPath(projectRoot, externalPath)

	if !filepath.IsAbs(filepath.FromSlash(got)) {
		t.Fatalf("expected external path to remain absolute, got %q", got)
	}
}

func TestIsTestSourceFileRecognizesPythonConventions(t *testing.T) {
	for _, path := range []string{"tests/test_service.py", "service_test.py"} {
		if !isTestSourceFile(path) {
			t.Fatalf("expected %s to be treated as a test source file", path)
		}
	}
	if isTestSourceFile("service.py") {
		t.Fatal("production Python source must not be treated as a test")
	}
}

func TestIsTestSourceFileRecognizesGDScriptConventions(t *testing.T) {
	for _, path := range []string{"tests/test_slot_controller.gd", "slot_controller_test.gd"} {
		if !isTestSourceFile(path) {
			t.Fatalf("expected %s to be treated as a GDScript test source file", path)
		}
	}
	if isTestSourceFile("slot_controller.gd") {
		t.Fatal("production GDScript source must not be treated as a test")
	}
}
