package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
)

const defaultReadinessPhase = "implementation"

var (
	preflightProfile string
	preflightPhase   string
	preflightFormat  string
)

// ResolvedExternalContract is the exact non-code contract embedded in a
// profile-selected source-free packet.
type ResolvedExternalContract struct {
	ID          string `json:"id" yaml:"id"`
	Path        string `json:"path" yaml:"path"`
	SHA256      string `json:"sha256" yaml:"sha256"`
	Description string `json:"description" yaml:"description"`
	Content     string `json:"content" yaml:"content"`
}

// ReadinessGate reports one gate relevant to the selected phase and profile.
type ReadinessGate struct {
	ID          string `json:"id" yaml:"id"`
	Kind        string `json:"kind" yaml:"kind"`
	Phase       string `json:"phase" yaml:"phase"`
	Status      string `json:"status" yaml:"status"`
	Description string `json:"description" yaml:"description"`
	Reason      string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Contract    string `json:"contract,omitempty" yaml:"contract,omitempty"`
}

// ImplementationReadinessReport separates authored contract completeness from
// permission to act in a selected phase.
type ImplementationReadinessReport struct {
	NodeID                    string                     `json:"node_id" yaml:"node_id"`
	SchemaVersion             string                     `json:"schema_version" yaml:"schema_version"`
	Profile                   string                     `json:"profile,omitempty" yaml:"profile,omitempty"`
	Phase                     string                     `json:"phase" yaml:"phase"`
	SourceFree                bool                       `json:"source_free" yaml:"source_free"`
	ContractComplete          bool                       `json:"contract_complete" yaml:"contract_complete"`
	DependencyClosureComplete bool                       `json:"dependency_closure_complete" yaml:"dependency_closure_complete"`
	ExternalContractsComplete bool                       `json:"external_contracts_complete" yaml:"external_contracts_complete"`
	GatesSatisfied            bool                       `json:"gates_satisfied" yaml:"gates_satisfied"`
	AuthorityComplete         bool                       `json:"authority_complete" yaml:"authority_complete"`
	EvidenceComplete          bool                       `json:"evidence_complete" yaml:"evidence_complete"`
	ProvenanceComplete        bool                       `json:"provenance_complete" yaml:"provenance_complete"`
	Sealed                    bool                       `json:"sealed" yaml:"sealed"`
	PhasePermitted            bool                       `json:"phase_permitted" yaml:"phase_permitted"`
	ImplementationPermitted   bool                       `json:"implementation_permitted" yaml:"implementation_permitted"`
	Missing                   []string                   `json:"missing,omitempty" yaml:"missing,omitempty"`
	BlockedBy                 []string                   `json:"blocked_by,omitempty" yaml:"blocked_by,omitempty"`
	Gates                     []ReadinessGate            `json:"gates,omitempty" yaml:"gates,omitempty"`
	ExternalContracts         []ResolvedExternalContract `json:"external_contracts,omitempty" yaml:"external_contracts,omitempty"`
}

var preflightCmd = &cobra.Command{
	Use:          "preflight <node>",
	Short:        "Evaluate source-free contract readiness before implementation",
	SilenceUsage: true,
	Long: `Evaluate a node's authored contract without reading implementation source.

Schema 1.2 preflight selects one implementation profile, closes its code and
external contracts, evaluates only relevant phase gates, and reports contract
completeness separately from permission to implement, verify, or publish.`,
	Args: cobra.ExactArgs(1),
	RunE: runPreflight,
}

func init() {
	preflightCmd.Flags().StringVar(&preflightProfile, "profile", "", "implementation profile (required when more than one is declared)")
	preflightCmd.Flags().StringVar(&preflightPhase, "phase", defaultReadinessPhase, "phase to evaluate (contract, implementation, verification, publish)")
	preflightCmd.Flags().StringVar(&preflightFormat, "format", "text", "output format (text, json)")
}

func runPreflight(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	nodesDir := cfg.Storage.NodesDir
	if nodesDir == "" {
		nodesDir = ".gdc/nodes"
	}
	allNodes, err := loadAllNodes(nodesDir)
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}
	lookup := buildSpecLookup(allNodes)
	spec, err := resolveExtractNodeSpec(nodesDir, args[0], lookup)
	if err != nil {
		return err
	}
	report := evaluateImplementationReadiness(spec, lookup, cfg.ProjectRoot, preflightProfile, preflightPhase)
	if resolveFormat(preflightFormat) == "json" {
		if err := outputJSONValue(report); err != nil {
			return err
		}
	} else {
		printReadinessReport(report)
	}
	if !report.PhasePermitted {
		return fmt.Errorf("preflight blocked for %s phase", report.Phase)
	}
	return nil
}

