package cli

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gdc-tools/gdc/internal/node"
)

func TestFindPathResolvesShortDependencyTargetsToCanonicalNodes(t *testing.T) {
	nodes := []*node.Spec{
		{
			Node: node.NodeInfo{ID: "PlayerController", Type: "class", Namespace: "Game.Controllers"},
			Dependencies: []node.Dependency{
				{Target: "IInputManager", Type: "interface"},
			},
		},
		{
			Node: node.NodeInfo{ID: "IInputManager", Type: "interface", Namespace: "Game.Input"},
		},
	}
	canonical := buildCanonicalSpecMap(nodes)
	lookup := buildSpecLookup(nodes)

	path := findPath("Game.Controllers.PlayerController", "Game.Input.IInputManager", canonical, lookup)
	expected := []string{"Game.Controllers.PlayerController", "Game.Input.IInputManager"}
	if !reflect.DeepEqual(path, expected) {
		t.Fatalf("expected canonical path %v, got %v", expected, path)
	}
}

func TestPrintDependencyTreeResolvesShortDependencyTargets(t *testing.T) {
	nodes := []*node.Spec{
		{
			Node: node.NodeInfo{ID: "PlayerController", Type: "class", Namespace: "Game.Controllers"},
			Dependencies: []node.Dependency{
				{Target: "IInputManager", Type: "interface"},
			},
		},
		{
			Node: node.NodeInfo{ID: "IInputManager", Type: "interface", Namespace: "Game.Input"},
		},
	}
	canonical := buildCanonicalSpecMap(nodes)
	lookup := buildSpecLookup(nodes)

	output := captureStdout(t, func() {
		printDependencyTree("Game.Controllers.PlayerController", canonical, lookup, 0, 0, make(map[string]bool))
	})

	if !strings.Contains(output, "Game.Input.IInputManager") {
		t.Fatalf("expected canonical dependency in output, got:\n%s", output)
	}
	if strings.Contains(output, "(missing)") {
		t.Fatalf("expected dependency to resolve without missing marker, got:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close stdout pipe: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read stdout pipe: %v", err)
	}
	return string(out)
}
