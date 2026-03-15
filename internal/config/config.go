// Package config handles GDC configuration management
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the GDC project configuration
type Config struct {
	Schema            string            `yaml:"schema_version"`
	Project           Project           `yaml:"project"`
	Storage           Storage           `yaml:"storage"`
	Database          DatabaseConfig    `yaml:"database"`
	Indexer           Indexer           `yaml:"indexer"`
	Validation        Validation        `yaml:"validation"`
	LLM               LLMConfig         `yaml:"llm"`
	Output            Output            `yaml:"output"`
	Architecture      Architecture      `yaml:"architecture"`
	NamingConventions NamingConventions `yaml:"naming_conventions"`

	// Runtime fields (not saved to YAML)
	ProjectRoot string `yaml:"-"` // Absolute path to project root
}

// Project contains basic project information
type Project struct {
	Name        string `yaml:"name"`
	Language    string `yaml:"language"`
	Description string `yaml:"description,omitempty"`
	Version     string `yaml:"version,omitempty"`
	SourceDir   string `yaml:"source_dir,omitempty"` // Source code directory for code extraction
}

// Storage configures node storage
type Storage struct {
	Mode         string `yaml:"mode"` // centralized or distributed
	NodesDir     string `yaml:"nodes_dir,omitempty"`
	TemplatesDir string `yaml:"templates_dir,omitempty"`
}

// DatabaseConfig configures the SQLite database
type DatabaseConfig struct {
	Path      string `yaml:"path"`
	WALMode   bool   `yaml:"wal_mode"`
	CacheSize int    `yaml:"cache_size,omitempty"`
}

// Indexer configures the YAML indexer
type Indexer struct {
	AutoSync   bool     `yaml:"auto_sync"`
	WatchPaths []string `yaml:"watch_paths,omitempty"`
	IgnoreDirs []string `yaml:"ignore_dirs,omitempty"`
}

// Validation configures validation rules
type Validation struct {
	StrictMode     bool        `yaml:"strict_mode"`
	SRPThreshold   int         `yaml:"srp_threshold"`
	WarnOnly       []string    `yaml:"warn_only,omitempty"`
	Disabled       []string    `yaml:"disabled,omitempty"`
	RequiredFields []string    `yaml:"required_fields,omitempty"`
	Orphan         OrphanRules `yaml:"orphan,omitempty"`
}

// OrphanRules configures orphan-node filtering.
type OrphanRules struct {
	IgnorePatterns []string `yaml:"ignore_patterns,omitempty"`
	EntryPoints    []string `yaml:"entry_points,omitempty"`
}

// LLMConfig configures LLM integration
type LLMConfig struct {
	DefaultProvider string            `yaml:"default_provider,omitempty"`
	DefaultModel    string            `yaml:"default_model,omitempty"`
	PromptMaxTokens int               `yaml:"prompt_max_tokens,omitempty"`
	Providers       map[string]string `yaml:"providers,omitempty"`
}

// Output configures output formatting
type Output struct {
	ColorEnabled bool   `yaml:"color_enabled"`
	DateFormat   string `yaml:"date_format,omitempty"`
	Language     string `yaml:"language,omitempty"` // en, ko, ja
}

// Architecture configures layered architecture rules
type Architecture struct {
	Enabled        bool        `yaml:"enabled"`
	Layers         []LayerRule `yaml:"layers,omitempty"`
	ViolationLevel string      `yaml:"violation_level,omitempty"`
}

// LayerRule defines a layer and its dependencies
type LayerRule struct {
	Name        string   `yaml:"name"`
	CanDependOn []string `yaml:"can_depend_on,omitempty"`
}

// NamingConventions configures naming patterns
type NamingConventions struct {
	InterfacePrefix string `yaml:"interface_prefix,omitempty"`
	AbstractPrefix  string `yaml:"abstract_prefix,omitempty"`
	ServiceSuffix   string `yaml:"service_suffix,omitempty"`
}

