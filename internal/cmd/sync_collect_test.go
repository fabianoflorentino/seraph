package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fabianoflorentino/seraph/internal/state"
)

const syncPackYAML = `
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
      url: %s
`

func TestSyncCollectsStatuspage(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":{"indicator":"minor"}}`)
	}))
	t.Cleanup(server.Close)

	packDir := t.TempDir()
	packPath := filepath.Join(packDir, "global.yaml")

	if err := os.WriteFile(packPath, []byte(fmt.Sprintf(syncPackYAML, server.URL)), 0o600); err != nil {
		t.Fatalf("write pack: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "out")
	configPath := filepath.Join(t.TempDir(), "seraph.yml")
	contents := fmt.Sprintf("out_dir: %q\nlog_level: error\nretries: 0\ntimeout: 2s\n", outDir)

	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer

	root := newRootCommand(&rootOptions{httpClient: server.Client()})
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--config", configPath, "sync", "--catalog", packDir})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(stdout.String(), "1 services") {
		t.Fatalf("output = %q, want service count", stdout.String())
	}

	snap, err := state.Read(filepath.Join(outDir, "state.json"))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(snap.Services) != 1 {
		t.Fatalf("services = %d, want 1", len(snap.Services))
	}

	service := snap.Services[0]

	if service.Status != state.StatusDegraded {
		t.Errorf("Status = %q, want degraded", service.Status)
	}

	if service.Error != "" {
		t.Errorf("Error = %q, want empty", service.Error)
	}

	if service.StatusURL != "https://www.githubstatus.com" {
		t.Errorf("StatusURL = %q, want status page", service.StatusURL)
	}

	if service.ObservedAt == nil {
		t.Error("ObservedAt is nil, want a timestamp")
	}

	if len(service.Points) != 1 {
		t.Errorf("Points = %d, want 1", len(service.Points))
	}

	if snap.Sources.OK != 1 || snap.Sources.Failed != 0 {
		t.Errorf("Sources = %+v, want 1 ok and 0 failed", snap.Sources)
	}
}
