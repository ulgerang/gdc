package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/db"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	nodeType  string
	nodeLayer string
	nodeForce bool
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage node specifications",
	Long:  `Create, delete, and rename node specifications.`,
}

var nodeCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new node specification",
	Long: `Create a new node specification file.

Examples:
  $ gdc node create PlayerController
  $ gdc node create IInputManager --type interface
  $ gdc node create GameService --type service --layer application`,
	Args: cobra.ExactArgs(1),
	RunE: runNodeCreate,
}

var nodeDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a node specification",
	Long: `Delete a node specification file.

Example:
  $ gdc node delete OldController`,
	Args: cobra.ExactArgs(1),
	RunE: runNodeDelete,
}

var nodeRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a node",
	Long: `Rename a node and update all references and the derived graph index.

Example:
  $ gdc node rename PlayerController CharacterController`,
	Args: cobra.ExactArgs(2),
	RunE: runNodeRename,
}

func init() {
	nodeCreateCmd.Flags().StringVarP(&nodeType, "type", "t", "class",
		"node type (class, interface, module, service, enum)")
	nodeCreateCmd.Flags().StringVarP(&nodeLayer, "layer", "l", "application",
		"architecture layer (domain, application, infrastructure, presentation)")
	nodeDeleteCmd.Flags().BoolVarP(&nodeForce, "force", "f", false,
		"delete referenced nodes and remove their dependency references")

	nodeCmd.AddCommand(nodeCreateCmd)
	nodeCmd.AddCommand(nodeDeleteCmd)
	nodeCmd.AddCommand(nodeRenameCmd)
}

func runNodeCreate(cmd *cobra.Command, args []string) error {
	nodeName := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config (is GDC initialized?): %w", err)
	}

	// Determine file path
	nodesDir := cfg.NodesDir()
	filePath := filepath.Join(nodesDir, nodeName+".yaml")

	// Check if already exists
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("node %s already exists at %s", nodeName, filePath)
	}

	// Create node spec
	spec := node.Spec{
		SchemaVersion: "1.0",
		Node: node.NodeInfo{
			ID:    nodeName,
			Type:  nodeType,
			Layer: nodeLayer,
		},
		Responsibility: node.Responsibility{
			Summary: fmt.Sprintf("%s의 책임을 정의하세요", nodeName),
		},
		Interface: node.Interface{
			Methods: []node.Method{
				{
					Name:        "ExampleMethod",
					Signature:   "void ExampleMethod()",
					Description: "메서드 설명을 작성하세요",
				},
			},
		},
		Metadata: node.Metadata{
			Status:  "draft",
			Created: time.Now().Format("2006-01-02"),
			Updated: time.Now().Format("2006-01-02"),
			Tags:    []string{},
		},
	}

	// Set template based on type
	if nodeType == "interface" {
		spec.Interface.Methods = []node.Method{
			{
				Name:        "Method1",
				Signature:   "ReturnType Method1(ParamType param)",
				Description: "인터페이스 메서드를 정의하세요",
			},
		}
		spec.Implementations = []string{}
	}

	if err := node.Save(filePath, &spec); err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	printSuccess("Created %s", filePath)
	printInfo("Edit the file to complete the specification")

	return nil
}

func runNodeDelete(cmd *cobra.Command, args []string) error {
	nodeName := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.NodesDir()
	filePath := filepath.Join(nodesDir, nodeName+".yaml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("node %s not found", nodeName)
	} else if err != nil {
		return fmt.Errorf("failed to inspect node %s: %w", nodeName, err)
	}

	files, err := loadNodeFilesStrict(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to plan node deletion: %w", err)
	}
	target := findNodeFile(files, filePath)
	if target == nil {
		return fmt.Errorf("node %s not found", nodeName)
	}

	lookup := buildSpecLookup(specsFromNodeFiles(files))
	targetID := target.spec.QualifiedID()
	var references []string
	updates := make(map[string]*node.Spec)
	for _, candidate := range files {
		if sameSyncPath(candidate.path, filePath) {
			continue
		}
		filtered := candidate.spec.Dependencies[:0]
		changed := false
		for _, dependency := range candidate.spec.Dependencies {
			if resolveNodeAlias(dependency.Target, lookup) == targetID {
				references = append(references, candidate.spec.QualifiedID())
				changed = true
				if nodeForce {
					continue
				}
			}
			filtered = append(filtered, dependency)
		}
		if changed && nodeForce {
			candidate.spec.Dependencies = filtered
			updates[candidate.path] = candidate.spec
		}
	}

	references = uniqueSorted(references)
	if len(references) > 0 && !nodeForce {
		return fmt.Errorf("node %s is referenced by %s; use --force to delete it and remove those references",
			targetID, strings.Join(references, ", "))
	}

	mutations, err := prepareNodeMutations(updates, []string{filePath})
	if err != nil {
		return fmt.Errorf("failed to prepare node deletion: %w", err)
	}
	if err := applyNodeMutations(mutations); err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	if err := refreshLifecycleDatabase(cfg); err != nil {
		return fmt.Errorf("node specifications were updated but the derived database refresh failed: %w; run 'gdc sync' to retry", err)
	}

	printSuccess("Deleted %s", filePath)
	if len(references) > 0 {
		printInfo("Removed references from %s", strings.Join(references, ", "))
	}

	return nil
}

