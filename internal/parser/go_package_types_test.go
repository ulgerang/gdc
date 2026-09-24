package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoParserCrossFileNamedTypes(t *testing.T) {
	dir := t.TempDir()
	put := func(name, source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put("a.go", "package dag\ntype A struct{}\nfunc (a A) Value() int {return 1}\n")
	put("b.go", "package dag\ntype B struct{}\nfunc (b B) Value() int {return 2}\n")
	put("c.go", "package dag\ntype C struct{a A;b B}\nfunc (c C) Value() int {return c.a.Value()+c.b.Value()}\n")
	nodes, err := NewGoParser().ParseFileNodes(filepath.Join(dir, "c.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != "C" {
		t.Fatalf("sibling declarations leaked into selected file: %+v", nodes)
	}
	deps := map[string]string{}
	for _, dep := range nodes[0].Dependencies {
		deps[dep.Target] = dep.Type
		if dep.Namespace != "dag" || dep.Injection != "field" {
			t.Fatalf("wrong sibling dependency scope: %+v", dep)
		}
	}
	if len(deps) != 2 || deps["A"] != "class" || deps["B"] != "class" {
		t.Fatalf("cross-file A/B typed fields must yield real class dependencies, got %+v", nodes[0].Dependencies)
	}
}

func TestGoParserPackageContextExcludesUnrelatedTypesAndRefreshes(t *testing.T) {
	dir := t.TempDir()
	put := func(name, source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put("a.go", "package dag\ntype A interface{Value() int}\ntype F func()\ntype N int\n")
	put("fixture_test.go", "package dag\ntype TestOnly struct{}\n")
	put("foreign.go", "package other\ntype Foreign struct{}\n")
	put("ignored.go", "//go:build ignore\n\npackage dag\ntype Excluded struct{}\n")
	put(".hidden.go", "package dag\ntype Hidden struct{}\n")
	put("c.go", "package dag\ntype C struct{a A;f F;n N;t TestOnly;x Foreign;e Excluded;h Hidden}\n")
	parser := NewGoParser()
	check := func(want string) {
		t.Helper()
		nodes, err := parser.ParseFileNodes(filepath.Join(dir, "c.go"))
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != 1 || len(nodes[0].Dependencies) != 1 || nodes[0].Dependencies[0].Target != "A" || nodes[0].Dependencies[0].Type != want {
			t.Fatalf("package context contains unrelated or stale declarations: %+v", nodes)
		}
	}
	check("interface")
	put("a.go", "package dag\ntype A struct{}\ntype F func()\ntype N int\n")
	check("class")
	put("invalid.go", "package dag\ntype Broken struct {\n")
	if _, err := parser.ParseFileNodes(filepath.Join(dir, "c.go")); err == nil {
		t.Fatal("same-package syntax failure was silently ignored")
	}
}
