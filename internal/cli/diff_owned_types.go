package cli

import "github.com/gdc-tools/gdc/internal/node"

// specOwnedTypes lists the types a node spec declares in its own interface.
// Such a type is part of the node's contract, not a collaborator: a Parser
// whose Parse method returns *Node, with Node declared in the Parser spec,
// does not depend on another node named Node. The Go parser reports it as a
// dependency (a pointer to a local struct), which made every such spec drift
// with no way to declare it (found by p-solver v2 live-015).
func specOwnedTypes(spec *node.Spec) map[string]bool {
	owned := make(map[string]bool, len(spec.Interface.Types))
	for _, contract := range spec.Interface.Types {
		if contract.Name != "" {
			owned[contract.Name] = true
		}
	}
	return owned
}
