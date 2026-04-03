package extract

import "testing"

func TestDetectLanguageFromPathSupportsRust(t *testing.T) {
	if got := detectLanguageFromPath("src/lib.rs"); got != "rust" {
		t.Fatalf("expected rust language, got %q", got)
	}
}

func TestGenerateSearchPatternsIncludesRustConventions(t *testing.T) {
	patterns := generateSearchPatterns("UserService", "class")

	found := false
	for _, pattern := range patterns {
		if pattern == "user_service.rs" || pattern == "UserService.rs" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rust search patterns to include a .rs candidate, got %v", patterns)
	}
}
