package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfigFile(t *testing.T, path, toml string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(toml), 0o600))
}

// newAlphaModel creates a model backed by a config file with a single repo
// "alpha", returning the model and the config path so tests can rewrite it
// to simulate a concurrent external change.
func newAlphaModel(t *testing.T) (*model, string) {
	t.Helper()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"
`)

	m, err := newTestModel(t.Context(), t, Options{ConfigPath: cfgPath})
	require.NoError(t, err)

	return m, cfgPath
}

func TestReloadConfigPicksUpNewRepo(t *testing.T) {
	m, cfgPath := newAlphaModel(t)
	assert.Equal(t, []string{"alpha"}, m.repoOrder, "repoOrder before reload")

	// Simulate a concurrent CLI invocation adding a repo.
	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"

[repos.beta]
path = "`+t.TempDir()+`"
`)

	require.NoError(t, m.reloadConfig())

	assert.Equal(t, []string{"alpha", "beta"}, m.repoOrder, "repoOrder after reload")
	assert.True(t, m.selected["beta"],
		"new repo should be auto-selected when the session was in select-all state")
}

func TestReloadConfigLeavesNewRepoUnselectedWithCuratedSelection(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"

[repos.gamma]
path = "`+t.TempDir()+`"
`)

	m, err := newTestModel(t.Context(), t, Options{ConfigPath: cfgPath})
	require.NoError(t, err)
	// Curate the selection down from "all" so it's no longer select-all.
	m.selected["gamma"] = false

	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"

[repos.beta]
path = "`+t.TempDir()+`"

[repos.gamma]
path = "`+t.TempDir()+`"
`)

	require.NoError(t, m.reloadConfig())

	assert.False(t, m.selected["beta"],
		"new repo should stay unselected once the selection has been curated")
}

func TestReloadConfigPrunesSelectionForRemovedRepo(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"

[repos.beta]
path = "`+t.TempDir()+`"
`)

	m, err := newTestModel(t.Context(), t, Options{ConfigPath: cfgPath})
	require.NoError(t, err)

	m.selected["alpha"] = true
	m.selected["beta"] = true

	// Simulate a concurrent CLI invocation removing "beta".
	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"
`)

	require.NoError(t, m.reloadConfig())

	assert.True(t, m.selected["alpha"], "alpha should remain selected")
	_, stillPresent := m.selected["beta"]
	assert.False(t, stillPresent, "beta should be dropped from selection after it disappears")
}

func TestReloadConfigClearsDeadGroupFilter(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"
groups = ["work"]
`)

	m, err := newTestModel(t.Context(), t, Options{ConfigPath: cfgPath, Group: "work"})
	require.NoError(t, err)
	require.Equal(t, "work", m.groupFilter, "sanity check group filter applied at startup")

	// The "work" tag disappears entirely from disk.
	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"
`)

	require.NoError(t, m.reloadConfig())

	assert.Empty(t, m.groupFilter, "dead group filter should reset to all repos")
}

func TestReloadConfigPreservesCursorByName(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"

[repos.beta]
path = "`+t.TempDir()+`"

[repos.gamma]
path = "`+t.TempDir()+`"
`)

	m, err := newTestModel(t.Context(), t, Options{ConfigPath: cfgPath})
	require.NoError(t, err)

	m.selected = map[string]bool{"alpha": true, "beta": true, "gamma": true}
	m.mode = modeSelect
	m.updateTableRows()

	names := m.tableRepos()
	betaIdx := -1

	for i, n := range names {
		if n == "beta" {
			betaIdx = i
		}
	}

	require.GreaterOrEqual(t, betaIdx, 0, "beta should be in the table")
	m.cursor = betaIdx
	m.repoTable.SetCursor(betaIdx)

	// Rewrite disk config so a new repo sorts before "beta", shifting index.
	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"

[repos.aardvark]
path = "`+t.TempDir()+`"

[repos.beta]
path = "`+t.TempDir()+`"

[repos.gamma]
path = "`+t.TempDir()+`"
`)

	require.NoError(t, m.reloadConfig())

	newNames := m.tableRepos()
	require.Less(t, m.cursor, len(newNames))
	assert.Equal(
		t,
		"beta",
		newNames[m.cursor],
		"cursor should follow the repo it was on, not the index",
	)
}

func TestReloadConfigLeavesStateUntouchedOnError(t *testing.T) {
	m, cfgPath := newAlphaModel(t)

	writeConfigFile(t, cfgPath, `not valid toml [[[`)

	err := m.reloadConfig()
	require.Error(t, err)
	assert.Equal(t, []string{"alpha"}, m.repoOrder, "repoOrder should be untouched on reload error")
}

func TestMutateConfigDoesNotClobberConcurrentChange(t *testing.T) {
	m, cfgPath := newAlphaModel(t)

	// Simulate a concurrent CLI `hrd repo add` while the TUI is open: it
	// writes a new repo directly to disk, bypassing m.cfg entirely.
	writeConfigFile(t, cfgPath, `[repos.alpha]
path = "`+t.TempDir()+`"

[repos.beta]
path = "`+t.TempDir()+`"
`)

	err := m.mutateConfig(func(cfg *config.Config) {
		cfg.AddRepoToGroup("alpha", "work")
	})
	require.NoError(t, err)

	// Reload from disk to see what actually got persisted.
	onDisk, err := config.Load(cfgPath)
	require.NoError(t, err)

	assert.Contains(
		t,
		onDisk.Repos,
		"beta",
		"concurrent CLI addition must survive a TUI-initiated save",
	)
	assert.Contains(
		t,
		onDisk.Repos["alpha"].Groups,
		"work",
		"the TUI's own mutation must also be persisted",
	)
	assert.Contains(
		t,
		m.cfg.Repos,
		"beta",
		"m.cfg should reflect the merged state after mutateConfig",
	)
}

func TestMutateConfigSkipsVanishedRepo(t *testing.T) {
	m, cfgPath := newAlphaModel(t)

	// Concurrent CLI removal of "alpha" before the TUI's mutation lands.
	writeConfigFile(t, cfgPath, `[repos.gamma]
path = "`+t.TempDir()+`"
`)

	err := m.mutateConfig(func(cfg *config.Config) {
		if _, ok := cfg.Repos["alpha"]; !ok {
			return
		}

		cfg.AddRepoToGroup("alpha", "work")
	})
	require.NoError(t, err)

	assert.NotContains(
		t,
		m.cfg.Repos,
		"alpha",
		"vanished repo must not be resurrected by a stale mutation",
	)
}
