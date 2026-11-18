package state_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fabianoflorentino/seraph/internal/state"
)

func TestNewSnapshotSortsAndNormalizesTime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 17, 18, 30, 0, 0, time.FixedZone("BRT", -3*60*60))

	snap := state.NewSnapshot(now, []state.Service{
		{Slug: "zeta", Status: state.StatusOperational},
		{Slug: "alpha", Status: state.StatusUnknown},
	})

	if snap.SchemaVersion != state.SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", snap.SchemaVersion, state.SchemaVersion)
	}

	if !snap.GeneratedAt.Equal(now) || snap.GeneratedAt.Location() != time.UTC {
		t.Errorf("GeneratedAt = %s, want the same instant in UTC", snap.GeneratedAt)
	}

	if snap.Services[0].Slug != "alpha" || snap.Services[1].Slug != "zeta" {
		t.Fatalf("services order = %s, %s; want alpha, zeta", snap.Services[0].Slug, snap.Services[1].Slug)
	}

	if snap.Services == nil {
		t.Fatal("Services must not be nil")
	}
}

func TestMergeHistoryAppendsAndSorts(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	second := first.Add(5 * time.Minute)
	third := second.Add(5 * time.Minute)

	previous := state.Snapshot{Services: []state.Service{{
		Slug: "github",
		Points: []state.Point{
			{At: third, Status: state.StatusOperational},
			{At: first, Status: state.StatusOperational},
		},
	}}}

	services := []state.Service{{Slug: "github", Points: []state.Point{{At: second, Status: state.StatusDegraded}}}}
	merged := state.MergeHistory(previous, services, 10)

	if len(merged) != 1 {
		t.Fatalf("services = %d, want 1", len(merged))
	}

	if len(merged[0].Points) != 3 {
		t.Fatalf("points = %d, want 3", len(merged[0].Points))
	}

	for index, want := range []time.Time{first, second, third} {
		if !merged[0].Points[index].At.Equal(want) {
			t.Errorf("point %d = %s, want %s", index, merged[0].Points[index].At, want)
		}
	}
}

func TestMergeHistoryRecordsUnknownFailures(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	services := []state.Service{{
		Slug:       "github",
		Status:     state.StatusUnknown,
		ObservedAt: &now,
	}}

	merged := state.MergeHistory(state.Snapshot{}, services, 10)

	if len(merged[0].Points) != 1 {
		t.Fatalf("points = %d, want 1", len(merged[0].Points))
	}

	if merged[0].Points[0].Status != state.StatusUnknown {
		t.Errorf("status = %q, want unknown", merged[0].Points[0].Status)
	}
}

func TestMergeHistoryCapsPoints(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	previous := state.Snapshot{Services: []state.Service{{
		Slug: "github",
		Points: []state.Point{
			{At: now.Add(-2 * time.Minute), Status: state.StatusOperational},
			{At: now.Add(-time.Minute), Status: state.StatusOperational},
		},
	}}}

	services := []state.Service{{Slug: "github", Points: []state.Point{{At: now, Status: state.StatusOperational}}}}
	merged := state.MergeHistory(previous, services, 2)

	if len(merged[0].Points) != 2 {
		t.Fatalf("points = %d, want 2", len(merged[0].Points))
	}

	if !merged[0].Points[1].At.Equal(now) {
		t.Errorf("last point = %s, want %s", merged[0].Points[1].At, now)
	}
}

func TestNewSnapshotEmpty(t *testing.T) {
	t.Parallel()

	snap := state.NewSnapshot(time.Now(), nil)
	if snap.Services == nil {
		t.Fatal("Services must be an empty slice, not nil")
	}

	if len(snap.Services) != 0 {
		t.Fatalf("Services = %d, want 0", len(snap.Services))
	}
}

func TestWriteAndReadRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "state.json")
	now := time.Now().UTC()

	input := state.NewSnapshot(now, []state.Service{
		{
			Slug:      "github",
			Name:      "GitHub",
			Category:  "cloud",
			StatusURL: "https://www.githubstatus.com",
			Regions:   []string{"global"},
			Status:    state.StatusOperational,
			Points:    []state.Point{{At: now, Status: state.StatusOperational}},
		},
	})
	input.Sources = state.Sources{OK: 1}

	if err := state.WriteAtomic(path, input); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	output, err := state.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if output.SchemaVersion != input.SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", output.SchemaVersion, input.SchemaVersion)
	}

	if len(output.Services) != 1 || output.Services[0].Slug != "github" {
		t.Fatalf("services = %+v, want github", output.Services)
	}

	if output.Services[0].StatusURL != "https://www.githubstatus.com" {
		t.Errorf("StatusURL = %q, want status page", output.Services[0].StatusURL)
	}

	if output.Sources.OK != 1 {
		t.Errorf("Sources.OK = %d, want 1", output.Sources.OK)
	}
}

func TestWriteAtomicLeavesNoTempFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := state.WriteAtomic(path, state.NewSnapshot(time.Now(), nil)); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	if len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Fatalf("entries = %v, want only state.json", entries)
	}
}

func TestWriteAtomicReplacesFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state.json")

	first := state.NewSnapshot(time.Now(), []state.Service{{Slug: "alpha"}})
	if err := state.WriteAtomic(path, first); err != nil {
		t.Fatalf("WriteAtomic first: %v", err)
	}

	second := state.NewSnapshot(time.Now(), []state.Service{{Slug: "beta"}})
	if err := state.WriteAtomic(path, second); err != nil {
		t.Fatalf("WriteAtomic second: %v", err)
	}

	output, err := state.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(output.Services) != 1 || output.Services[0].Slug != "beta" {
		t.Fatalf("services = %+v, want beta", output.Services)
	}
}

func TestWriteAtomicFailsWhenDirIsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")

	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	path := filepath.Join(blocker, "state.json")

	if err := state.WriteAtomic(path, state.NewSnapshot(time.Now(), nil)); err == nil {
		t.Fatal("expected an error")
	}
}

func TestReadRejectsMissingFile(t *testing.T) {
	t.Parallel()

	if _, err := state.Read(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected an error")
	}
}

func TestReadRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := state.Read(path); err == nil {
		t.Fatal("expected an error")
	}
}

func TestReadRejectsUnsupportedSchema(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":99}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := state.Read(path)
	if !errors.Is(err, state.ErrUnsupportedSchema) {
		t.Fatalf("error = %v, want ErrUnsupportedSchema", err)
	}
}

func TestWriteAtomicFailsWhenTargetIsDir(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "state.json")
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := state.WriteAtomic(target, state.NewSnapshot(time.Now(), nil)); err == nil {
		t.Fatal("expected an error")
	}
}
