package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/require"
)

// makeDirReadOnly removes write permission from dir for the duration of the
// test, to force a config.Save call targeting it to fail.
func makeDirReadOnly(t *testing.T, dir string) {
	t.Helper()

	chmodErr := os.Chmod(dir, 0o500) // #nosec G302 -- dir needs the execute bit to stay traversable
	require.NoError(t, chmodErr)

	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700) // #nosec G302 -- restoring the test dir's normal mode
	})
}

func newTestModel(ctx context.Context, tb testing.TB, opts Options) (*model, error) {
	tb.Helper()

	if opts.StatePath == "" {
		opts.StatePath = filepath.Join(tb.TempDir(), "state.json")
	}

	return newModel(ctx, opts)
}

// newSingleRepoModel builds a real model backed by one real, isolated git
// repo ("testrepo") and a config.toml pointing at it, with the repo
// pre-selected — used by tests that need a real (not stubbed) backend.
func newSingleRepoModel(t *testing.T) *model {
	t.Helper()

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	err := os.WriteFile(cfgPath, []byte(
		`[repos.testrepo]
path = "`+repoDir+`"
backends = ["git"]`,
	), 0o644)
	require.NoError(t, err)

	m, err := newTestModel(t.Context(), t, Options{ConfigPath: cfgPath})
	require.NoError(t, err)

	m.selected["testrepo"] = true

	return m
}

// newAlphaBetaModel builds a real model (via newTestModel, no on-disk repos
// needed since these tests don't touch a backend) with two repos, "alpha"
// and "beta", in that order — used by table row-rendering tests.
func newAlphaBetaModel(t *testing.T) *model {
	t.Helper()

	m, err := newTestModel(t.Context(), t, Options{})
	require.NoError(t, err)

	m.cfg = config.Config{
		Repos: map[string]config.Repo{
			"alpha": {Path: t.TempDir()},
			"beta":  {Path: t.TempDir()},
		},
		Settings: config.Settings{Concurrency: 4},
	}
	m.repoOrder = []string{"alpha", "beta"}

	return m
}

// baseModel creates a model with core fields and initTable() ready for
// behavioral tests that don't need full newModel initialization.
func baseModel(repoOrder []string, selected map[string]bool) *model {
	m := &model{
		repoOrder: repoOrder,
		selected:  selected,
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
	}
	m.cursor = 0
	m.initTable()
	m.initFilterInput()

	return m
}

// readyModel creates a model with ready=true, suitable for tests that send
// tea.Msg through Update().
func readyModel(repoOrder []string, selected map[string]bool) *model {
	m := baseModel(repoOrder, selected)
	m.ready = true

	return m
}

func mouseClick(y int) tea.Msg {
	return mouseClickAt(0, y)
}

func mouseClickAt(x, y int) tea.Msg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func initGroupTestList(m *model, items []list.Item) {
	m.initGroupList()
	m.groupList.SetItems(items)
	m.groupList.Paginator.PerPage = 10
}

// selectSavedModel returns a model in select mode with a pre-set selectSaved
// snapshot, ready for tests that verify discard (ESC) vs persist (ENTER).
func selectSavedModel() *model {
	m := readyModel([]string{"a", "b"}, map[string]bool{"a": true, "b": false})
	m.mode = modeSelect
	m.selectSaved = map[string]bool{"a": true, "b": false}

	return m
}
