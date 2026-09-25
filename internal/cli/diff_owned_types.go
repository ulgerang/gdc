package cli

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"strings"

	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/parser"
)

// specOwnedTypes lists the types that belong to the node itself rather than to
// a collaborator: a Parser whose Parse method returns *Node does not depend on
// another node named Node (found by p-solver v2 live-015). The Go parser reports
// such a local struct as a dependency.
//
// The spec alone is not evidence of ownership: listing a collaborator's type in
// interface.types would otherwise hide a missing dependency. A type is owned only
// when the spec declares it AND the node's own implementation file declares it.
// Other languages keep the strict comparison.
func specOwnedTypes(spec *node.Spec, extracted *parser.ExtractedNode) map[string]bool {
	owned := map[string]bool{}
	if extracted == nil || extracted.Language != "go" || strings.TrimSpace(extracted.FilePath) == "" {
		return owned
	}
	declared, err := goFileTypeNames(extracted.FilePath)
	if err != nil {
		return owned
	}
	for _, contract := range spec.Interface.Types {
		if contract.Name != "" && contract.Name != extracted.ID && declared[contract.Name] {
			owned[contract.Name] = true
		}
	}
	return owned
}

// goFileTypeNames returns the top-level type names declared in one Go file.
func goFileTypeNames(path string) (map[string]bool, error) {
	file, err := goparser.ParseFile(token.NewFileSet(), path, nil, goparser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok {
				names[typeSpec.Name.Name] = true
			}
		}
	}
	return names, nil
}
