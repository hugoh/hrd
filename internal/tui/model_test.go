package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestSavePersState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	m := &model{
		stateFile: statePath,
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{"a": true},
		cfg: config.Config{
			Groups: map[string]config.Group{"work": {Repos: []string{"a", "b"}}},
		},
		persState: PersistentState{
			History: []HistoryEntry{},
		},
		groupFilter: "work",
	}

	m.savePersState()

	assert.Equal(t, "work", m.persState.LastGroup)
}

func TestExecCancelAllNoCancel(t *testing.T) {
	m := &model{}

	m.execCancelAll()

	assert.False(t, m.executing)
}

func TestExecCancelAllWithCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	m := &model{
		execCancel: cancel,
		executing:  true,
	}

	m.execCancelAll()

	assert.False(t, m.executing)

	select {
	case <-ctx.Done():
	default:
		t.Error("context should be cancelled after execCancelAll")
	}
}

func TestQuit(t *testing.T) {
	dir := t.TempDir()

	m := &model{
		stateFile: filepath.Join(dir, "state.json"),
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{"a": true},
		persState: PersistentState{
			History: []HistoryEntry{},
		},
	}

	m.quit()

	assert.NotNil(t, m.persState.LastRepos)

	_, err := os.Stat(m.stateFile)
	assert.False(t, os.IsNotExist(err), "state file should exist after quit")
}
