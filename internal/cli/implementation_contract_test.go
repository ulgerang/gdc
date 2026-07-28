package cli

import (
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/node"
)

func TestValidateImplementationClosureAcceptsCompleteContracts(t *testing.T) {
	validator := readyImplementationSpec("Validator", "Validate")
	target := readyImplementationSpec("Worker", "Execute")
	target.Dependencies = []node.Dependency{{
		Target:       "Validator",
		Type:         "interface",
		Injection:    "constructor",
		Usage:        "Validate each request before effects.",
		ContractHash: calculateSpecHash(validator),
		Requires:     []string{"Validate"},
	}}

	issues := validateImplementationClosure(target, buildSpecLookup([]*node.Spec{target, validator}))
	if len(issues) != 0 {
		t.Fatalf("complete contract was rejected: %v", issues)
	}
}

func TestValidateImplementationClosureRejectsIncompleteContracts(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(target, dependency *node.Spec)
		want       string
		includeDep bool
	}{
		{
			name: "placeholder",
			mutate: func(target, _ *node.Spec) {
				target.Interface.Methods[0].Description = "TBD"
			},
			want: "unresolved placeholder",
		},
		{
			name: "missing responsibility",
			mutate: func(target, _ *node.Spec) {
				target.Responsibility.Summary = ""
			},
			want: "responsibility.summary is required",
		},
		{
			name: "missing method behavior",
			mutate: func(target, _ *node.Spec) {
				target.Interface.Methods[0].Postconditions = nil
				target.Interface.Methods[0].SideEffects = nil
				target.Interface.Methods[0].Preconditions = nil
				target.Interface.Methods[0].Throws = nil
			},
			want: "behavioral contract",
		},
		{
			name: "blank method behavior",
			mutate: func(target, _ *node.Spec) {
				target.Interface.Methods[0].Postconditions = []string{""}
			},
			want: "postconditions[0]: value must not be blank",
		},
		{
			name: "missing return description",
			mutate: func(target, _ *node.Spec) {
				target.Interface.Methods[0].Returns.Description = ""
			},
			want: "returns.type and returns.description",
		},
		{
			name: "missing acceptance",
			mutate: func(target, _ *node.Spec) {
				target.ImplementationContract.Acceptance = nil
			},
			want: "acceptance scenario",
		},
		{
			name: "blank global constraint",
			mutate: func(target, _ *node.Spec) {
				target.ImplementationContract.Constraints = []string{""}
			},
			want: "constraints[0]: value must not be blank",
		},
		{
			name: "blank acceptance outcome",
			mutate: func(target, _ *node.Spec) {
				target.ImplementationContract.Acceptance[0].Then = []string{""}
			},
			want: "then[0]: value must not be blank",
		},
		{
			name: "missing dependency",
			mutate: func(target, dependency *node.Spec) {
				target.Dependencies = []node.Dependency{{
					Target:       dependency.Node.ID,
					ContractHash: calculateSpecHash(dependency),
					Requires:     []string{"Validate"},
				}}
			},
			want: "dependency is missing",
		},
		{
			name: "missing required member list",
			mutate: func(target, dependency *node.Spec) {
				target.Dependencies = []node.Dependency{{
					Target:       dependency.Node.ID,
					ContractHash: calculateSpecHash(dependency),
				}}
			},
			want:       "requires must name",
			includeDep: true,
		},
		{
			name: "missing dependency contract hash reports current value",
			mutate: func(target, dependency *node.Spec) {
				target.Dependencies = []node.Dependency{{
					Target:   dependency.Node.ID,
					Requires: []string{"Validate"},
				}}
			},
			want:       "current contract hash is " + calculateSpecHash(readyImplementationSpec("Validator", "Validate")),
			includeDep: true,
		},
		{
			name: "unknown required member",
			mutate: func(target, dependency *node.Spec) {
				target.Dependencies = []node.Dependency{{
					Target:       dependency.Node.ID,
					ContractHash: calculateSpecHash(dependency),
					Requires:     []string{"MissingMember"},
				}}
			},
			want:       "required member MissingMember",
			includeDep: true,
		},
		{
			name: "stale contract hash",
			mutate: func(target, dependency *node.Spec) {
				target.Dependencies = []node.Dependency{{
					Target:       dependency.Node.ID,
					ContractHash: "deadbeef",
					Requires:     []string{"Validate"},
				}}
			},
			want:       "contract_hash mismatch",
			includeDep: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependency := readyImplementationSpec("Validator", "Validate")
			target := readyImplementationSpec("Worker", "Execute")
			tt.mutate(target, dependency)
			nodes := []*node.Spec{target}
			if tt.includeDep {
				nodes = append(nodes, dependency)
			}
			issues := validateImplementationClosure(target, buildSpecLookup(nodes))
			if !strings.Contains(strings.Join(issues, "\n"), tt.want) {
				t.Fatalf("expected issue containing %q, got %v", tt.want, issues)
			}
		})
	}
}

