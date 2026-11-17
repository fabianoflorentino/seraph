package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fabianoflorentino/seraph/internal/config"
)

var envKeys = []string{
	"SERAPH_OUT_DIR",
	"SERAPH_BASE_URL",
	"SERAPH_TIMEOUT",
	"SERAPH_CONCURRENCY",
	"SERAPH_RETRIES",
	"SERAPH_LOG_LEVEL",
}

func isolateEnv(t *testing.T) {
	t.Helper()

	for _, key := range envKeys {
		t.Setenv(key, "")
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "seraph.yml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}

//nolint:paralleltest // the test isolates the process environment.
func TestLoadDefaults(t *testing.T) {
	isolateEnv(t)

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.OutDir != "site/data" {
		t.Errorf("OutDir = %q, want site/data", cfg.OutDir)
	}

	if cfg.Concurrency != 12 {
		t.Errorf("Concurrency = %d, want 12", cfg.Concurrency)
	}

	if cfg.Timeout.Duration() != 8*time.Second {
		t.Errorf("Timeout = %s, want 8s", cfg.Timeout)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

//nolint:paralleltest // the test isolates the process environment.
func TestLoadFile(t *testing.T) {
	isolateEnv(t)

	path := writeConfig(t, `
out_dir: public/data
base_url: https://example.com/state.json
timeout: 3s
concurrency: 4
retries: 1
log_level: debug
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.OutDir != "public/data" {
		t.Errorf("OutDir = %q, want public/data", cfg.OutDir)
	}

	if cfg.Timeout.Duration() != 3*time.Second {
		t.Errorf("Timeout = %s, want 3s", cfg.Timeout)
	}

	if cfg.Concurrency != 4 {
		t.Errorf("Concurrency = %d, want 4", cfg.Concurrency)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestLoadEnvOverridesFile(t *testing.T) {
	isolateEnv(t)

	path := writeConfig(t, "concurrency: 4\nlog_level: warn\n")

	t.Setenv("SERAPH_CONCURRENCY", "7")
	t.Setenv("SERAPH_TIMEOUT", "2s")
	t.Setenv("SERAPH_LOG_LEVEL", "error")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Concurrency != 7 {
		t.Errorf("Concurrency = %d, want 7", cfg.Concurrency)
	}

	if cfg.Timeout.Duration() != 2*time.Second {
		t.Errorf("Timeout = %s, want 2s", cfg.Timeout)
	}

	if cfg.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want error", cfg.LogLevel)
	}
}

//nolint:paralleltest // the test isolates the process environment.
func TestLoadRejectsUnknownField(t *testing.T) {
	isolateEnv(t)

	_, err := config.Load(writeConfig(t, "unknown_field: true\n"))
	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "unknown_field") {
		t.Errorf("error = %v, want a message naming unknown_field", err)
	}
}

//nolint:paralleltest // the test isolates the process environment.
func TestLoadRejectsMissingExplicitPath(t *testing.T) {
	isolateEnv(t)

	path := filepath.Join(t.TempDir(), "missing.yml")

	if _, err := config.Load(path); err == nil {
		t.Fatal("expected an error")
	}
}

func TestLoadRejectsInvalidEnv(t *testing.T) {
	isolateEnv(t)

	t.Setenv("SERAPH_CONCURRENCY", "many")

	if _, err := config.Load(""); err == nil {
		t.Fatal("expected an error")
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*config.Config)
	}{
		{name: "empty out dir", mutate: func(c *config.Config) { c.OutDir = " " }},
		{name: "http base url", mutate: func(c *config.Config) { c.BaseURL = "http://example.com" }},
		{name: "zero timeout", mutate: func(c *config.Config) { c.Timeout = 0 }},
		{name: "huge timeout", mutate: func(c *config.Config) { c.Timeout = config.Duration(2 * time.Minute) }},
		{name: "zero concurrency", mutate: func(c *config.Config) { c.Concurrency = 0 }},
		{name: "negative retries", mutate: func(c *config.Config) { c.Retries = -1 }},
		{name: "unknown log level", mutate: func(c *config.Config) { c.LogLevel = "trace" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.Default()
			test.mutate(cfg)

			if err := cfg.Validate(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestDurationYAML(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Timeout = config.Duration(1500 * time.Millisecond)

	if got := cfg.Timeout.String(); got != "1.5s" {
		t.Errorf("String() = %q, want 1.5s", got)
	}
}
