package screenshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanerRemovesOnlyManagedPaths(t *testing.T) {
	directory := t.TempDir()
	cleaner := Cleaner{Dir: directory}
	managed := filepath.Join(directory, "window-20260727.png")
	unmanaged := filepath.Join(directory, "notes.png")
	external := filepath.Join(t.TempDir(), "region-external.png")
	for _, path := range []string{managed, unmanaged, external} {
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	removed, err := cleaner.RemoveManaged([]string{managed, managed, unmanaged, external})
	if err != nil {
		t.Fatalf("RemoveManaged returned error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(managed); !os.IsNotExist(err) {
		t.Fatalf("managed screenshot still exists: %v", err)
	}
	for _, path := range []string{unmanaged, external} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("protected path %s was removed: %v", path, err)
		}
	}
}

func TestCleanerRemovesOnlyOldUnreferencedScreenshots(t *testing.T) {
	directory := t.TempDir()
	cleaner := Cleaner{Dir: directory}
	referenced := filepath.Join(directory, "region-referenced.png")
	old := filepath.Join(directory, "window-old.png")
	recent := filepath.Join(directory, "region-recent.png")
	unmanaged := filepath.Join(directory, "other-old.png")
	for _, path := range []string{referenced, old, recent, unmanaged} {
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	for _, path := range []string{referenced, old, unmanaged} {
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatalf("age %s: %v", path, err)
		}
	}

	removed, err := cleaner.RemoveUnreferenced([]string{referenced}, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("RemoveUnreferenced returned error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old unreferenced screenshot still exists: %v", err)
	}
	for _, path := range []string{referenced, recent, unmanaged} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("protected path %s was removed: %v", path, err)
		}
	}
}

func TestCleanerRemoveAllRemovesEveryManagedScreenshot(t *testing.T) {
	directory := t.TempDir()
	cleaner := Cleaner{Dir: directory}
	for _, name := range []string{"region-one.png", "window-two.png", "keep.png"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("test"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	removed, err := cleaner.RemoveAll()
	if err != nil {
		t.Fatalf("RemoveAll returned error: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if _, err := os.Stat(filepath.Join(directory, "keep.png")); err != nil {
		t.Fatalf("unmanaged file was removed: %v", err)
	}
}
