package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
)

var (
	ctxDepth       int
	ctxWithImpl    bool
	ctxWithTests   bool
	ctxWithCallers bool
)

var contextCmd = &cobra.Command{
	Use:   "context <node-id>",
	Short: "Return full extraction context for a node (JSON output)",
	Long: `Return the full extraction context for a node as JSON — spec, deps, and optionally evidence.

Always outputs JSON. This is a machine-facing command.

Examples:
  $ gdc context PlayerController
  $ gdc context PlayerController --depth 2
  $ gdc context PlayerController --with-impl --with-tests --with-callers`,
	Args: cobra.ExactArgs(1),
	RunE: runContext,
}

func init() {
	contextCmd.Flags().IntVarP(&ctxDepth, "depth", "d", 1, "dependency depth")
	contextCmd.Flags().BoolVar(&ctxWithImpl, "with-impl", false, "include implementation code evidence")
	contextCmd.Flags().BoolVar(&ctxWithTests, "with-tests", false, "include test file evidence")
	contextCmd.Flags().BoolVar(&ctxWithCallers, "with-callers", false, "include caller/reference evidence")
}

func runContext(cmd *cobra.Command, args []string) error {
	nodeID := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.NodesDir()

	allNodes, err := loadAllNodes(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}

	lookup := buildSpecLookup(allNodes)
	spec, canonical, found := resolveNodeSpec(nodeID, lookup)
	if !found {
		return fmt.Errorf("node %s not found", nodeID)
	}

	deps := gatherDependencies(spec, lookup, ctxDepth, cfg.Project.Language)

	var evidence extractEvidence
	if ctxWithImpl || ctxWithTests || ctxWithCallers {
		savedFlags := saveExtractFlags()
		extractWithImpl = ctxWithImpl
		extractWithTests = ctxWithTests
		extractWithCallers = ctxWithCallers
		evidence, err = collectExtractEvidence(context.Background(), spec, cfg)
		restoreExtractFlags(savedFlags)
		if err != nil {
			return fmt.Errorf("failed to collect evidence: %w", err)
		}
	}

	refs := findReferences(canonical, allNodes)

	out := buildContextOutput(spec, canonical, deps, evidence, refs)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

type extractFlags struct {
	impl    bool
	tests   bool
	callers bool
}

func saveExtractFlags() extractFlags {
	return extractFlags{
		impl:    extractWithImpl,
		tests:   extractWithTests,
		callers: extractWithCallers,
	}
}

func restoreExtractFlags(f extractFlags) {
	extractWithImpl = f.impl
	extractWithTests = f.tests
	extractWithCallers = f.callers
}

type contextNodeInfo struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Layer     string `json:"layer"`
	Namespace string `json:"namespace"`
	FilePath  string `json:"file_path"`
	Status    string `json:"status"`
}

type contextResponsibility struct {
	Summary    string   `json:"summary"`
	Details    string   `json:"details"`
	Invariants []string `json:"invariants"`
	Boundaries string   `json:"boundaries"`
}

type contextInterface struct {
	Constructors []contextConstructor `json:"constructors"`
	Methods      []contextMethod      `json:"methods"`
	Properties   []contextProperty    `json:"properties"`
	Events       []contextEvent       `json:"events"`
}

type contextConstructor struct {
	Signature   string             `json:"signature"`
	Description string             `json:"description"`
	Parameters  []contextParameter `json:"parameters"`
}

type contextMethod struct {
	Name        string             `json:"name"`
	Signature   string             `json:"signature"`
	Description string             `json:"description"`
	Parameters  []contextParameter `json:"parameters"`
	Returns     contextReturn      `json:"returns"`
}

type contextParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type contextReturn struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type contextProperty struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Access      string `json:"access"`
	Description string `json:"description"`
}

type contextEvent struct {
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	Description string `json:"description"`
}

type contextDepSpec struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Layer         string `json:"layer"`
	Status        string `json:"status"`
	InterfaceCode string `json:"interface_code,omitempty"`
}

