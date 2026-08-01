package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdc-tools/gdc/internal/node"
)

type hashFixPlan struct {
	Kind           string `json:"kind" yaml:"kind"`
	Disposition    string `json:"disposition" yaml:"disposition"`
	SourceNode     string `json:"source_node" yaml:"source_node"`
	Target         string `json:"target" yaml:"target"`
	Path           string `json:"path,omitempty" yaml:"path,omitempty"`
	ExpectedHash   string `json:"expected_hash,omitempty" yaml:"expected_hash,omitempty"`
	ObservedHash   string `json:"observed_hash,omitempty" yaml:"observed_hash,omitempty"`
	AutoApplicable bool   `json:"auto_applicable" yaml:"auto_applicable"`
	Action         string `json:"action" yaml:"action"`
	Error          string `json:"error,omitempty" yaml:"error,omitempty"`
}

type hashFixPlanOutput struct {
	Category string        `json:"category" yaml:"category"`
	Plans    []hashFixPlan `json:"plans" yaml:"plans"`
}

func buildHashFixPlans(nodes []*node.Spec, lookup map[string]*node.Spec, projectRoot string) []hashFixPlan {
	plans := make([]hashFixPlan, 0)
	for _, source := range nodes {
		for _, dependency := range source.Dependencies {
			target, canonical, ok := resolveNodeSpec(dependency.Target, lookup)
			if !ok {
				continue
			}
			observed := calculateSpecHash(target)
			if dependency.ContractHash == observed {
				continue
			}
			plans = append(plans, hashFixPlan{
				Kind: "dependency", Disposition: "safe_mechanical", SourceNode: source.QualifiedID(), Target: canonical,
				ExpectedHash: dependency.ContractHash, ObservedHash: observed, AutoApplicable: true,
				Action: "review the structural diff, then update the dependency contract_hash",
			})
		}
		if source.ImplementationContract == nil {
			continue
		}
		for _, external := range source.ImplementationContract.ExternalContracts {
			observed, err := observeExternalContractHash(projectRoot, external.Path)
			if err == nil && external.ContractHash == observed {
				continue
			}
			plan := hashFixPlan{
				Kind: "external_contract", Disposition: "review_required", SourceNode: source.QualifiedID(), Target: external.ID,
				Path: external.Path, ExpectedHash: external.ContractHash, ObservedHash: observed, AutoApplicable: false,
				Action: "review the canonical document change and explicitly acknowledge the new hash; never auto-apply",
			}
			if err != nil {
				plan.Error = err.Error()
			}
			plans = append(plans, plan)
		}
	}
	sort.SliceStable(plans, func(i, j int) bool {
		left := plans[i].Disposition + "\x00" + plans[i].Kind + "\x00" + plans[i].SourceNode + "\x00" + plans[i].Target
		right := plans[j].Disposition + "\x00" + plans[j].Kind + "\x00" + plans[j].SourceNode + "\x00" + plans[j].Target
		return left < right
	})
	return plans
}

func observeExternalContractHash(projectRoot, contractPath string) (string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	cleaned := filepath.Clean(contractPath)
	if filepath.IsAbs(contractPath) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must be repository-relative and stay inside the project root")
	}
	absolute, err := filepath.Abs(filepath.Join(root, cleaned))
	if err != nil || (absolute != root && !strings.HasPrefix(absolute, root+string(filepath.Separator))) {
		return "", fmt.Errorf("path must be repository-relative and stay inside the project root")
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func outputHashFixPlans(plans []hashFixPlan, format string) error {
	output := hashFixPlanOutput{Category: "hash_mismatch", Plans: plans}
	if resolveFormat(format) == "json" {
		return outputJSONValue(output)
	}
	if len(plans) == 0 {
		fmt.Println("Hash fix plan: no mismatches")
		return nil
	}
	fmt.Println("Hash fix plan:")
	for _, plan := range plans {
		fmt.Printf("- %s/%s %s -> %s: %s -> %s\n", plan.Disposition, plan.Kind, plan.SourceNode, plan.Target, plan.ExpectedHash, plan.ObservedHash)
		fmt.Printf("  action: %s\n", plan.Action)
		if plan.Error != "" {
			fmt.Printf("  error: %s\n", plan.Error)
		}
	}
	return nil
}
