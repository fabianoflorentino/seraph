package catalog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fabianoflorentino/seraph/internal/catalog"
)

func writePack(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "global.yaml")

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write pack: %v", err)
	}

	return path
}

func slackService() catalog.Service {
	return catalog.Service{
		Slug:        "slack",
		Name:        "Slack",
		Category:    catalog.CategoryMessaging,
		OfficialURL: "https://slack.com",
		StatusURL:   "https://status.slack.com",
		Regions:     map[string]catalog.Level{catalog.GlobalRegion: catalog.LevelPrimary},
		Source: catalog.Source{
			Type:   catalog.SourceJSON,
			URL:    "https://status.slack.com/api/v2.0.0/current",
			Format: catalog.JSONFormatSlack,
		},
	}
}

func TestAddService(t *testing.T) {
	t.Parallel()

	path := writePack(t, validPack)

	if err := catalog.AddService(path, slackService()); err != nil {
		t.Fatalf("AddService: %v", err)
	}

	cat, err := catalog.Load(os.DirFS(filepath.Dir(path)), ".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cat.Services) != 2 {
		t.Fatalf("services = %d, want 2", len(cat.Services))
	}

	if cat.Services[0].Slug != "github" || cat.Services[1].Slug != "slack" {
		t.Errorf("services = %q, %q; want github, slack", cat.Services[0].Slug, cat.Services[1].Slug)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(data), "slug: slack") || !strings.Contains(string(data), "slug: github") {
		t.Errorf("pack = %s, want both services", data)
	}
}

func TestAddServiceRejectsDuplicate(t *testing.T) {
	t.Parallel()

	path := writePack(t, validPack)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	service := slackService()
	service.Slug = "github"

	err = catalog.AddService(path, service)
	if err == nil {
		t.Fatal("expected duplicate error")
	}

	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %v, want duplicate", err)
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile after failure: %v", readErr)
	}

	if string(before) != string(after) {
		t.Error("pack changed after failed validation")
	}
}

func TestAddServiceRejectsInvalidService(t *testing.T) {
	t.Parallel()

	path := writePack(t, validPack)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	service := slackService()
	service.Category = "crypto"

	err = catalog.AddService(path, service)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "category") {
		t.Errorf("error = %v, want category", err)
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile after failure: %v", readErr)
	}

	if string(before) != string(after) {
		t.Error("pack changed after failed validation")
	}
}

func TestAddServiceMissingPack(t *testing.T) {
	t.Parallel()

	err := catalog.AddService(filepath.Join(t.TempDir(), "missing.yaml"), slackService())
	if err == nil {
		t.Fatal("expected an error")
	}
}