func printReadinessReport(report ImplementationReadinessReport) {
	fmt.Printf("Node: %s\n", report.NodeID)
	if report.Profile != "" {
		fmt.Printf("Profile: %s\n", report.Profile)
	}
	fmt.Printf("Phase: %s\n", report.Phase)
	fmt.Printf("Contract Complete: %t\n", report.ContractComplete)
	fmt.Printf("Dependency Closure Complete: %t\n", report.DependencyClosureComplete)
	fmt.Printf("External Contracts Complete: %t\n", report.ExternalContractsComplete)
	fmt.Printf("Gates Satisfied: %t\n", report.GatesSatisfied)
	fmt.Printf("Sealed: %t\n", report.Sealed)
	fmt.Printf("Phase Permitted: %t\n", report.PhasePermitted)
	fmt.Printf("Implementation Permitted: %t\n", report.ImplementationPermitted)
	if len(report.Missing) > 0 {
		fmt.Println("Missing:")
		for _, issue := range report.Missing {
			fmt.Printf("- %s\n", issue)
		}
	}
	if len(report.BlockedBy) > 0 {
		fmt.Println("Blocked By:")
		for _, blocker := range report.BlockedBy {
			fmt.Printf("- %s\n", blocker)
		}
	}
}

func evaluateImplementationReadiness(spec *node.Spec, lookup map[string]*node.Spec, projectRoot, requestedProfile, requestedPhase string) ImplementationReadinessReport {
	phase := strings.TrimSpace(requestedPhase)
	if phase == "" {
		phase = defaultReadinessPhase
	}
	report := ImplementationReadinessReport{
		NodeID:                    spec.QualifiedID(),
		SchemaVersion:             spec.SchemaVersion,
		Phase:                     phase,
		SourceFree:                true,
		DependencyClosureComplete: true,
		ExternalContractsComplete: true,
		GatesSatisfied:            true,
		AuthorityComplete:         true,
		EvidenceComplete:          true,
		ProvenanceComplete:        true,
		Sealed:                    true,
	}
	if !validReadinessPhase(phase) {
		report.Missing = append(report.Missing, fmt.Sprintf("phase %q is invalid; expected contract, implementation, verification, or publish", phase))
	}

	closureIssues := validateImplementationClosure(spec, lookup)
	for _, issue := range closureIssues {
		report.Missing = append(report.Missing, issue)
		if strings.Contains(issue, ".dependencies[") || strings.Contains(issue, "dependency is missing") {
			report.DependencyClosureComplete = false
		}
	}

	closure := implementationClosureSpecs(spec, lookup)
	selectedProfiles := make(map[string]string, len(closure))
	for _, current := range closure {
		if strings.TrimSpace(current.SchemaVersion) != "1.2" {
			continue
		}
		selected := ""
		if current.QualifiedID() == spec.QualifiedID() {
			selected = strings.TrimSpace(requestedProfile)
		}
		profile, issue := selectImplementationProfile(current, selected)
		if issue != "" {
			report.Missing = append(report.Missing, issue)
			if current.QualifiedID() != spec.QualifiedID() {
				report.DependencyClosureComplete = false
			}
			continue
		}
		selectedProfiles[current.QualifiedID()] = profile.ID
		if current.QualifiedID() == spec.QualifiedID() {
			report.Profile = profile.ID
		}
		if current.ImplementationContract.Status != "sealed" {
			report.Sealed = false
			report.BlockedBy = append(report.BlockedBy, fmt.Sprintf("%s: implementation_contract.status=%s; sealed is required", current.QualifiedID(), current.ImplementationContract.Status))
		}
	}

	for _, current := range closure {
		if strings.TrimSpace(current.SchemaVersion) != "1.2" || current.ImplementationContract == nil {
			continue
		}
		profileID := selectedProfiles[current.QualifiedID()]
		if profileID == "" {
			continue
		}
		contracts, issues := resolveSelectedExternalContracts(current, projectRoot, profileID)
		if len(issues) > 0 {
			report.ExternalContractsComplete = false
			report.Missing = append(report.Missing, issues...)
		} else {
			report.ExternalContracts = append(report.ExternalContracts, contracts...)
		}
		for _, gate := range current.ImplementationContract.Gates {
			if !profileApplies(gate.Profiles, profileID) || (gate.Phase != "contract" && gate.Phase != phase) {
				continue
			}
			resolved := ReadinessGate{
				ID: gate.ID, Kind: gate.Kind, Phase: gate.Phase, Status: gate.Status,
				Description: gate.Description, Reason: gate.Reason, Contract: gate.Contract,
			}
			report.Gates = append(report.Gates, resolved)
			if gate.Status != "satisfied" {
				report.GatesSatisfied = false
				report.BlockedBy = append(report.BlockedBy, fmt.Sprintf("%s.%s: %s gate is %s", current.QualifiedID(), gate.ID, gate.Kind, gate.Status))
				switch gate.Kind {
				case "approval":
					report.AuthorityComplete = false
				case "evidence":
					report.EvidenceComplete = false
				case "provenance":
					report.ProvenanceComplete = false
				}
			}
		}
	}

	sort.SliceStable(report.ExternalContracts, func(i, j int) bool {
		if report.ExternalContracts[i].ID == report.ExternalContracts[j].ID {
			return report.ExternalContracts[i].Path < report.ExternalContracts[j].Path
		}
		return report.ExternalContracts[i].ID < report.ExternalContracts[j].ID
	})
	sort.SliceStable(report.Gates, func(i, j int) bool { return report.Gates[i].ID < report.Gates[j].ID })
	report.Missing = uniqueSortedStrings(report.Missing)
	report.BlockedBy = uniqueSortedStrings(report.BlockedBy)
	report.ContractComplete = len(report.Missing) == 0

	sealRequired := phase != "contract" && strings.TrimSpace(spec.SchemaVersion) == "1.2"
	report.PhasePermitted = report.ContractComplete && report.DependencyClosureComplete && report.ExternalContractsComplete && report.GatesSatisfied && (!sealRequired || report.Sealed)
	report.ImplementationPermitted = phase == "implementation" && report.PhasePermitted
	return report
}

