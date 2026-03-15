// Package cli provides the command-line interface for GDC
package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	cfgFile string
	verbose bool
	quiet   bool
	jsonOut bool
	noColor bool

	// Version info
	Version   = "1.0.0-dev"
	BuildDate = "unknown"
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "gdc",
	Short: "GDC - Graph-Driven Codebase CLI",
	Long: `GDC (Graph-Driven Codebase) is a specification-driven development tool
that models software systems as graphs (Nodes & Edges) to optimize 
AI-assisted development.

Core Principles:
  • Single Source of Truth: YAML specs are the sole source of truth
  • Context Isolation: Provide minimal context to AI for maximum accuracy
  • Graph-First Design: Express systems as nodes (classes) and edges (dependencies)

Quick Start:
  $ gdc init                    # Initialize a new project
  $ gdc node create MyClass     # Create a new node specification
  $ gdc list                    # List all nodes
  $ gdc extract MyClass         # Generate AI prompt for implementation`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if noColor {
			color.NoColor = true
		}
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: .gdc/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "minimal output")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(nodeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(traceCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(extractCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(queryCmd)
}

// versionCmd shows version information
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gdc version %s (built %s)\n", Version, BuildDate)
	},
}

// Helper functions for output
func printSuccess(format string, args ...interface{}) {
	if !quiet {
		color.Green("✓ "+format, args...)
	}
}

func printInfo(format string, args ...interface{}) {
	if !quiet {
		color.Cyan("ℹ "+format, args...)
	}
}

func printWarning(format string, args ...interface{}) {
	if !quiet {
		color.Yellow("⚠ "+format, args...)
	}
}

func printError(format string, args ...interface{}) {
	color.Red("✗ "+format, args...)
}

func exitWithError(msg string, err error) {
	if err != nil {
		printError("%s: %v", msg, err)
	} else {
		printError("%s", msg)
	}
	os.Exit(1)
}
