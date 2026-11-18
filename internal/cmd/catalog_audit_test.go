package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogAuditCommand(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"page":{"name":"Example","url":"https://status.example.com"},"status":{"indicator":"none"}}`)
	}))
	t.Cleanup(server.Close)

	packDir := t.TempDir()
	packPath := filepath.Join(packDir, "global.yaml")

	if err := os.WriteFile(packPath, []byte(fmt.Sprintf(syncPackYAML, server.URL)), 0o600); err != nil {
		t.Fatalf("write pack: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "seraph.yml")
	if err := os.WriteFile(configPath, []byte("out_dir: /tmp/audit-out\nlog_level: error\nretries: 0\ntimeout: 2s\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer

	root := newRootCommand(&rootOptions{httpClient: server.Client()})
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	root.SetArgs([]string{
		"--config", configPath,
		"catalog", "audit", "--catalog", packDir, "--json",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var output auditOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode output: %v", err)
	}

	if output.Total != 1 || output.OK != 1 || output.Failed != 0 {
		t.Errorf("output = %+v, want one ok service", output)
	}

	if len(output.Services) != 1 || output.Services[0].Status == "" {
		t.Errorf("services = %+v, want a populated service", output.Services)
	}
}