func implementationClosureSpecs(spec *node.Spec, lookup map[string]*node.Spec) []*node.Spec {
	seen := map[string]bool{}
	var result []*node.Spec
	var visit func(*node.Spec)
	visit = func(current *node.Spec) {
		if current == nil || seen[current.QualifiedID()] {
			return
		}
		seen[current.QualifiedID()] = true
		result = append(result, current)
		edges := append([]node.Dependency(nil), current.Dependencies...)
		sort.SliceStable(edges, func(i, j int) bool { return edges[i].Target < edges[j].Target })
		for _, edge := range edges {
			visit(lookup[edge.Target])
		}
	}
	visit(spec)
	return result
}

func selectImplementationProfile(spec *node.Spec, requested string) (node.ImplementationProfile, string) {
	if spec == nil || spec.ImplementationContract == nil || strings.TrimSpace(spec.SchemaVersion) != "1.2" {
		return node.ImplementationProfile{}, ""
	}
	profiles := spec.ImplementationContract.Profiles
	if requested == "" {
		if len(profiles) == 1 {
			return profiles[0], ""
		}
		ids := make([]string, 0, len(profiles))
		for _, profile := range profiles {
			ids = append(ids, profile.ID)
		}
		sort.Strings(ids)
		return node.ImplementationProfile{}, fmt.Sprintf("%s: profile is required; available profiles: %s", spec.QualifiedID(), strings.Join(ids, ", "))
	}
	for _, profile := range profiles {
		if profile.ID == requested {
			return profile, ""
		}
	}
	return node.ImplementationProfile{}, fmt.Sprintf("%s: unknown profile %q", spec.QualifiedID(), requested)
}

func resolveSelectedExternalContracts(spec *node.Spec, projectRoot, profileID string) ([]ResolvedExternalContract, []string) {
	var resolved []ResolvedExternalContract
	var issues []string
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: resolve project root: %v", spec.QualifiedID(), err)}
	}
	for _, contract := range spec.ImplementationContract.ExternalContracts {
		if !profileApplies(contract.Profiles, profileID) {
			continue
		}
		cleaned := filepath.Clean(contract.Path)
		if filepath.IsAbs(contract.Path) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			issues = append(issues, fmt.Sprintf("%s.external_contracts[%s]: path must be repository-relative and stay inside the project root", spec.QualifiedID(), contract.ID))
			continue
		}
		absolute, err := filepath.Abs(filepath.Join(root, cleaned))
		if err != nil || (absolute != root && !strings.HasPrefix(absolute, root+string(filepath.Separator))) {
			issues = append(issues, fmt.Sprintf("%s.external_contracts[%s]: path must be repository-relative and stay inside the project root", spec.QualifiedID(), contract.ID))
			continue
		}
		content, err := os.ReadFile(absolute)
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s.external_contracts[%s]: cannot read %s: %v", spec.QualifiedID(), contract.ID, contract.Path, err))
			continue
		}
		if !utf8.Valid(content) {
			issues = append(issues, fmt.Sprintf("%s.external_contracts[%s]: contract must be UTF-8 text", spec.QualifiedID(), contract.ID))
			continue
		}
		digest := sha256.Sum256(content)
		observed := hex.EncodeToString(digest[:])
		if contract.ContractHash != observed {
			issues = append(issues, fmt.Sprintf("%s.external_contracts[%s]: contract_hash mismatch: expected %s, observed %s", spec.QualifiedID(), contract.ID, contract.ContractHash, observed))
			continue
		}
		resolved = append(resolved, ResolvedExternalContract{
			ID: contract.ID, Path: filepath.ToSlash(cleaned), SHA256: observed,
			Description: contract.Description, Content: string(content),
		})
	}
	return resolved, issues
}

func profileApplies(profiles []string, selected string) bool {
	if len(profiles) == 0 {
		return true
	}
	for _, profile := range profiles {
		if profile == selected {
			return true
		}
	}
	return false
}

func validReadinessPhase(phase string) bool {
	switch phase {
	case "contract", "implementation", "verification", "publish":
		return true
	default:
		return false
	}
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	var result []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