func TestGatherImplementationDependenciesIncludesTransitiveCycleOnce(t *testing.T) {
	target := readyImplementationSpec("Target", "Run")
	first := readyImplementationSpec("First", "Execute")
	second := readyImplementationSpec("Second", "Validate")
	target.Dependencies = []node.Dependency{{Target: "First", Requires: []string{"Execute"}}}
	first.Dependencies = []node.Dependency{{Target: "Second", Requires: []string{"Validate"}}}
	second.Dependencies = []node.Dependency{{Target: "Target", Requires: []string{"Run"}}}
	target.Dependencies[0].ContractHash = calculateSpecHash(first)
	first.Dependencies[0].ContractHash = calculateSpecHash(second)
	second.Dependencies[0].ContractHash = calculateSpecHash(target)

	nodeMap := buildSpecLookup([]*node.Spec{second, target, first})
	if issues := validateImplementationClosure(target, nodeMap); len(issues) != 0 {
		t.Fatalf("valid cyclic closure was rejected: %v", issues)
	}
	deps := gatherImplementationDependencies(target, nodeMap, "go")
	if len(deps) != 2 || deps[0].Target != "First" || deps[1].Target != "Second" {
		t.Fatalf("expected deterministic once-only closure [First Second], got %+v", deps)
	}
}

func TestCalculateSpecHashIncludesSchema11BehaviorAndPreservesSchema10Compatibility(t *testing.T) {
	left := readyImplementationSpec("Worker", "Execute")
	right := readyImplementationSpec("Worker", "Execute")
	right.Interface.Methods[0].Postconditions[0] = "output is canonical and immutable"
	if calculateSpecHash(left) == calculateSpecHash(right) {
		t.Fatal("schema 1.1 behavior change did not alter the contract hash")
	}

	right = readyImplementationSpec("Worker", "Execute")
	right.Node.FilePath = "another-machine/worker.go"
	if calculateSpecHash(left) != calculateSpecHash(right) {
		t.Fatal("schema 1.1 contract hash depends on repository-local file paths")
	}

	right = readyImplementationSpec("Worker", "Execute")
	right.LanguageSpec.Package = "anotherpackage"
	if calculateSpecHash(left) == calculateSpecHash(right) {
		t.Fatal("schema 1.1 language contract change did not alter the contract hash")
	}

	right = readyImplementationSpec("Worker", "Execute")
	right.Dependencies = []node.Dependency{{Target: "Validator", Requires: []string{"Validate"}, ContractHash: "aaaaaaaa"}}
	withDependency := calculateSpecHash(right)
	right.Dependencies[0].Requires = []string{"Validate", "Normalize"}
	if withDependency == calculateSpecHash(right) {
		t.Fatal("schema 1.1 dependency member change did not alter the contract hash")
	}
	right.Dependencies[0].Requires = []string{"Validate"}
	right.Dependencies[0].ContractHash = "bbbbbbbb"
	if withDependency != calculateSpecHash(right) {
		t.Fatal("nested contract_hash value caused recursive hash churn")
	}

	left.SchemaVersion = "1.0"
	right.SchemaVersion = "1.0"
	if calculateSpecHash(left) != calculateSpecHash(right) {
		t.Fatal("schema 1.0 signature-only hash compatibility regressed")
	}
}

func TestValidateImplementationNodeRequiresExactSchema11(t *testing.T) {
	spec := readyImplementationSpec("Worker", "Execute")
	spec.SchemaVersion = "1.10"
	issues := validateImplementationNode(spec)
	if !strings.Contains(strings.Join(issues, "\n"), "schema_version 1.1 or 1.2 is required") {
		t.Fatalf("future schema version was incorrectly treated as supported: %v", issues)
	}
}

func readyImplementationSpec(id, methodName string) *node.Spec {
	return &node.Spec{
		SchemaVersion: "1.1",
		Node:          node.NodeInfo{ID: id, Type: "service", FilePath: strings.ToLower(id) + ".go"},
		Responsibility: node.Responsibility{
			Summary:    "Provide " + id + " behavior.",
			Invariants: []string{"Calls are deterministic."},
		},
		Interface: node.Interface{Methods: []node.Method{{
			Name:           methodName,
			Signature:      methodName + "(input Input) (Output, error)",
			Description:    "Process one immutable input.",
			Parameters:     []node.Parameter{{Name: "input", Type: "Input", Description: "Immutable input."}},
			Returns:        node.Returns{Type: "Output", Description: "Canonical output."},
			Preconditions:  []string{"input is valid"},
			Postconditions: []string{"output is canonical"},
		}}},
		ImplementationContract: &node.ImplementationContract{
			Status:      "ready",
			Constraints: []string{"No external implementation source is required."},
			Acceptance: []node.AcceptanceScenario{{
				ID:    strings.ToUpper(id) + "-SUCCESS",
				Given: "A valid immutable input.",
				When:  methodName + " is called.",
				Then:  []string{"One canonical output is returned."},
			}},
		},
		Metadata: node.Metadata{Status: "specified"},
	}
}
