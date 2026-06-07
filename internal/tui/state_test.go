package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	require.NoError(t, saveState(path, original))

	_, err := os.Stat(path)
	require.False(t, os.IsNotExist(err))

	loaded, err := loadState(path)
	require.NoError(t, err)

	assert.Equal(t, currentStateVersion, loaded.Version)
	assert.Len(t, loaded.History, 2)
	assert.Equal(t, "work", loaded.LastGroup)
}

func TestLoadStateMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	state, err := loadState(path)
	require.NoError(t, err)

	assert.Equal(t, currentStateVersion, state.Version)
}

func TestLoadStateCorruptFile(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "corrupt.json")
	require.NoError(t, os.WriteFile(path, []byte("not valid json"), 0o600))

	state, err := loadState(path)
	require.NoError(t, err)

	assert.Equal(t, currentStateVersion, state.Version)
}

func TestLoadStateReadError(t *testing.T) {
	dir := t.TempDir()

	state, err := loadState(dir)
	require.Error(t, err)
	assert.Equal(t, 0, state.Version)
}

func TestSaveStateCreatesDir(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "subdir", "state.json")

	state := PersistentState{Version: currentStateVersion, History: []HistoryEntry{}}
	require.NoError(t, saveState(path, state))

	_, err := os.Stat(path)
	require.False(t, os.IsNotExist(err))
}

func TestSaveStateSetsVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := PersistentState{Version: 0}
	require.NoError(t, saveState(path, state))

	loaded, _ := loadState(path)
	assert.Equal(t, currentStateVersion, loaded.Version)
}

func TestLoadStateVersionMismatchResetsHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old_version.json")

	require.NoError(t, os.WriteFile(
		path,
		[]byte(`{"version":1,"history":[{"p":"git","c":"old cmd"}]}`),
		0o600,
	))

	loaded, err := loadState(path)
	require.NoError(t, err)

	assert.Empty(t, loaded.History)
	assert.Equal(t, currentStateVersion, loaded.Version)
}

func TestSaveStateMkdirError(t *testing.T) {
	err := saveState("/nonexistent-dir-12345/state.json", PersistentState{})
	assert.Error(t, err)
}

func TestDefaultStatePath(t *testing.T) {
	path := defaultStatePath()
	require.NotEmpty(t, path)
}

func TestDefaultStatePathWithXdg(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	path := defaultStatePath()
	assert.Equal(t, "/custom/config/hrd/tui-state.json", path)
}

func TestDefaultStatePathNoXdg(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	path := defaultStatePath()
	require.NotEmpty(t, path)
	assert.Greater(t, len(path), len("/.config/hrd/tui-state.json"))
}