func runNodeRename(cmd *cobra.Command, args []string) error {
	oldName := args[0]
	newName := args[1]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.NodesDir()

	oldPath := filepath.Join(nodesDir, oldName+".yaml")
	newPath := filepath.Join(nodesDir, newName+".yaml")

	// Check if new name already exists
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("node %s already exists", newName)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect rename target %s: %w", newName, err)
	}

	files, err := loadNodeFilesStrict(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to plan node rename: %w", err)
	}
	target := findNodeFile(files, oldPath)
	if target == nil {
		return fmt.Errorf("node %s not found", oldName)
	}

	lookup := buildSpecLookup(specsFromNodeFiles(files))
	canonicalAliases := snapshotCanonicalAliases(lookup)
	oldCanonicalID := target.spec.QualifiedID()
	target.spec.Node.ID = newName
	target.spec.Metadata.Updated = time.Now().Format("2006-01-02")
	newCanonicalID := target.spec.QualifiedID()

	updates := make(map[string]*node.Spec)
	for _, candidate := range files {
		changed := sameSyncPath(candidate.path, oldPath)
		for index := range candidate.spec.Dependencies {
			dependency := &candidate.spec.Dependencies[index]
			if resolveCanonicalAlias(dependency.Target, canonicalAliases) != oldCanonicalID {
				continue
			}
			if dependency.Target == oldCanonicalID {
				dependency.Target = newCanonicalID
			} else {
				dependency.Target = newName
			}
			changed = true
		}
		if changed {
			path := candidate.path
			if sameSyncPath(path, oldPath) {
				path = newPath
			}
			updates[path] = candidate.spec
		}
	}

	mutations, err := prepareNodeMutations(updates, []string{oldPath})
	if err != nil {
		return fmt.Errorf("failed to prepare node rename: %w", err)
	}
	if err := applyNodeMutations(mutations); err != nil {
		return fmt.Errorf("failed to rename node: %w", err)
	}
	if err := refreshLifecycleDatabase(cfg); err != nil {
		return fmt.Errorf("node specifications were updated but the derived database refresh failed: %w; run 'gdc sync' to retry", err)
	}

	printSuccess("Renamed %s to %s", oldName, newName)
	printInfo("Updated dependency references and refreshed the graph index")

	return nil
}

type nodeFile struct {
	path string
	spec *node.Spec
}

type nodeMutation struct {
	path   string
	data   []byte
	delete bool
}

type stagedNodeMutation struct {
	nodeMutation
	tempPath   string
	backupPath string
	hadFile    bool
}

func loadNodeFilesStrict(nodesDir string) ([]*nodeFile, error) {
	var files []*nodeFile
	err := filepath.Walk(nodesDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
			return nil
		}
		spec, err := node.Load(path)
		if err != nil {
			return err
		}
		files = append(files, &nodeFile{path: path, spec: spec})
		return nil
	})
	return files, err
}

func findNodeFile(files []*nodeFile, path string) *nodeFile {
	for _, file := range files {
		if sameSyncPath(file.path, path) {
			return file
		}
	}
	return nil
}

func specsFromNodeFiles(files []*nodeFile) []*node.Spec {
	specs := make([]*node.Spec, 0, len(files))
	for _, file := range files {
		specs = append(specs, file.spec)
	}
	return specs
}

