package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryLaunchersRequireExplicitPrebuiltOptIn(t *testing.T) {
	tests := []struct {
		path       string
		sourceRun  string
		prebuilt   string
		optInGuard string
	}{
		{path: "gdc.bat", sourceRun: "go run", prebuilt: "gdc.exe", optInGuard: "GDC_USE_PREBUILT"},
		{path: "gdc.sh", sourceRun: "go run", prebuilt: "gdc-linux-amd64", optInGuard: "GDC_USE_PREBUILT"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			path := filepath.Join("..", "..", test.path)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read launcher: %v", err)
			}
			content := string(data)
			if !strings.Contains(content, test.optInGuard) || !strings.Contains(content, test.sourceRun) || !strings.Contains(content, test.prebuilt) {
				t.Fatalf("launcher must contain explicit prebuilt opt-in and current-source fallback: %s", content)
			}
			guardIndex := strings.Index(content, test.optInGuard)
			prebuiltIndex := strings.Index(content, test.prebuilt)
			if guardIndex < 0 || prebuiltIndex < guardIndex {
				t.Fatalf("prebuilt executable is not guarded by explicit opt-in: %s", content)
			}
		})
	}
}
