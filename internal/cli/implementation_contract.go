package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdc-tools/gdc/internal/node"
	"gopkg.in/yaml.v3"
)

var unresolvedImplementationTokens = []string{
	"requires_approval",
	"requires approval",
	"needs description",
	"[needs description]",
	"<tbd>",
	"<todo>",
	" tbd",
	" todo",
}

func validateImplementationClosure(target *node.Spec, nodeMap map[string]*node.Spec) []string {
	if target == nil {
		return []string{"target: implementation contract is missing"}
	}

	var issues []string
	visited := make(map[string]bool)
	var visit func(*node.Spec)
	visit = func(spec *node.Spec) {
		id := spec.QualifiedID()
		if visited[id] {
			return
		}
		visited[id] = true
		issues = append(issues, validateImplementationNode(spec)...)

		deps := append([]node.Dependency(nil), spec.Dependencies...)
		sort.SliceStable(deps, func(i, j int) bool { return deps[i].Target < deps[j].Target })
		for _, dep := range deps {
			field := id + ".dependencies[" + dep.Target + "]"
			depSpec, exists := nodeMap[dep.Target]
			if !exists || depSpec == nil {
				issues = append(issues, field+": dependency is missing")
				continue
			}
			if len(dep.Requires) == 0 {
				issues = append(issues, field+": requires must name every dependency member used by the implementation")
			}
			currentHash := calculateSpecHash(depSpec)
			if strings.TrimSpace(dep.ContractHash) == "" {
				issues = append(issues, fmt.Sprintf("%s: contract_hash is required for implementation-ready dependencies; current contract hash is %s", field, currentHash))
			} else {
				if dep.ContractHash != currentHash {
					issues = append(issues, fmt.Sprintf("%s: contract_hash mismatch: expected %s, current %s", field, dep.ContractHash, currentHash))
				}
			}
			for _, required := range dep.Requires {
				if !dependencyMemberExists(depSpec, required) {
					issues = append(issues, fmt.Sprintf("%s: required member %s is not declared by %s", field, required, depSpec.QualifiedID()))
				}
			}
			visit(depSpec)
		}
	}

	visit(target)
	return issues
}

