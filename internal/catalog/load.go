package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads every .yaml/.yml pack in dir, validates the merged catalog, and
// returns services sorted by slug. dir is relative to fsys.
func Load(fsys fs.FS, dir string) (*Catalog, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read catalog dir %s: %w", dir, err)
	}

	cat := &Catalog{}
	seen := make(map[string]string)
	errs := make([]error, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if ext := strings.ToLower(path.Ext(name)); ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, readErr := fs.ReadFile(fsys, path.Join(dir, name))
		if readErr != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", name, readErr))

			continue
		}

		var pack Pack

		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)

		if decodeErr := decoder.Decode(&pack); decodeErr != nil {
			errs = append(errs, fmt.Errorf("%s: decode: %w", name, decodeErr))

			continue
		}

		if validateErr := pack.validate(name); validateErr != nil {
			errs = append(errs, validateErr)

			continue
		}

		for _, service := range pack.Services {
			if previous, ok := seen[service.Slug]; ok {
				errs = append(errs, &FieldError{
					File:    name,
					Slug:    service.Slug,
					Field:   "slug",
					Message: fmt.Sprintf("duplicate slug, already defined in %s", previous),
				})

				continue
			}

			seen[service.Slug] = name
		}

		cat.Packs = append(cat.Packs, pack)
		cat.Services = append(cat.Services, pack.Services...)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	if len(cat.Packs) == 0 {
		return nil, fmt.Errorf("%w: no packs found in %s", ErrInvalidCatalog, dir)
	}

	sort.Slice(cat.Services, func(i, j int) bool {
		return cat.Services[i].Slug < cat.Services[j].Slug
	})

	return cat, nil
}
