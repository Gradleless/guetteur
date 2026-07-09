package scheduler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneEmptyDirs_RemovesEmptyChainStopsAtRoot(t *testing.T) {
	root := t.TempDir()
	leaf := filepath.Join(root, "Show", "Season 01")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}

	pruneEmptyDirs(leaf, root)

	if _, err := os.Stat(filepath.Join(root, "Show")); !os.IsNotExist(err) {
		t.Error("expected Show/ to be pruned")
	}
	if _, err := os.Stat(root); err != nil {
		t.Errorf("root must survive: %v", err)
	}
}

func TestPruneEmptyDirs_StopsAtNonEmptyDir(t *testing.T) {
	root := t.TempDir()
	s1 := filepath.Join(root, "Show", "Season 01")
	s2 := filepath.Join(root, "Show", "Season 02")
	for _, d := range []string{s1, s2} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(s2, "ep.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pruneEmptyDirs(s1, root)

	if _, err := os.Stat(s1); !os.IsNotExist(err) {
		t.Error("expected empty Season 01/ to be pruned")
	}
	if _, err := os.Stat(filepath.Join(s2, "ep.mkv")); err != nil {
		t.Errorf("Season 02 content must survive: %v", err)
	}
}

func TestPruneEmptyDirs_NeverEscapesRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "media")
	outside := filepath.Join(parent, "elsewhere")
	for _, d := range []string{root, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// A path outside the root must be left alone entirely.
	pruneEmptyDirs(outside, root)

	if _, err := os.Stat(outside); err != nil {
		t.Errorf("dir outside root must survive: %v", err)
	}
}