func validateImplementationNode(spec *node.Spec) []string {
	id := spec.QualifiedID()
	var issues []string
	if strings.TrimSpace(spec.SchemaVersion) != "1.1" {
		issues = append(issues, id+": schema_version 1.1 is required for implementation-ready extraction")
	}
	if spec.ImplementationContract == nil || spec.ImplementationContract.Status != "ready" {
		issues = append(issues, id+": implementation_contract.status must be ready")
	} else {
		if !hasNonBlankContractValue(spec.ImplementationContract.Lifecycle) && !hasNonBlankContractValue(spec.ImplementationContract.Constraints) {
			issues = append(issues, id+": implementation_contract requires lifecycle or global constraints")
		}
		issues = append(issues, blankContractValueIssues(id+".implementation_contract.lifecycle", spec.ImplementationContract.Lifecycle)...)
		issues = append(issues, blankContractValueIssues(id+".implementation_contract.constraints", spec.ImplementationContract.Constraints)...)
		if len(spec.ImplementationContract.Acceptance) == 0 {
			issues = append(issues, id+": at least one acceptance scenario is required")
		}
		for i, scenario := range spec.ImplementationContract.Acceptance {
			field := fmt.Sprintf("%s.implementation_contract.acceptance[%d]", id, i)
			if strings.TrimSpace(scenario.ID) == "" || strings.TrimSpace(scenario.Given) == "" || strings.TrimSpace(scenario.When) == "" || !hasNonBlankContractValue(scenario.Then) {
				issues = append(issues, field+": id, given, when, and then are required")
			}
			issues = append(issues, blankContractValueIssues(field+".then", scenario.Then)...)
		}
	}
	if strings.TrimSpace(spec.Responsibility.Summary) == "" {
		issues = append(issues, id+": responsibility.summary is required")
	}
	issues = append(issues, blankContractValueIssues(id+".responsibility.invariants", spec.Responsibility.Invariants)...)

	memberCount := len(spec.Interface.Types) + len(spec.Interface.Constructors) + len(spec.Interface.Methods) + len(spec.Interface.Properties) + len(spec.Interface.Events)
	if memberCount == 0 {
		issues = append(issues, id+": at least one public interface member is required")
	}
	for i, contractType := range spec.Interface.Types {
		if strings.TrimSpace(contractType.Name) == "" || strings.TrimSpace(contractType.Signature) == "" || strings.TrimSpace(contractType.Description) == "" {
			issues = append(issues, fmt.Sprintf("%s.interface.types[%d]: name, signature, and description are required", id, i))
		}
		for f, field := range contractType.Fields {
			if strings.TrimSpace(field.Name) == "" || strings.TrimSpace(field.Type) == "" || strings.TrimSpace(field.Description) == "" {
				issues = append(issues, fmt.Sprintf("%s.interface.types[%d].fields[%d]: name, type, and description are required", id, i, f))
			}
		}
		issues = append(issues, blankContractValueIssues(fmt.Sprintf("%s.interface.types[%d].values", id, i), contractType.Values)...)
	}
	for i, constructor := range spec.Interface.Constructors {
		field := fmt.Sprintf("%s.interface.constructors[%d]", id, i)
		if strings.TrimSpace(constructor.Signature) == "" || strings.TrimSpace(constructor.Description) == "" {
			issues = append(issues, field+": signature and description are required")
		}
		for p, parameter := range constructor.Parameters {
			if strings.TrimSpace(parameter.Name) == "" || strings.TrimSpace(parameter.Type) == "" || strings.TrimSpace(parameter.Description) == "" {
				issues = append(issues, fmt.Sprintf("%s.parameters[%d]: name, type, and description are required", field, p))
			}
		}
	}
	for i, method := range spec.Interface.Methods {
		field := fmt.Sprintf("%s.interface.methods[%d]", id, i)
		if strings.TrimSpace(method.Name) == "" || strings.TrimSpace(method.Signature) == "" || strings.TrimSpace(method.Description) == "" {
			issues = append(issues, field+": name, signature, and description are required")
		}
		if strings.TrimSpace(method.Returns.Type) == "" || strings.TrimSpace(method.Returns.Description) == "" {
			issues = append(issues, field+": returns.type and returns.description are required")
		}
		hasBehavior := hasNonBlankContractValue(method.Preconditions) || hasNonBlankContractValue(method.Postconditions) || hasNonBlankContractValue(method.SideEffects)
		issues = append(issues, blankContractValueIssues(field+".preconditions", method.Preconditions)...)
		issues = append(issues, blankContractValueIssues(field+".postconditions", method.Postconditions)...)
		issues = append(issues, blankContractValueIssues(field+".side_effects", method.SideEffects)...)
		for declaredIndex, declared := range method.Throws {
			if strings.TrimSpace(declared.Type) == "" || strings.TrimSpace(declared.Condition) == "" {
				issues = append(issues, fmt.Sprintf("%s.throws[%d]: type and condition are required", field, declaredIndex))
			} else {
				hasBehavior = true
			}
		}
		if !hasBehavior {
			issues = append(issues, field+": behavioral contract requires a precondition, postcondition, side effect, or declared error")
		}
		for p, parameter := range method.Parameters {
			if strings.TrimSpace(parameter.Name) == "" || strings.TrimSpace(parameter.Type) == "" || strings.TrimSpace(parameter.Description) == "" {
				issues = append(issues, fmt.Sprintf("%s.parameters[%d]: name, type, and description are required", field, p))
			}
		}
	}
	for i, property := range spec.Interface.Properties {
		if strings.TrimSpace(property.Name) == "" || strings.TrimSpace(property.Type) == "" || strings.TrimSpace(property.Access) == "" || strings.TrimSpace(property.Description) == "" {
			issues = append(issues, fmt.Sprintf("%s.interface.properties[%d]: name, type, access, and description are required", id, i))
		}
	}
	for i, event := range spec.Interface.Events {
		if strings.TrimSpace(event.Name) == "" || strings.TrimSpace(event.Signature) == "" || strings.TrimSpace(event.Description) == "" {
			issues = append(issues, fmt.Sprintf("%s.interface.events[%d]: name, signature, and description are required", id, i))
		}
	}

	encoded, err := yaml.Marshal(spec)
	if err != nil {
		issues = append(issues, id+": contract cannot be serialized: "+err.Error())
	} else {
		lower := strings.ToLower(string(encoded))
		for _, token := range unresolvedImplementationTokens {
			if strings.Contains(lower, token) {
				issues = append(issues, fmt.Sprintf("%s: unresolved placeholder %q", id, strings.TrimSpace(token)))
				break
			}
		}
	}
	return issues
}

func hasNonBlankContractValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func blankContractValueIssues(field string, values []string) []string {
	var issues []string
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			issues = append(issues, fmt.Sprintf("%s[%d]: value must not be blank", field, i))
		}
	}
	return issues
}

func dependencyMemberExists(spec *node.Spec, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return false
	}
	for _, method := range spec.Interface.Methods {
		if method.Name == required {
			return true
		}
	}
	for _, contractType := range spec.Interface.Types {
		if contractType.Name == required {
			return true
		}
	}
	for _, property := range spec.Interface.Properties {
		if property.Name == required {
			return true
		}
	}
	for _, event := range spec.Interface.Events {
		if event.Name == required {
			return true
		}
	}
	for _, constructor := range spec.Interface.Constructors {
		if required == "constructor" || strings.Contains(constructor.Signature, required) {
			return true
		}
	}
	return false
}

func gatherImplementationDependencies(spec *node.Spec, nodeMap map[string]*node.Spec, language string) []DependencyInfo {
	seen := map[string]bool{spec.QualifiedID(): true}
	var deps []DependencyInfo
	var gather func(*node.Spec)
	gather = func(current *node.Spec) {
		edges := append([]node.Dependency(nil), current.Dependencies...)
		sort.SliceStable(edges, func(i, j int) bool { return edges[i].Target < edges[j].Target })
		for _, dep := range edges {
			depSpec := nodeMap[dep.Target]
			if depSpec == nil || seen[depSpec.QualifiedID()] {
				continue
			}
			seen[depSpec.QualifiedID()] = true
			deps = append(deps, DependencyInfo{
				Target:              dep.Target,
				Type:                dep.Type,
				Injection:           dep.Injection,
				Optional:            dep.Optional,
				Usage:               dep.Usage,
				ContractHash:        dep.ContractHash,
				Requires:            append([]string(nil), dep.Requires...),
				RequiredMembers:     strings.Join(dep.Requires, ", "),
				InterfaceCode:       generateInterfaceCodeForLanguage(depSpec, language),
				ContractDetails:     generateContractDetails(depSpec),
				ContractYAML:        generateContractYAML(depSpec),
				Spec:                depSpec,
				MissingDescriptions: collectMissingDescriptions(depSpec),
			})
			gather(depSpec)
		}
	}
	gather(spec)
	return deps
}

