package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := PersistentState{
		Version: currentStateVersion,
		History: []HistoryEntry{
			{Prefix: "git", Command: "status"},
			{Prefix: "jj", Command: "log"},
		},
		LastRepos: []string{"repo1", "repo2"},
		LastGroup: "work",
	}

	if err := saveState(path, original); err != nil {
		t.Fatalf("saveState() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("saveState() did not create file")
	}

	loaded, err := loadState(path)
	if err != nil {
		t.Fatalf("loadState() error = %v", err)
	}

	if loaded.Version != currentStateVersion {
		t.Errorf("loadState().Version = %d, want %d", loaded.Version, currentStateVersion)
	}

	if len(loaded.History) != 2 {
		t.Errorf("loadState().History = %v, want 2 entries", loaded.History)
	}

	if loaded.LastGroup != "work" {
		t.Errorf("loadState().LastGroup = %q, want %q", loaded.LastGroup, "work")
	}
}

func TestLoadStateMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	state, err := loadState(path)
	if err != nil {
		t.Fatalf("loadState() for missing file should not error, got %v", err)
	}

	if state.Version != currentStateVersion {
		t.Errorf("loadState().Version = %d, want %d", state.Version, currentStateVersion)
	}
}

func TestLoadStateCorruptFile(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(path, []byte("not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	state, err := loadState(path)
	if err != nil {
		t.Fatalf("loadState() for corrupt file should not error, got %v", err)
	}

	if state.Version != currentStateVersion {
		t.Errorf("loadState().Version = %d, want %d", state.Version, currentStateVersion)
	}
}

func TestLoadStateReadError(t *testing.T) {
	// Use a directory path instead of a file path to trigger a non-NotExist error
	dir := t.TempDir()

	state, err := loadState(dir)
	if err == nil {
		t.Fatal("loadState() on directory should error")
	}
	// state should be zero value on error
	if state.Version != 0 {
		t.Errorf("loadState().Version = %d, want 0", state.Version)
	}
}

func TestSaveStateCreatesDir(t *testing.T) {
	// Directory doesn't exist yet — saveState should create it
	base := t.TempDir()
	path := filepath.Join(base, "subdir", "state.json")

	state := PersistentState{Version: currentStateVersion, History: []HistoryEntry{}}
	if err := saveState(path, state); err != nil {
		t.Fatalf("saveState() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("saveState() did not create file in new directory")
	}
}

func TestSaveStateSetsVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := PersistentState{Version: 0} // version 0 should be updated to current
	if err := saveState(path, state); err != nil {
		t.Fatalf("saveState() error = %v", err)
	}

	loaded, _ := loadState(path)
	if loaded.Version != currentStateVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, currentStateVersion)
	}
}

func TestLoadStateVersionMismatchResetsHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old_version.json")

	// Version 1 used the same struct format, so this exercises the version
	// mismatch path (not the corrupt-file path).
	if err := os.WriteFile(
		path,
		[]byte(`{"version":1,"history":[{"p":"git","c":"old cmd"}]}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadState(path)
	if err != nil {
		t.Fatalf("loadState() error = %v", err)
	}

	if len(loaded.History) != 0 {
		t.Errorf("expected history to be empty after version mismatch, got %v", loaded.History)
	}

	if loaded.Version != currentStateVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, currentStateVersion)
	}
}

func TestSaveStateMkdirError(t *testing.T) {
	// Path in a non-writable location should fail
	err := saveState("/nonexistent-dir-12345/state.json", PersistentState{})
	if err == nil {
		t.Error("saveState with bad path should error")
	}
}

func TestDefaultStatePath(t *testing.T) {
	path := defaultStatePath()
	if path == "" {
		t.Fatal("defaultStatePath() returned empty")
	}
}

func TestDefaultStatePathWithXdg(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	path := defaultStatePath()

	want := "/custom/config/hrd/tui-state.json"
	if path != want {
		t.Errorf("defaultStatePath() = %q, want %q", path, want)
	}
}

func TestDefaultStatePathNoXdg(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	path := defaultStatePath()
	if path == "" {
		t.Fatal("defaultStatePath() returned empty")
	}

	if len(path) < len("/.config/hrd/tui-state.json") {
		t.Errorf("defaultStatePath() = %q, too short", path)
	}
}