type contextDep struct {
	Target    string          `json:"target"`
	Type      string          `json:"type"`
	Injection string          `json:"injection"`
	Optional  bool            `json:"optional"`
	Usage     string          `json:"usage"`
	Depth     int             `json:"depth"`
	Spec      *contextDepSpec `json:"spec,omitempty"`
}

type contextOutput struct {
	Node           contextNodeInfo       `json:"node"`
	Responsibility contextResponsibility `json:"responsibility"`
	Interface      contextInterface      `json:"interface"`
	Dependencies   []contextDep          `json:"dependencies"`
	Implementation interface{}           `json:"implementation"`
	Tests          interface{}           `json:"tests"`
	Callers        interface{}           `json:"callers"`
	References     interface{}           `json:"references"`
	Warnings       []string              `json:"warnings"`
}

func buildContextOutput(spec *node.Spec, canonical string, deps []DependencyInfo, evidence extractEvidence, refs []string) contextOutput {
	out := contextOutput{
		Node: contextNodeInfo{
			ID:        spec.Node.ID,
			Type:      spec.Node.Type,
			Layer:     spec.Node.Layer,
			Namespace: spec.Node.Namespace,
			FilePath:  spec.Node.FilePath,
			Status:    spec.Metadata.Status,
		},
		Responsibility: contextResponsibility{
			Summary:    spec.Responsibility.Summary,
			Details:    spec.Responsibility.Details,
			Invariants: spec.Responsibility.Invariants,
			Boundaries: spec.Responsibility.Boundaries,
		},
		Interface:      convertInterface(spec.Interface),
		Dependencies:   convertDeps(deps),
		Implementation: nil,
		Tests:          nil,
		Callers:        nil,
		References:     nil,
		Warnings:       evidence.Warnings,
	}

	if len(refs) > 0 {
		out.References = refs
	}

	if evidence.Implementation != nil {
		out.Implementation = evidence.Implementation
	}
	if len(evidence.Tests) > 0 {
		out.Tests = evidence.Tests
	}
	if len(evidence.Callers) > 0 {
		out.Callers = evidence.Callers
	}

	return out
}

func convertInterface(iface node.Interface) contextInterface {
	out := contextInterface{}

	for _, ctor := range iface.Constructors {
		params := make([]contextParameter, len(ctor.Parameters))
		for i, p := range ctor.Parameters {
			params[i] = contextParameter{Name: p.Name, Type: p.Type, Description: p.Description}
		}
		out.Constructors = append(out.Constructors, contextConstructor{
			Signature:   ctor.Signature,
			Description: ctor.Description,
			Parameters:  params,
		})
	}

	for _, m := range iface.Methods {
		params := make([]contextParameter, len(m.Parameters))
		for i, p := range m.Parameters {
			params[i] = contextParameter{Name: p.Name, Type: p.Type, Description: p.Description}
		}
		out.Methods = append(out.Methods, contextMethod{
			Name:        m.Name,
			Signature:   m.Signature,
			Description: m.Description,
			Parameters:  params,
			Returns:     contextReturn{Type: m.Returns.Type, Description: m.Returns.Description},
		})
	}

	for _, p := range iface.Properties {
		out.Properties = append(out.Properties, contextProperty{
			Name: p.Name, Type: p.Type, Access: p.Access, Description: p.Description,
		})
	}

	for _, e := range iface.Events {
		out.Events = append(out.Events, contextEvent{
			Name: e.Name, Signature: e.Signature, Description: e.Description,
		})
	}

	return out
}

func convertDeps(deps []DependencyInfo) []contextDep {
	out := make([]contextDep, len(deps))
	for i, d := range deps {
		entry := contextDep{
			Target:    d.Target,
			Type:      d.Type,
			Injection: d.Injection,
			Optional:  d.Optional,
			Usage:     d.Usage,
			Depth:     1,
		}
		if d.Spec != nil {
			entry.Spec = &contextDepSpec{
				ID:            d.Spec.Node.ID,
				Type:          d.Spec.Node.Type,
				Layer:         d.Spec.Node.Layer,
				Status:        d.Spec.Metadata.Status,
				InterfaceCode: d.InterfaceCode,
			}
		}
		out[i] = entry
	}
	return out
}
