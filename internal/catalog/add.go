package catalog

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AddService appends a service to packPath, validates the whole catalog, and
// writes the file atomically. The file is left untouched when validation fails.
func AddService(packPath string, service Service) error {
	//nolint:gosec // the pack path is provided by the operator, not by user input.
	original, err := os.ReadFile(packPath)
	if err != nil {
		return fmt.Errorf("read catalog pack %s: %w", packPath, err)
	}

	updated, err := appendService(original, service)
	if err != nil {
		return fmt.Errorf("update catalog pack %s: %w", packPath, err)
	}

	if err := validateInDirectory(packPath, updated); err != nil {
		return err
	}

	if err := writeAtomic(packPath, updated); err != nil {
		return fmt.Errorf("write catalog pack %s: %w", packPath, err)
	}

	return nil
}

func appendService(data []byte, service Service) ([]byte, error) {
	var document yaml.Node

	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode yaml: %w", err)
	}

	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("pack root must be a mapping")
	}

	root := document.Content[0]

	var services *yaml.Node

	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value == "services" {
			services = root.Content[index+1]
			break
		}
	}

	if services == nil || services.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("pack must contain a services sequence")
	}

	serviceYAML, err := yaml.Marshal(service)
	if err != nil {
		return nil, fmt.Errorf("encode service: %w", err)
	}

	var serviceDocument yaml.Node
	if err := yaml.Unmarshal(serviceYAML, &serviceDocument); err != nil {
		return nil, fmt.Errorf("decode service: %w", err)
	}

	if len(serviceDocument.Content) == 0 {
		return nil, fmt.Errorf("service encoded as an empty document")
	}

	services.Content = append(services.Content, serviceDocument.Content[0])

	var output bytes.Buffer

	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)

	if err := encoder.Encode(&document); err != nil {
		return nil, fmt.Errorf("encode pack: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close pack encoder: %w", err)
	}

	return output.Bytes(), nil
}

func validateInDirectory(packPath string, updated []byte) error {
	dir := filepath.Dir(packPath)
	base := filepath.Base(packPath)

	tmpDir, err := os.MkdirTemp("", "seraph-catalog-*")
	if err != nil {
		return fmt.Errorf("create validation dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read catalog dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		extension := strings.ToLower(filepath.Ext(name))
		if extension != ".yaml" && extension != ".yml" {
			continue
		}

		contents := updated
		if name != base {
			//nolint:gosec // name comes from os.ReadDir and cannot contain separators.
			contents, err = os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return fmt.Errorf("read %s: %w", name, err)
			}
		}

		//nolint:gosec // name comes from os.ReadDir and cannot contain separators.
		if err := os.WriteFile(filepath.Join(tmpDir, name), contents, 0o600); err != nil {
			return fmt.Errorf("write validation file %s: %w", name, err)
		}
	}

	if _, err := Load(os.DirFS(tmpDir), "."); err != nil {
		return fmt.Errorf("catalog validation failed: %w", err)
	}

	return nil
}

func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)

	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".catalog-*.yaml")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()

	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

		return err
	}

	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()

		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, path)
}