// DefaultConfig returns a new config with default values
func DefaultConfig() *Config {
	return &Config{
		Schema: "1.0",
		Project: Project{
			Name:     "my-project",
			Language: "csharp",
		},
		Storage: Storage{
			Mode:         "centralized",
			NodesDir:     ".gdc/nodes",
			TemplatesDir: ".gdc/templates",
		},
		Database: DatabaseConfig{
			Path:      ".gdc/graph.db",
			WALMode:   true,
			CacheSize: 2000,
		},
		Indexer: Indexer{
			AutoSync:   true,
			IgnoreDirs: []string{".git", "node_modules", "bin", "obj"},
		},
		Validation: Validation{
			StrictMode:   false,
			SRPThreshold: 5,
			WarnOnly:     []string{"orphan", "srp_violation"},
		},
		Output: Output{
			ColorEnabled: true,
			DateFormat:   "2006-01-02",
			Language:     "en",
		},
		Architecture: Architecture{
			Enabled: true,
			Layers: []LayerRule{
				{Name: "presentation", CanDependOn: []string{"application"}},
				{Name: "application", CanDependOn: []string{"domain", "infrastructure"}},
				{Name: "domain", CanDependOn: []string{}},
				{Name: "infrastructure", CanDependOn: []string{"domain"}},
			},
		},
		NamingConventions: NamingConventions{
			InterfacePrefix: "I",
			AbstractPrefix:  "Abstract",
			ServiceSuffix:   "Service",
		},
	}
}

// Load reads the configuration from a file
func Load(path string) (*Config, error) {
	var projectRoot string

	if path == "" {
		// Check for environment variable
		if envPath := os.Getenv("GDC_CONFIG"); envPath != "" {
			path = envPath
			projectRoot = filepath.Dir(filepath.Dir(path)) // .gdc/config.yaml → project root
		} else {
			// Find .gdc directory by walking up
			gdcDir, err := GetGDCDir()
			if err != nil {
				return nil, err
			}
			path = filepath.Join(gdcDir, "config.yaml")
			projectRoot = filepath.Dir(gdcDir)
		}
	} else {
		// Explicit path provided
		projectRoot = filepath.Dir(filepath.Dir(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Store project root
	cfg.ProjectRoot = projectRoot

	// Apply defaults for missing fields
	applyDefaults(&cfg)

	return &cfg, nil
}

// Save writes the configuration to a file
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func applyDefaults(cfg *Config) {
	if cfg.Schema == "" {
		cfg.Schema = "1.0"
	}
	if cfg.Storage.Mode == "" {
		cfg.Storage.Mode = "centralized"
	}
	if cfg.Storage.NodesDir == "" {
		cfg.Storage.NodesDir = ".gdc/nodes"
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = ".gdc/graph.db"
	}
	if cfg.Validation.SRPThreshold == 0 {
		cfg.Validation.SRPThreshold = 5
	}
	if cfg.Output.DateFormat == "" {
		cfg.Output.DateFormat = "2006-01-02"
	}
}

// GetGDCDir returns the path to the .gdc directory
func GetGDCDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for .gdc
	dir := cwd
	for {
		gdcPath := filepath.Join(dir, ".gdc")
		if info, err := os.Stat(gdcPath); err == nil && info.IsDir() {
			return gdcPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(".gdc directory not found. Run 'gdc init' first")
		}
		dir = parent
	}
}

// ResolvePath converts a relative path to absolute path based on project root
func (cfg *Config) ResolvePath(relativePath string) string {
	if filepath.IsAbs(relativePath) {
		return relativePath
	}
	return filepath.Join(cfg.ProjectRoot, relativePath)
}

// NodesDir returns the absolute path to the nodes directory
func (cfg *Config) NodesDir() string {
	return cfg.ResolvePath(cfg.Storage.NodesDir)
}

// DatabasePath returns the absolute path to the database
func (cfg *Config) DatabasePath() string {
	return cfg.ResolvePath(cfg.Database.Path)
}

// TemplatesDir returns the absolute path to the templates directory
func (cfg *Config) TemplatesDir() string {
	return cfg.ResolvePath(cfg.Storage.TemplatesDir)
}
