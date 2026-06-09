package tui

import (
	"context"
	"path/filepath"
	"testing"

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

// selectSavedModel returns a model in select mode with a pre-set selectSaved
// snapshot, ready for tests that verify discard (ESC) vs persist (ENTER).
func selectSavedModel() *model {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true, "b": false})
	m.ready = true
	m.mode = modeSelect
	m.selectSaved = map[string]bool{"a": true, "b": false}

	return m
}
