package tui

import (
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/stretchr/testify/require"
)

func TestShortcutCmdIgnoredWhileExecuting(t *testing.T) {
	m := &model{
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
		selected:  map[string]bool{"r1": true},
		repoOrder: []string{"r1"},
		executing: true,
		screen:    screenMain,
		execGen:   3,
	}

	cmd := shortcutCmd(m, "log", false)

	require.Nil(t, cmd)
	require.Equal(t, screenMain, m.screen)
	require.Equal(t, 3, m.execGen, "must not start a second run")
}

func TestExecCmdBumpsGenerationAndTagsMessages(t *testing.T) {
	m := &model{
		cfg:      config.Config{Settings: config.Settings{Concurrency: 1}},
		selected: map[string]bool{},
	}

	execCmd(m, nil, "", "status")
	first := m.execGen

	execCmd(m, nil, "", "status")

	require.Equal(t, first+1, m.execGen)
}

func TestWaitForResultTagsGeneration(t *testing.T) {
	ch := make(chan runner.Result, 1)
	ch <- runner.Result{RepoName: "r1"}

	close(ch)

	got, ok := waitForResult(ch, 7)().(execResultMsg)
	require.True(t, ok)
	require.Equal(t, 7, got.gen)

	done, ok := waitForResult(ch, 7)().(execDoneMsg)
	require.True(t, ok)
	require.Equal(t, 7, done.gen)
}

func TestStaleExecMessagesDropped(t *testing.T) {
	m := &model{execGen: 2, executing: true, execTotal: 1}

	_, cmd := m.handleExecResult(execResultMsg{
		gen:    1,
		result: execResult{name: "old"},
	})
	require.Nil(t, cmd)
	require.Empty(t, m.execResults, "stale result must not join the current run")

	_, cmd = m.handleExecDone(execDoneMsg{gen: 1})
	require.Nil(t, cmd)
	require.True(t, m.executing, "stale done must not end the current run")
}

func TestStaleStatusMessagesDropped(t *testing.T) {
	m := &model{
		statusGen: 2,
		loading:   true,
		statuses:  map[string]runner.StatusResult{},
		pending:   map[string]bool{"r1": true},
	}

	_, cmd := m.handleStatusUpdate(statusUpdateMsg{
		gen:    1,
		result: runner.StatusResult{RepoName: "old"},
	})
	require.Nil(t, cmd)
	require.Empty(t, m.statuses)

	_, cmd = m.handleStatusDone(statusDoneMsg{gen: 1})
	require.Nil(t, cmd)
	require.True(t, m.loading, "stale done must not end the current load")
	require.True(t, m.pending["r1"])
}

func TestLoadStatusesBumpsGenerationAndCancelsPrevious(t *testing.T) {
	m := &model{
		ctx:      t.Context(),
		selected: map[string]bool{"r1": true},
		cfg: config.Config{
			Repos:    map[string]config.Repo{"r1": {Path: t.TempDir()}},
			Settings: config.Settings{Concurrency: 1},
		},
	}

	loadStatusesForCmd(m, []string{"r1"})
	firstGen := m.statusGen
	firstCancelled := false
	realCancel := m.statusCancel
	m.statusCancel = func() {
		firstCancelled = true

		realCancel()
	}

	loadStatusesForCmd(m, []string{"r1"})

	require.Equal(t, firstGen+1, m.statusGen)
	require.True(t, firstCancelled, "previous status run must be cancelled")
}
