package catalog_test

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fabianoflorentino/seraph/internal/catalog"
)

const validPack = `
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

func loadOne(t *testing.T, name, contents string) (*catalog.Catalog, error) {
	t.Helper()

	return catalog.Load(fstest.MapFS{
		name: &fstest.MapFile{Data: []byte(contents)},
	}, ".")
}

func TestLoadValid(t *testing.T) {
	t.Parallel()

	cat, err := loadOne(t, "global.yaml", validPack)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cat.Packs) != 1 {
		t.Fatalf("packs = %d, want 1", len(cat.Packs))
	}

	if len(cat.Services) != 1 {
		t.Fatalf("services = %d, want 1", len(cat.Services))
	}

	service := cat.Services[0]
	if service.Slug != "github" {
		t.Errorf("Slug = %q, want github", service.Slug)
	}

	if service.Category != catalog.CategoryCloud {
		t.Errorf("Category = %q, want cloud", service.Category)
	}

	if got := service.RegionList(); len(got) != 1 || got[0] != catalog.GlobalRegion {
		t.Errorf("RegionList = %v, want [global]", got)
	}

	if service.Source.Type != catalog.SourceStatuspage {
		t.Errorf("Source.Type = %q, want statuspage", service.Source.Type)
	}
}

func TestLoadSortsServices(t *testing.T) {
	t.Parallel()

	contents := `
version: 1
services:
  - slug: zeta
    name: Zeta
    category: social
    regions:
      BR: primary
    source:
      type: rss
      url: https://example.com/feed.xml
  - slug: alpha
    name: Alpha
    category: telecom
    regions:
      BR: secondary
      global: primary
    source:
      type: probe
      url: https://example.com/health
`

	cat, err := loadOne(t, "br.yaml", contents)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cat.Services[0].Slug != "alpha" || cat.Services[1].Slug != "zeta" {
		t.Fatalf("services order = %s, %s; want alpha, zeta", cat.Services[0].Slug, cat.Services[1].Slug)
	}

	if len(cat.Services[0].Regions) != 2 {
		t.Errorf("regions = %v, want two entries", cat.Services[0].Regions)
	}
}

func TestLoadRejectsInvalidPack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contents string
		fragment string
	}{
		{
			name:     "unsupported version",
			contents: strings.Replace(validPack, "version: 1", "version: 2", 1),
			fragment: "version",
		},
		{
			name:     "invalid slug",
			contents: strings.Replace(validPack, "slug: github", "slug: GitHub", 1),
			fragment: "slug",
		},
		{
			name:     "empty name",
			contents: strings.Replace(validPack, "name: GitHub", "name: \"\"", 1),
			fragment: "name",
		},
		{
			name:     "invalid category",
			contents: strings.Replace(validPack, "category: cloud", "category: crypto", 1),
			fragment: "category",
		},
		{
			name:     "lowercase region",
			contents: strings.Replace(validPack, "global: primary", "br: primary", 1),
			fragment: "region",
		},
		{
			name:     "no primary region",
			contents: strings.Replace(validPack, "global: primary", "global: secondary", 1),
			fragment: "primary",
		},
		{
			name:     "invalid source type",
			contents: strings.Replace(validPack, "type: statuspage", "type: html", 1),
			fragment: "source.type",
		},
		{
			name:     "insecure source url",
			contents: strings.Replace(validPack, "https://www.githubstatus.com", "http://www.githubstatus.com", 1),
			fragment: "source.url",
		},
		{
			name:     "insecure official url",
			contents: strings.Replace(validPack, "https://github.com", "http://github.com", 1),
			fragment: "official_url",
		},
		{
			name:     "duplicate slug",
			contents: strings.Replace(validPack, "  - slug: github", "  - slug: github\n    name: GitHub\n    category: cloud\n    regions:\n      global: primary\n    source:\n      type: statuspage\n      url: https://www.githubstatus.com/api/v2/summary.json\n  - slug: github", 1),
			fragment: "duplicate",
		},
		{
			name:     "invalid yaml",
			contents: "version: [",
			fragment: "decode",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := loadOne(t, "global.yaml", test.contents)
			if err == nil {
				t.Fatal("expected an error")
			}

			if !strings.Contains(err.Error(), test.fragment) {
				t.Fatalf("error = %v, want a message containing %q", err, test.fragment)
			}
		})
	}
}

func TestLoadValidationErrorsAreSentinel(t *testing.T) {
	t.Parallel()

	_, err := loadOne(t, "global.yaml", strings.Replace(validPack, "category: cloud", "category: crypto", 1))
	if !errors.Is(err, catalog.ErrInvalidCatalog) {
		t.Fatalf("error = %v, want ErrInvalidCatalog", err)
	}
}

func TestLoadRejectsDuplicateAcrossPacks(t *testing.T) {
	t.Parallel()

	cat, err := catalog.Load(fstest.MapFS{
		"a.yaml": &fstest.MapFile{Data: []byte(validPack)},
		"b.yaml": &fstest.MapFile{Data: []byte(validPack)},
	}, ".")
	if err == nil {
		t.Fatal("expected an error")
	}

	if cat != nil {
		t.Fatalf("catalog = %v, want nil", cat)
	}

	if !strings.Contains(err.Error(), "duplicate slug") {
		t.Errorf("error = %v, want duplicate slug", err)
	}
}

func TestLoadRejectsEmptyDir(t *testing.T) {
	t.Parallel()

	_, err := catalog.Load(fstest.MapFS{"notes.txt": &fstest.MapFile{Data: []byte("none")}}, ".")
	if !errors.Is(err, catalog.ErrInvalidCatalog) {
		t.Fatalf("error = %v, want ErrInvalidCatalog", err)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	t.Parallel()

	_, err := loadOne(t, "global.yaml", validPack+"\nunknown: true\n")
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("error = %v, want unknown field error", err)
	}
}

func TestCategoriesReturnsCopy(t *testing.T) {
	t.Parallel()

	first := catalog.Categories()
	first[0] = "tampered"

	second := catalog.Categories()
	if second[0] == "tampered" {
		t.Fatal("Categories leaked its internal slice")
	}
}
