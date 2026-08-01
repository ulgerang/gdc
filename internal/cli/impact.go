package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/spf13/cobra"
)

var (
	impactSymbol string
	impactFormat string
)

type impactFinding struct {
	Kind         string   `json:"kind" yaml:"kind"`
	SourceNode   string   `json:"source_node" yaml:"source_node"`
	TargetSymbol string   `json:"target_symbol" yaml:"target_symbol"`
	FilePath     string   `json:"file_path,omitempty" yaml:"file_path,omitempty"`
	ContractHash string   `json:"contract_hash,omitempty" yaml:"contract_hash,omitempty"`
	AcceptanceID string   `json:"acceptance_id,omitempty" yaml:"acceptance_id,omitempty"`
	Aspects      []string `json:"aspects,omitempty" yaml:"aspects,omitempty"`
	Provenance   string   `json:"provenance" yaml:"provenance"`
	Confidence   string   `json:"confidence" yaml:"confidence"`
	Action       string   `json:"action" yaml:"action"`
}

type impactReport struct {
	Query        string          `json:"query" yaml:"query"`
	Symbol       string          `json:"symbol" yaml:"symbol"`
	OwnerNode    string          `json:"owner_node" yaml:"owner_node"`
	Member       string          `json:"member,omitempty" yaml:"member,omitempty"`
	Completeness string          `json:"completeness" yaml:"completeness"`
	Findings     []impactFinding `json:"findings" yaml:"findings"`
	Notes        []string        `json:"notes,omitempty" yaml:"notes,omitempty"`
}

var impactCmd = &cobra.Command{
	Use:          "impact [symbol]",
	Short:        "Report declared structural blast radius before a change",
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runImpact,
}

func init() {
	impactCmd.Flags().StringVar(&impactSymbol, "symbol", "", "node or member symbol to analyze")
	impactCmd.Flags().StringVar(&impactFormat, "format", "text", "output format (text, json)")
}

func runImpact(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(impactSymbol)
	if len(args) == 1 {
		if query != "" && query != strings.TrimSpace(args[0]) {
			return fmt.Errorf("provide the symbol either as an argument or with --symbol, not both")
		}
		query = strings.TrimSpace(args[0])
	}
	if query == "" {
		return fmt.Errorf("symbol is required")
	}

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	nodes, err := loadAllNodes(cfg.NodesDir())
	if err != nil {
		return fmt.Errorf("failed to load nodes: %w", err)
	}
	report, err := buildImpactReport(query, nodes)
	if err != nil {
		return err
	}
	if resolveFormat(impactFormat) == "json" {
		return outputJSONValue(report)
	}
	printImpactReport(report)
	return nil
}

func buildImpactReport(query string, nodes []*node.Spec) (impactReport, error) {
	owner, member, symbol, err := resolveImpactSymbol(query, nodes)
	if err != nil {
		return impactReport{}, err
	}
	lookup := buildSpecLookup(nodes)
	report := impactReport{
		Query: query, Symbol: symbol, OwnerNode: owner.QualifiedID(), Member: member,
		Completeness: "declared_graph_only",
		Findings:     make([]impactFinding, 0),
		Notes: []string{
			"Runtime behavior remains owned by executable acceptance tests.",
			"Dynamic calls and acceptance prose without covers links are not inferred.",
		},
	}

	for _, source := range nodes {
		for _, dependency := range source.Dependencies {
			if resolveNodeAlias(dependency.Target, lookup) != owner.QualifiedID() {
				continue
			}
			required := member == "" || dependencyRequiresMember(dependency, member)
			report.Findings = append(report.Findings, impactFinding{
				Kind: "contract_holder", SourceNode: source.QualifiedID(), TargetSymbol: symbol,
				FilePath: source.Node.FilePath, ContractHash: dependency.ContractHash,
				Provenance: "declared", Confidence: "high", Action: "mechanical_hash_review",
			})
			if member != "" && required {
				report.Findings = append(report.Findings, impactFinding{
					Kind: "composition_site", SourceNode: source.QualifiedID(), TargetSymbol: symbol,
					FilePath: source.Node.FilePath, Provenance: "declared", Confidence: "high", Action: "review_required",
				})
			}
		}
		if source.ImplementationContract == nil {
			continue
		}
		for _, scenario := range source.ImplementationContract.Acceptance {
			for _, coverage := range scenario.Covers {
				covered := canonicalCoverageSymbol(source, coverage.Symbol, nodes)
				if covered != symbol {
					continue
				}
				report.Findings = append(report.Findings, impactFinding{
					Kind: "acceptance", SourceNode: source.QualifiedID(), TargetSymbol: symbol,
					FilePath: source.Node.FilePath, AcceptanceID: scenario.ID,
					Aspects:    append([]string(nil), coverage.Aspects...),
					Provenance: "declared", Confidence: "high", Action: "review_and_rerun",
				})
			}
		}
	}

	sort.SliceStable(report.Findings, func(i, j int) bool {
		a, b := report.Findings[i], report.Findings[j]
		left := a.Kind + "\x00" + a.SourceNode + "\x00" + a.AcceptanceID
		right := b.Kind + "\x00" + b.SourceNode + "\x00" + b.AcceptanceID
		return left < right
	})
	return report, nil
}

