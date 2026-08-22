package tui

import (
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartExecEmptyPrefix(t *testing.T) {
	ch, err := startExec(t.Context(), map[string]config.Repo{}, nil, "", "status", 1)
	require.NoError(t, err)
	require.NotNil(t, ch)
}

func TestStartExecFetch(t *testing.T) {
	ch, err := startExec(t.Context(), map[string]config.Repo{}, nil, "", "fetch", 1)
	require.NoError(t, err)
	require.NotNil(t, ch)
}

func TestStartExecShellPrefix(t *testing.T) {
	ch, err := startExec(t.Context(), map[string]config.Repo{}, nil, "sh", "echo hello", 1)
	require.NoError(t, err)
	require.NotNil(t, ch)
}

func TestExecCmd(t *testing.T) {
	m := &model{
		cfg:      config.Config{Settings: config.Settings{Concurrency: 1}},
		selected: map[string]bool{},
		persState: PersistentState{
			History: []HistoryEntry{},
		},
	}

	cmd := execCmd(m, nil, "", "status")
	require.NotNil(t, cmd)
}

// TestExecCmdResetsProgressBar guards against percentShown animating
// backwards when a new exec run starts while the previous run's bar is
// still sitting near 100%: execCmd must replace the shared progressModel
// with a fresh instance (percent 0) rather than reuse the old one.
func TestExecCmdResetsProgressBar(t *testing.T) {
	progressModel.SetPercent(1)
	require.InDelta(t, 1.0, progressModel.Percent(), 0.0001)

	m := &model{
		cfg:      config.Config{Settings: config.Settings{Concurrency: 1}},
		selected: map[string]bool{},
		persState: PersistentState{
			History: []HistoryEntry{},
		},
	}

	execCmd(m, nil, "", "status")

	assert.InDelta(t, 0.0, progressModel.Percent(), 0.0001, "progress bar should reset to 0 for a new run")
}

func TestExecCmdSetsLabel(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		cmdStr    string
		wantLabel string
	}{
		{"vcs shortcut", "", "status", "status"},
		{"git command", "git", "diff HEAD", "git diff HEAD"},
		{"shell command", "sh", "ls -la", "sh ls -la"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				cfg: config.Config{Settings: config.Settings{Concurrency: 1}},
			}

			execCmd(m, nil, tt.prefix, tt.cmdStr)
			assert.Equal(t, tt.wantLabel, m.execLabel)
		})
	}
}

func TestShortcutCmdPrefixIndependent(t *testing.T) {
	tests := []struct {
		name   string
		prefix cmdPrefix
	}{
		{"default prefix", prefixNone},
		{"shell prefix", prefixShell},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
				cmdPrefix: tt.prefix,
				repoOrder: []string{"test-repo"},
				selected:  map[string]bool{"test-repo": true},
				persState: PersistentState{
					History: []HistoryEntry{},
				},
			}

			cmd := shortcutCmd(m, "status", false)
			require.NotNil(t, cmd)
		})
	}
}

func TestStreamNextResultNilChannel(t *testing.T) {
	m := &model{}

	cmd := streamNextResult(m)
	assert.Nil(t, cmd)
}

func TestLoadStatusesCmdEmptyRepos(t *testing.T) {
	m := &model{
		repoOrder: []string{},
		selected:  map[string]bool{},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
	}

	cmd := loadStatusesCmd(m)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(statusDoneMsg)
	require.True(t, ok)
}

func TestLoadStatusesCmdStreaming(t *testing.T) {
	m := &model{
		ctx:       t.Context(),
		repoOrder: []string{"r1"},
		selected:  map[string]bool{"r1": true},
		cfg: config.Config{
			Repos:    map[string]config.Repo{"r1": {Path: t.TempDir()}},
			Settings: config.Settings{Concurrency: 1},
		},
	}

	cmd := loadStatusesCmd(m)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(statusUpdateMsg)
	require.True(t, ok)
}

func TestStreamNextStatusCmdNilChannel(t *testing.T) {
	m := &model{}

	cmd := streamNextStatusCmd(m)
	assert.Nil(t, cmd)
}

func TestStreamNextStatusCmdClosedChannel(t *testing.T) {
	ch := make(chan runner.StatusResult)
	close(ch)

	m := &model{statusCh: ch}

	cmd := streamNextStatusCmd(m)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(statusDoneMsg)
	require.True(t, ok)

	// The cmd closure must not touch the model; handleStatusDone
	// (running on the Update goroutine) clears the channel.
	m.handleStatusDone()
	assert.Nil(t, m.statusCh)
}

func TestStartExecGitJjQuotedArgs(t *testing.T) {
	tests := []struct {
		name  string
		cmd   string
		args  []string
		shell bool
	}{
		{
			name:  "single-quoted message with colon",
			cmd:   `commit -m 'fix: tasks'`,
			args:  []string{"commit", "-m", "fix: tasks"},
			shell: false,
		},
		{
			name:  "double-quoted message",
			cmd:   `commit -m "fix: tasks"`,
			args:  []string{"commit", "-m", "fix: tasks"},
			shell: false,
		},
		{
			name:  "escaped space",
			cmd:   `commit -m fix:\ tasks`,
			args:  []string{"commit", "-m", "fix: tasks"},
			shell: false,
		},
		{
			name:  "multiple flags with quoting",
			cmd:   `log --oneline -5`,
			args:  []string{"log", "--oneline", "-5"},
			shell: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := startExec(
				t.Context(),
				map[string]config.Repo{},
				nil,
				"git",
				tt.cmd,
				1,
			)
			require.NoError(t, err)

			for range ch {
			}
		})
	}
}

func TestStartExecGitJjUnclosedQuotes(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{
			name: "unclosed single quote",
			cmd:  `commit -m 'fix: tasks`,
		},
		{
			name: "unclosed double quote",
			cmd:  `commit -m "fix: tasks`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := startExec(
				t.Context(),
				map[string]config.Repo{},
				nil,
				"git",
				tt.cmd,
				1,
			)
			assert.Error(t, err)
		})
	}
}

func TestStreamNextResultClosedChannel(t *testing.T) {
	ch := make(chan runner.Result)
	close(ch)

	m := &model{resultsCh: ch}

	cmd := streamNextResult(m)
	require.NotNil(t, cmd)

	msg := cmd()

	_, ok := msg.(execDoneMsg)
	require.True(t, ok)
}
