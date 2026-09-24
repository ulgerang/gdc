package parser

import (
	"fmt"
	"go/ast"
	"go/build"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// packageTypeKinds supplies declaration evidence from sibling files rather
// than guessing dependency kinds from suffixes. It intentionally owns no cache:
// separate syncs must observe edits and concurrent parsers must not share maps.
// Only the current build's non-test files in this exact package contribute.
func packageTypeKinds(filePath, packageName string) (map[string]string, error) {
	directory := filepath.Dir(filePath)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	kinds := make(map[string]string)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		matches, err := build.Default.MatchFile(directory, name)
		if err != nil {
			return nil, fmt.Errorf("read Go package file constraints %s: %w", name, err)
		}
		if !matches {
			continue
		}
		path := filepath.Join(directory, name)
		file, err := goparser.ParseFile(token.NewFileSet(), path, nil, goparser.PackageClauseOnly)
		if err != nil {
			return nil, fmt.Errorf("read Go sibling package %s: %w", name, err)
		}
		if file.Name.Name != packageName {
			continue
		}
		file, err = goparser.ParseFile(token.NewFileSet(), path, nil, goparser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("read Go sibling declarations %s: %w", name, err)
		}
		for _, declaration := range file.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok || group.Tok != token.TYPE {
				continue
			}
			for _, spec := range group.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch typeSpec.Type.(type) {
				case *ast.StructType:
					kinds[typeSpec.Name.Name] = "class"
				case *ast.InterfaceType:
					kinds[typeSpec.Name.Name] = "interface"
				case *ast.FuncType:
					kinds[typeSpec.Name.Name] = "func"
				default:
					kinds[typeSpec.Name.Name] = "other"
				}
			}
		}
	}
	return kinds, nil
}
