package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRootAndCmdBuildProduceCLI(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for _, tc := range []struct {
		name string
		pkg  string
	}{
		{name: "root", pkg: "."},
		{name: "cmd", pkg: "./cmd/gdc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := filepath.Join(t.TempDir(), "gdc-test")
			if runtime.GOOS == "windows" {
				bin += ".exe"
			}

			build := exec.Command("go", "build", "-o", bin, tc.pkg)
			build.Dir = repoRoot
			if output, err := build.CombinedOutput(); err != nil {
				t.Fatalf("build %s failed: %v\n%s", tc.pkg, err, string(output))
			}

			assertCLIOutput(t, repoRoot, bin, "version")
			assertCLIOutput(t, repoRoot, bin, "--version")
		})
	}
}

func assertCLIOutput(t *testing.T, repoRoot, bin string, arg string) {
	t.Helper()

	cmd := exec.Command(bin, arg)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", arg, err, string(output))
	}

	text := string(output)
	if !strings.Contains(text, "gdc version") {
		t.Fatalf("expected version output for %s, got %q", arg, text)
	}
	if strings.Contains(text, "TypeScript Parser Results") {
		t.Fatalf("expected CLI output for %s, got debug sample output: %q", arg, text)
	}
}
