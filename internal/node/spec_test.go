package node

import "testing"

func TestNodeInfoQualifiedIDPrefixesNamespaceForBareIDs(t *testing.T) {
	info := NodeInfo{
		ID:        "Runtime",
		Namespace: "tools",
	}

	if got := info.QualifiedID(); got != "tools.Runtime" {
		t.Fatalf("expected tools.Runtime, got %q", got)
	}
}

func TestNodeInfoQualifiedIDDoesNotDoublePrefixDottedIDs(t *testing.T) {
	info := NodeInfo{
		ID:        "tools.Runtime",
		Namespace: "tools",
	}

	if got := info.QualifiedID(); got != "tools.Runtime" {
		t.Fatalf("expected tools.Runtime, got %q", got)
	}
}
