package tui

import (
	"context"
	"path/filepath"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/config"
)

func newTestModel(tb testing.TB, ctx context.Context, opts Options) (*model, error) {
	tb.Helper()

	if opts.StatePath == "" {
		opts.StatePath = filepath.Join(tb.TempDir(), "state.json")
	}

	return newModel(ctx, opts)
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
