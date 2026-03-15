package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
)

var (
	nodeType  string
	nodeLayer string
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
	Long: `Rename a node and update all references.

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
	nodesDir := cfg.Storage.NodesDir
	if nodesDir == "" {
		nodesDir = ".gdc/nodes"
	}
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

	nodesDir := cfg.Storage.NodesDir
	if nodesDir == "" {
		nodesDir = ".gdc/nodes"
	}
	filePath := filepath.Join(nodesDir, nodeName+".yaml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("node %s not found", nodeName)
	}

	// TODO: Check for references before deleting

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	printSuccess("Deleted %s", filePath)

	return nil
}

func runNodeRename(cmd *cobra.Command, args []string) error {
	oldName := args[0]
	newName := args[1]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.Storage.NodesDir
	if nodesDir == "" {
		nodesDir = ".gdc/nodes"
	}

	oldPath := filepath.Join(nodesDir, oldName+".yaml")
	newPath := filepath.Join(nodesDir, newName+".yaml")

	// Load existing spec
	spec, err := node.Load(oldPath)
	if err != nil {
		return fmt.Errorf("failed to load node %s: %w", oldName, err)
	}

	// Check if new name already exists
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("node %s already exists", newName)
	}

	// Update ID
	spec.Node.ID = newName
	spec.Metadata.Updated = time.Now().Format("2006-01-02")

	// Save to new path
	if err := node.Save(newPath, spec); err != nil {
		return fmt.Errorf("failed to save renamed node: %w", err)
	}

	// Delete old file
	if err := os.Remove(oldPath); err != nil {
		printWarning("Failed to remove old file: %v", err)
	}

	printSuccess("Renamed %s to %s", oldName, newName)
	printInfo("Remember to update any references to this node")

	return nil
}