func generateContractDetails(spec *node.Spec) string {
	if spec == nil {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "**Responsibility:** %s\n", spec.Responsibility.Summary)
	if spec.Responsibility.Details != "" {
		fmt.Fprintf(&out, "\n%s\n", spec.Responsibility.Details)
	}
	if len(spec.Responsibility.Invariants) > 0 {
		out.WriteString("\n**Invariants:**\n")
		for _, invariant := range spec.Responsibility.Invariants {
			fmt.Fprintf(&out, "- %s\n", invariant)
		}
	}
	if len(spec.Interface.Types) > 0 {
		out.WriteString("\n**Public Types:**\n")
		for _, contractType := range spec.Interface.Types {
			fmt.Fprintf(&out, "- %s: `%s` — %s\n", contractType.Name, contractType.Signature, contractType.Description)
			for _, field := range contractType.Fields {
				fmt.Fprintf(&out, "  - %s (%s): %s", field.Name, field.Type, field.Description)
				if field.Constraint != "" {
					fmt.Fprintf(&out, " [%s]", field.Constraint)
				}
				out.WriteString("\n")
			}
			if len(contractType.Values) > 0 {
				fmt.Fprintf(&out, "  - Values: %s\n", strings.Join(contractType.Values, ", "))
			}
		}
	}
	for _, method := range spec.Interface.Methods {
		fmt.Fprintf(&out, "\n### Method %s\n\n%s\n", method.Name, method.Description)
		if len(method.Preconditions) > 0 {
			out.WriteString("\n**Preconditions:**\n")
			writeContractList(&out, method.Preconditions)
		}
		if len(method.Postconditions) > 0 {
			out.WriteString("\n**Postconditions:**\n")
			writeContractList(&out, method.Postconditions)
		}
		if len(method.SideEffects) > 0 {
			out.WriteString("\n**Side Effects:**\n")
			writeContractList(&out, method.SideEffects)
		}
		if len(method.Throws) > 0 {
			out.WriteString("\n**Errors:**\n")
			for _, declared := range method.Throws {
				fmt.Fprintf(&out, "- %s: %s\n", declared.Type, declared.Condition)
			}
		}
	}
	if len(spec.Logic.Rules) > 0 {
		out.WriteString("\n**Rules:**\n")
		for _, rule := range spec.Logic.Rules {
			fmt.Fprintf(&out, "- %s: when %s, %s\n", rule.Name, rule.Condition, rule.Action)
		}
	}
	if contract := spec.ImplementationContract; contract != nil {
		if len(contract.Lifecycle) > 0 {
			out.WriteString("\n**Lifecycle:**\n")
			writeContractList(&out, contract.Lifecycle)
		}
		if len(contract.Constraints) > 0 {
			out.WriteString("\n**Global Constraints:**\n")
			writeContractList(&out, contract.Constraints)
		}
		if len(contract.Acceptance) > 0 {
			out.WriteString("\n**Acceptance Scenarios:**\n")
			for _, scenario := range contract.Acceptance {
				fmt.Fprintf(&out, "- %s\n  - Given: %s\n  - When: %s\n", scenario.ID, scenario.Given, scenario.When)
				for _, then := range scenario.Then {
					fmt.Fprintf(&out, "  - Then: %s\n", then)
				}
			}
		}
	}
	if len(spec.Dependencies) > 0 {
		out.WriteString("\n**Dependency Edges:**\n")
		for _, dep := range spec.Dependencies {
			fmt.Fprintf(&out, "- %s (hash %s; requires: %s)", dep.Target, dep.ContractHash, strings.Join(dep.Requires, ", "))
			if dep.Usage != "" {
				fmt.Fprintf(&out, ": %s", dep.Usage)
			}
			out.WriteString("\n")
		}
	}
	return strings.TrimSpace(out.String())
}

func writeContractList(out *strings.Builder, values []string) {
	for _, value := range values {
		fmt.Fprintf(out, "- %s\n", value)
	}
}

func generateContractYAML(spec *node.Spec) string {
	if spec == nil {
		return ""
	}
	encoded, err := yaml.Marshal(spec)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func buildContractMap(spec *node.Spec) map[string]any {
	encoded := generateContractYAML(spec)
	if encoded == "" {
		return nil
	}
	var contract map[string]any
	if err := yaml.Unmarshal([]byte(encoded), &contract); err != nil {
		return nil
	}
	return contract
}
