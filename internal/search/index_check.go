package search

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Common errors
var (
	ErrIndexNotBuilt  = errors.New("code index not built")
	ErrProjectNotInit = errors.New("project not initialized")
	ErrNoNodesFound   = errors.New("no node specifications found")
)

// IndexChecker provides graceful degradation utilities
type IndexChecker struct {
	projectRoot string
	gdcDir      string
}

// NewIndexChecker creates a new index checker
func NewIndexChecker(projectRoot string) *IndexChecker {
	return &IndexChecker{
		projectRoot: projectRoot,
		gdcDir:      filepath.Join(projectRoot, ".gdc"),
	}
}

// Check verifies if the required index/data is available
func (c *IndexChecker) Check() error {
	// Check if .gdc directory exists
	if _, err := os.Stat(c.gdcDir); os.IsNotExist(err) {
		return ErrProjectNotInit
	}

	// Check if nodes directory exists and has files
	nodesDir := filepath.Join(c.gdcDir, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		return ErrNoNodesFound
	}

	yamlCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			yamlCount++
		}
	}

	if yamlCount == 0 {
		return ErrNoNodesFound
	}

	return nil
}

// SuggestCommand returns a helpful message for the user
func (c *IndexChecker) SuggestCommand(err error) string {
	switch {
	case errors.Is(err, ErrProjectNotInit):
		return `Project not initialized. Run:
  gdc init
  
This will create the .gdc directory and set up the project structure.`

	case errors.Is(err, ErrNoNodesFound):
		return `No node specifications found. Create nodes with:
  gdc node create <NodeName>
  
Or sync from existing code:
  gdc sync --direction code`

	case errors.Is(err, ErrIndexNotBuilt):
		return `Code index not built. Try re-initializing the project:
  gdc init
  
Note: Basic search works without index, but having node specs improves results.`

	default:
		return fmt.Sprintf("Error: %v", err)
	}
}

// IsGracefulError checks if an error should be handled gracefully
func IsGracefulError(err error) bool {
	return errors.Is(err, ErrIndexNotBuilt) ||
		errors.Is(err, ErrProjectNotInit) ||
		errors.Is(err, ErrNoNodesFound)
}

// CheckAndSuggest checks index availability and returns suggestion if unavailable
func CheckAndSuggest(projectRoot string) error {
	checker := NewIndexChecker(projectRoot)
	if err := checker.Check(); err != nil {
		return fmt.Errorf("%w\n\n%s", err, checker.SuggestCommand(err))
	}
	return nil
}
