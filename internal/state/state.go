package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SchemaVersion is the current snapshot schema version.
const SchemaVersion = 1

// ErrUnsupportedSchema reports a snapshot written by an incompatible version.
var ErrUnsupportedSchema = errors.New("unsupported snapshot schema version")

// Status is the normalized service state shown by the dashboard.
type Status string

const (
	// StatusOperational means the service reports no active incident.
	StatusOperational Status = "operational"
	// StatusDegraded means partial impact or a failing probe.
	StatusDegraded Status = "degraded"
	// StatusMajor means a declared major outage.
	StatusMajor Status = "major"
	// StatusUnknown means the last collection attempt failed.
	StatusUnknown Status = "unknown"
)

// Point is one observation in a service history series.
type Point struct {
	At     time.Time `json:"at"`
	Status Status    `json:"status"`
}

// Service is one service entry in a snapshot.
type Service struct {
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Category    string     `json:"category"`
	OfficialURL string     `json:"official_url,omitempty"`
	Regions     []string   `json:"regions"`
	Status      Status     `json:"status"`
	ObservedAt  *time.Time `json:"observed_at,omitempty"`
	LatencyMS   int64      `json:"latency_ms,omitempty"`
	Error       string     `json:"error,omitempty"`
	Points      []Point    `json:"points,omitempty"`
}

// Sources summarizes collection health for the run.
type Sources struct {
	OK     int `json:"ok"`
	Failed int `json:"failed"`
}

// Snapshot is the static JSON document served by the dashboard.
type Snapshot struct {
	SchemaVersion int       `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	Sources       Sources   `json:"sources"`
	Services      []Service `json:"services"`
}

// NewSnapshot builds a snapshot with services sorted by slug.
func NewSnapshot(now time.Time, services []Service) Snapshot {
	sorted := make([]Service, len(services))
	copy(sorted, services)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Slug < sorted[j].Slug
	})

	return Snapshot{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   now.UTC(),
		Sources:       Sources{},
		Services:      sorted,
	}
}

// WriteAtomic marshals the snapshot and replaces path in a single rename.
func WriteAtomic(path string, snap Snapshot) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create output dir %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}

	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return fmt.Errorf("create temp snapshot: %w", err)
	}

	tmpName := tmp.Name()

	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("write temp snapshot: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp snapshot: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace snapshot %s: %w", path, err)
	}

	return nil
}

// Read loads and validates a snapshot written by WriteAtomic.
func Read(path string) (Snapshot, error) {
	//nolint:gosec // the path is provided by the operator, not by user input.
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot %s: %w", path, err)
	}

	var snap Snapshot

	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot %s: %w", path, err)
	}

	if snap.SchemaVersion != SchemaVersion {
		return Snapshot{}, fmt.Errorf("%w: %d", ErrUnsupportedSchema, snap.SchemaVersion)
	}

	return snap, nil
}
