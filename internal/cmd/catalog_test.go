package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fabianoflorentino/seraph/internal/catalog"
)

const catalogAddPackYAML = `
version: 1
services:
  - slug: github
    name: GitHub
    category: cloud
    official_url: https://github.com
    status_url: https://www.githubstatus.com
    regions:
      global: primary
    source:
      type: statuspage
      url: https://www.githubstatus.com/api/v2/summary.json
`

func writeCatalogPack(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "global.yaml")
	if err := os.WriteFile(path, []byte(catalogAddPackYAML), 0o600); err != nil {
		t.Fatalf("write pack: %v", err)
	}

	return path
}

func runCatalogAdd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var stdout bytes.Buffer

	root := newRootCommand(&rootOptions{})
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"catalog", "add"}, args...))

	err := root.Execute()

	return stdout.String(), err
}

func TestCatalogAddCommand(t *testing.T) {
	t.Parallel()

	path := writeCatalogPack(t)
	output, err := runCatalogAdd(
		t,
		"--pack", path,
		"--slug", "slack",
		"--name", "Slack",
		"--category", "messaging",
		"--official-url", "https://slack.com",
		"--status-url", "https://status.slack.com",
		"--source-type", "json",
		"--source-url", "https://status.slack.com/api/v2.0.0/current",
		"--format", "slack",
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(output, "added slack") {
		t.Errorf("output = %q, want added slack", output)
	}

	cat, err := catalog.Load(os.DirFS(filepath.Dir(path)), ".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cat.Services) != 2 {
		t.Fatalf("services = %d, want 2", len(cat.Services))
	}

	if cat.Services[1].Source.Format != catalog.JSONFormatSlack {
		t.Errorf("Source.Format = %q, want slack", cat.Services[1].Source.Format)
	}
}

func TestCatalogAddCommandRequiresFlags(t *testing.T) {
	t.Parallel()

	path := writeCatalogPack(t)
	_, err := runCatalogAdd(t, "--pack", path, "--slug", "slack")
	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "--name is required") {
		t.Errorf("error = %v, want missing name", err)
	}
}

func TestCatalogAddCommandRejectsInvalidService(t *testing.T) {
	t.Parallel()

	path := writeCatalogPack(t)
	_, err := runCatalogAdd(
		t,
		"--pack", path,
		"--slug", "slack",
		"--name", "Slack",
		"--category", "crypto",
		"--source-type", "json",
		"--source-url", "https://status.slack.com/api/v2.0.0/current",
		"--format", "slack",
	)
	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "category") {
		t.Errorf("error = %v, want category", err)
	}
}
