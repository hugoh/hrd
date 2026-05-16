package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSavePersState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	m := &model{
		stateFile: statePath,
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{"a": true},
		persState: PersistentState{
			History: []string{},
		},
		groupFilter: "work",
		cmdPrefix:   prefixGit,
	}

	m.savePersState()

	if m.persState.LastGroup != "work" {
		t.Errorf("LastGroup = %q, want %q", m.persState.LastGroup, "work")
	}

	if m.persState.LastPrefix != "git" {
		t.Errorf("LastPrefix = %q, want %q", m.persState.LastPrefix, "git")
	}
}

func TestExecCancelAllNoCancel(t *testing.T) {
	m := &model{}

	m.execCancelAll()

	if m.executing {
		t.Error("executing should be false after execCancelAll")
	}
}

func TestExecCancelAllWithCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	m := &model{
		execCancel: cancel,
		executing:  true,
	}

	m.execCancelAll()

	if m.executing {
		t.Error("executing should be false after execCancelAll")
	}

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
			History: []string{},
		},
	}

	m.quit()

	if m.persState.LastRepos == nil {
		t.Error("LastRepos should be set after quit")
	}

	_, err := os.Stat(m.stateFile)
	if os.IsNotExist(err) {
		t.Error("state file should exist after quit")
	}
}
