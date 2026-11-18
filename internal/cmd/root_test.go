package cmd_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fabianoflorentino/seraph/internal/cmd"
	"github.com/fabianoflorentino/seraph/internal/state"
)

const packYAML = `
version: 1
services:
  - slug: github
    name: GitHub
    category: cloud
    official_url: https://github.com
    regions:
      global: primary
    source:
      type: statuspage
      url: https://www.githubstatus.com/api/v2/summary.json
`

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	root := cmd.NewRootCommand()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	err := root.Execute()

	return stdout.String(), err
}

func writePack(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "global.yaml"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write pack: %v", err)
	}

	return dir
}

func writeConfig(t *testing.T, outDir string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "seraph.yml")
	contents := fmt.Sprintf("out_dir: %q\nlog_level: error\nretries: 0\ntimeout: 1s\n", outDir)

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	out, err := run(t, "version")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(out, "commit") {
		t.Fatalf("output = %q, want build metadata", out)
	}
}

func TestValidateCommand(t *testing.T) {
	t.Parallel()

	packDir := writePack(t, packYAML)
	configPath := writeConfig(t, t.TempDir())

	out, err := run(t, "--config", configPath, "validate", "--catalog", packDir)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(out, "1 packs, 1 services") {
		t.Fatalf("output = %q, want catalog summary", out)
	}
}

func TestValidateCommandRejectsInvalidPack(t *testing.T) {
	t.Parallel()

	packDir := writePack(t, strings.Replace(packYAML, "category: cloud", "category: crypto", 1))
	configPath := writeConfig(t, t.TempDir())

	if _, err := run(t, "--config", configPath, "validate", "--catalog", packDir); err == nil {
		t.Fatal("expected an error")
	}
}

func TestValidateCommandRejectsInvalidLogLevel(t *testing.T) {
	t.Parallel()

	packDir := writePack(t, packYAML)
	configPath := writeConfig(t, t.TempDir())

	if _, err := run(t, "--config", configPath, "--log-level", "verbose", "validate", "--catalog", packDir); err == nil {
		t.Fatal("expected an error")
	}
}

func TestSyncCommandWritesSnapshot(t *testing.T) {
	t.Parallel()

	packDir := writePack(t, strings.Replace(
		packYAML,
		"https://www.githubstatus.com/api/v2/summary.json",
		"https://127.0.0.1:1/summary.json",
		1,
	))
	outDir := filepath.Join(t.TempDir(), "out")
	configPath := writeConfig(t, outDir)

	out, err := run(t, "--config", configPath, "sync", "--catalog", packDir)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(out, "1 services") {
		t.Fatalf("output = %q, want service count", out)
	}

	snap, err := state.Read(filepath.Join(outDir, "state.json"))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(snap.Services) != 1 || snap.Services[0].Slug != "github" {
		t.Fatalf("services = %+v, want github", snap.Services)
	}

	if snap.Services[0].Status != state.StatusUnknown {
		t.Errorf("Status = %q, want unknown", snap.Services[0].Status)
	}

	if snap.Services[0].Error == "" {
		t.Error("Error is empty, want a collection error")
	}

	if snap.Sources.OK != 0 || snap.Sources.Failed != 1 {
		t.Errorf("Sources = %+v, want 0 ok and 1 failed", snap.Sources)
	}
}

func TestSyncCommandFailsWithoutCatalog(t *testing.T) {
	t.Parallel()

	configPath := writeConfig(t, t.TempDir())
	emptyDir := t.TempDir()

	if _, err := run(t, "--config", configPath, "sync", "--catalog", emptyDir); err == nil {
		t.Fatal("expected an error")
	}
}
