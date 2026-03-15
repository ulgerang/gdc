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

	files := collectExplicitSourceScopeFiles(cfg, filepath.Join(projectRoot, "pkg"), []string{".go"})
	if len(files) != 1 {
		t.Fatalf("expected 1 explicit scoped file, got %d (%v)", len(files), files)
	}
	if !sameSyncPath(files[0], internalFile) {
		t.Fatalf("expected %s, got %s", internalFile, files[0])
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