func resolveImpactSymbol(query string, nodes []*node.Spec) (*node.Spec, string, string, error) {
	lookup := buildSpecLookup(nodes)
	if spec, canonical, ok := resolveNodeSpec(query, lookup); ok {
		return spec, "", canonical, nil
	}

	ordered := append([]*node.Spec(nil), nodes...)
	sort.SliceStable(ordered, func(i, j int) bool { return len(ordered[i].QualifiedID()) > len(ordered[j].QualifiedID()) })
	for _, spec := range ordered {
		for _, alias := range []string{spec.QualifiedID(), spec.Node.ID} {
			prefix := alias + "."
			if !strings.HasPrefix(query, prefix) {
				continue
			}
			member := strings.TrimSpace(strings.TrimPrefix(query, prefix))
			if member != "" && dependencyMemberExists(spec, member) {
				return spec, member, spec.QualifiedID() + "." + member, nil
			}
		}
	}

	var matches []*node.Spec
	for _, spec := range nodes {
		if dependencyMemberExists(spec, query) {
			matches = append(matches, spec)
		}
	}
	if len(matches) == 1 {
		return matches[0], query, matches[0].QualifiedID() + "." + query, nil
	}
	if len(matches) > 1 {
		owners := make([]string, 0, len(matches))
		for _, spec := range matches {
			owners = append(owners, spec.QualifiedID())
		}
		sort.Strings(owners)
		return nil, "", "", fmt.Errorf("symbol %q is ambiguous; qualify it with one of: %s", query, strings.Join(owners, ", "))
	}
	return nil, "", "", fmt.Errorf("symbol %q was not found", query)
}

func dependencyRequiresMember(dependency node.Dependency, member string) bool {
	for _, required := range dependency.Requires {
		required = strings.TrimSpace(required)
		if required == member || strings.HasSuffix(required, "."+member) {
			return true
		}
	}
	return false
}

func canonicalCoverageSymbol(source *node.Spec, declared string, nodes []*node.Spec) string {
	declared = strings.TrimSpace(declared)
	if declared == "" {
		return ""
	}
	if !strings.Contains(declared, ".") && dependencyMemberExists(source, declared) {
		return source.QualifiedID() + "." + declared
	}
	_, _, canonical, err := resolveImpactSymbol(declared, nodes)
	if err != nil {
		return declared
	}
	return canonical
}

func printImpactReport(report impactReport) {
	fmt.Printf("Symbol: %s\n", report.Symbol)
	fmt.Printf("Completeness: %s\n", report.Completeness)
	if len(report.Findings) == 0 {
		fmt.Println("Findings: none")
	} else {
		fmt.Println("Findings:")
		for _, finding := range report.Findings {
			detail := finding.SourceNode
			if finding.AcceptanceID != "" {
				detail += " / " + finding.AcceptanceID
			}
			if finding.FilePath != "" {
				detail += " / " + finding.FilePath
			}
			fmt.Printf("- %s: %s [%s, %s, %s]\n", finding.Kind, detail, finding.Provenance, finding.Confidence, finding.Action)
		}
	}
	for _, note := range report.Notes {
		fmt.Printf("Note: %s\n", note)
	}
}
