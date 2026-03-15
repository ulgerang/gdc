package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/db"
	"github.com/spf13/cobra"
)

var (
	initLanguage string
	initStorage  string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new GDC project",
	Long: `Initialize a new GDC project in the current directory.

This command creates:
  • .gdc/config.yaml    - Project configuration
  • .gdc/graph.db       - SQLite database for indexing
  • .gdc/nodes/         - Directory for node specifications
  • .gdc/templates/     - Directory for prompt templates

Example:
  $ gdc init
  $ gdc init --language typescript
  $ gdc init --language go --storage distributed`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initLanguage, "language", "l", "csharp",
		"primary language (csharp, typescript, go, python, java)")
	initCmd.Flags().StringVarP(&initStorage, "storage", "s", "centralized",
		"storage mode (centralized, distributed)")
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	gdcDir := filepath.Join(cwd, ".gdc")

	// Check if already initialized
	if _, err := os.Stat(gdcDir); err == nil {
		printWarning("GDC is already initialized in this directory")
		return nil
	}

	// Create directory structure
	dirs := []string{
		gdcDir,
		filepath.Join(gdcDir, "nodes"),
		filepath.Join(gdcDir, "templates"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create config file
	cfg := config.DefaultConfig()
	cfg.Project.Language = initLanguage
	cfg.Storage.Mode = initStorage
	cfg.Project.Name = filepath.Base(cwd)

	configPath := filepath.Join(gdcDir, "config.yaml")
	if err := config.Save(configPath, cfg); err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}
	printSuccess("Created %s", configPath)

	// Initialize database
	dbPath := filepath.Join(gdcDir, "graph.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer database.Close()

	if err := database.InitSchema(); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	printSuccess("Initialized %s", dbPath)

	// Create default template
	templatePath := filepath.Join(gdcDir, "templates", "implement.md.j2")
	if err := createDefaultTemplate(templatePath); err != nil {
		printWarning("Failed to create default template: %v", err)
	} else {
		printSuccess("Created %s", templatePath)
	}

	// Suggest adding to .gitignore
	printInfo("Add '.gdc/graph.db' to your .gitignore")

	fmt.Println()
	printSuccess("GDC initialized successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Create your first node: gdc node create MyClass")
	fmt.Println("  2. Edit the node spec:     .gdc/nodes/MyClass.yaml")
	fmt.Println("  3. Sync and validate:      gdc sync && gdc check")
	fmt.Println("  4. Generate AI prompt:     gdc extract MyClass")

	return nil
}

func createDefaultTemplate(path string) error {
	template := `# Implementation Request: {{ .Node.ID }}

## Target Node Specification

### Responsibility
{{ .Node.Responsibility.Summary }}

{{ if .Node.Responsibility.Details }}
**Details:**
{{ .Node.Responsibility.Details }}
{{ end }}

## Public Interface to Implement

` + "```{{ .Config.Language }}" + `
{{ .InterfaceCode }}
` + "```" + `

{{ if .Dependencies }}
## Available Dependencies (Contracts)

{{ range .Dependencies }}
### {{ .Target }}{{ if .Optional }} (Optional){{ end }}

` + "```{{ $.Config.Language }}" + `
{{ .InterfaceCode }}
` + "```" + `

{{ if .Usage }}
**Usage:**
{{ .Usage }}
{{ end }}

{{ end }}
{{ end }}

## Implementation Guidelines

1. Implement EXACTLY the interface specified above.
2. Use only the provided dependencies.
3. Maintain all listed invariants.
4. Follow {{ .Config.Language }} idiomatic conventions.
`
	return os.WriteFile(path, []byte(template), 0644)
}