func uniqueSorted(values []string) []string {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func snapshotCanonicalAliases(lookup map[string]*node.Spec) map[string]string {
	aliases := make(map[string]string, len(lookup))
	for alias, spec := range lookup {
		if spec != nil {
			aliases[alias] = spec.QualifiedID()
		}
	}
	return aliases
}

func resolveCanonicalAlias(id string, aliases map[string]string) string {
	id = strings.TrimSpace(id)
	if canonical, ok := aliases[id]; ok {
		return canonical
	}
	return id
}

func prepareNodeMutations(updates map[string]*node.Spec, deletes []string) ([]nodeMutation, error) {
	paths := make([]string, 0, len(updates))
	for path := range updates {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	mutations := make([]nodeMutation, 0, len(updates)+len(deletes))
	for _, path := range paths {
		data, err := yaml.Marshal(updates[path])
		if err != nil {
			return nil, fmt.Errorf("failed to serialize %s: %w", path, err)
		}
		mutations = append(mutations, nodeMutation{path: path, data: data})
	}
	for _, path := range deletes {
		if _, alsoUpdated := updates[path]; !alsoUpdated {
			mutations = append(mutations, nodeMutation{path: path, delete: true})
		}
	}
	return mutations, nil
}

func applyNodeMutations(mutations []nodeMutation) error {
	suffix := fmt.Sprintf(".gdc-txn-%d", time.Now().UnixNano())
	staged := make([]stagedNodeMutation, 0, len(mutations))
	for _, mutation := range mutations {
		entry := stagedNodeMutation{nodeMutation: mutation, tempPath: mutation.path + suffix + ".tmp", backupPath: mutation.path + suffix + ".bak"}
		if info, err := os.Stat(mutation.path); err == nil {
			entry.hadFile = true
			if info.IsDir() {
				return fmt.Errorf("mutation target is a directory: %s", mutation.path)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if !mutation.delete {
			if err := os.MkdirAll(filepath.Dir(mutation.path), 0755); err != nil {
				cleanupStagedNodeMutations(staged)
				return err
			}
			if err := os.WriteFile(entry.tempPath, mutation.data, 0644); err != nil {
				_ = os.Remove(entry.tempPath)
				cleanupStagedNodeMutations(staged)
				return err
			}
		}
		staged = append(staged, entry)
	}

	applied := 0
	for index := range staged {
		entry := &staged[index]
		if entry.hadFile {
			if err := os.Rename(entry.path, entry.backupPath); err != nil {
				rollbackNodeMutations(staged, applied)
				return err
			}
		}
		if !entry.delete {
			if err := os.Rename(entry.tempPath, entry.path); err != nil {
				if entry.hadFile {
					_ = os.Rename(entry.backupPath, entry.path)
				}
				rollbackNodeMutations(staged, applied)
				return err
			}
		}
		applied++
	}

	for _, entry := range staged {
		_ = os.Remove(entry.backupPath)
		_ = os.Remove(entry.tempPath)
	}
	return nil
}

func rollbackNodeMutations(staged []stagedNodeMutation, applied int) {
	for index := applied - 1; index >= 0; index-- {
		entry := staged[index]
		if !entry.delete {
			_ = os.Remove(entry.path)
		}
		if entry.hadFile {
			_ = os.Rename(entry.backupPath, entry.path)
		}
	}
	cleanupStagedNodeMutations(staged)
}

func cleanupStagedNodeMutations(staged []stagedNodeMutation) {
	for _, entry := range staged {
		_ = os.Remove(entry.tempPath)
	}
}

func refreshLifecycleDatabase(cfg *config.Config) error {
	nodes, err := loadAllNodes(cfg.NodesDir())
	if err != nil {
		return err
	}
	database, err := db.Open(cfg.DatabasePath())
	if err != nil {
		return err
	}
	defer database.Close()
	if err := database.InitSchema(); err != nil {
		return err
	}
	existing, err := database.GetAllNodes()
	if err != nil {
		return err
	}
	remaining := make(map[string]bool, len(existing))
	for _, record := range existing {
		remaining[record.QualifiedID] = true
	}
	for _, spec := range nodes {
		if err := syncNodeToDB(database, spec, calculateSpecHash(spec)); err != nil {
			return fmt.Errorf("sync %s: %w", spec.QualifiedID(), err)
		}
		delete(remaining, spec.QualifiedID())
	}
	for qualifiedID := range remaining {
		if err := database.DeleteNode(qualifiedID); err != nil {
			return fmt.Errorf("delete stale node %s: %w", qualifiedID, err)
		}
	}
	return nil
}
