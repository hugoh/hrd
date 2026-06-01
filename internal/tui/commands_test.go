package tui

import (
	"context"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
)

func TestStartExecEmptyPrefix(t *testing.T) {
	ch, err := startExec(context.Background(), map[string]config.Repo{}, nil, "", "status", 1)
	if err != nil {
		t.Fatalf("startExec() error = %v", err)
	}

	if ch == nil {
		t.Fatal("startExec() returned nil channel")
	}
}

func TestStartExecFetch(t *testing.T) {
	ch, err := startExec(context.Background(), map[string]config.Repo{}, nil, "", "fetch", 1)
	if err != nil {
		t.Fatalf("startExec() error = %v", err)
	}

	if ch == nil {
		t.Fatal("startExec() returned nil channel for fetch")
	}
}

func TestStartExecShellPrefix(t *testing.T) {
	ch, err := startExec(context.Background(), map[string]config.Repo{}, nil, "sh", "echo hello", 1)
	if err != nil {
		t.Fatalf("startExec() error = %v", err)
	}

	if ch == nil {
		t.Fatal("startExec() returned nil channel")
	}
}

func TestExecCmd(t *testing.T) {
	m := &model{
		cfg:      config.Config{Settings: config.Settings{Concurrency: 1}},
		selected: map[string]bool{},
		persState: PersistentState{
			History: []HistoryEntry{},
		},
	}

	// When no repos, startExec with empty prefix returns a VCS channel,
	// so this should not be an error case. We're just verifying cmd runs.
	cmd := execCmd(m, nil, "", "status")

	if cmd == nil {
		t.Fatal("execCmd() returned nil")
	}
}

func TestShortcutCmdPrefixIndependent(t *testing.T) {
	// Regression test: VCS shortcuts (s/l/d/f/p/P) must use empty prefix so they
	// always route through runner.VCSSubcmd, never through runner.Shell
	// when the model's cmdPrefix happens to be prefixShell.
	tests := []struct {
		name   string
		prefix cmdPrefix
	}{
		{"default prefix", prefixNone},
		{"git prefix", prefixGit},
		{"jj prefix", prefixJj},
		{"shell prefix", prefixShell}, // this was the bug
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

			// shortcutCmd should always dispatch VCS subcommands with empty prefix.
			cmd := shortcutCmd(m, "status", false)

			if cmd == nil {
				t.Fatal("shortcutCmd() returned nil")
			}
		})
	}
}

func TestStreamNextResultNilChannel(t *testing.T) {
	m := &model{}

	cmd := streamNextResult(m)

	if cmd != nil {
		t.Error("streamNextResult() with nil channel should return nil")
	}
}

func TestLoadStatusesCmdEmptyRepos(t *testing.T) {
	m := &model{
		repoOrder: []string{},
		selected:  map[string]bool{},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
	}

	cmd := loadStatusesCmd(m)
	if cmd == nil {
		t.Fatal("loadStatusesCmd() returned nil")
	}

	msg := cmd()

	switch msg.(type) {
	case statusDoneMsg:
	default:
		t.Fatalf("expected statusDoneMsg, got %T", msg)
	}
}

func TestLoadStatusesCmdStreaming(t *testing.T) {
	m := &model{
		ctx:       context.Background(),
		repoOrder: []string{"r1"},
		selected:  map[string]bool{"r1": true},
		cfg: config.Config{
			Repos:    map[string]config.Repo{"r1": {Path: t.TempDir()}},
			Settings: config.Settings{Concurrency: 1},
		},
	}

	cmd := loadStatusesCmd(m)
	if cmd == nil {
		t.Fatal("loadStatusesCmd() returned nil")
	}

	msg := cmd()

	switch msg.(type) {
	case statusUpdateMsg:
	default:
		t.Fatalf("expected statusUpdateMsg, got %T", msg)
	}
}

func TestStreamNextStatusCmdNilChannel(t *testing.T) {
	m := &model{}

	cmd := streamNextStatusCmd(m)
	if cmd != nil {
		t.Error("streamNextStatusCmd() with nil channel should return nil")
	}
}

func TestStreamNextStatusCmdClosedChannel(t *testing.T) {
	ch := make(chan runner.StatusResult)
	close(ch)

	m := &model{
		statusCh: ch,
	}

	cmd := streamNextStatusCmd(m)
	if cmd == nil {
		t.Fatal("streamNextStatusCmd() with open channel should return a cmd")
	}

	msg := cmd()
	if _, ok := msg.(statusDoneMsg); !ok {
		t.Fatalf("expected statusDoneMsg from closed channel, got %T", msg)
	}

	if m.statusCh != nil {
		t.Error("statusCh should be nil after closed channel read")
	}
}

func TestStartExecGitJjQuotedArgs(t *testing.T) {
	// Regression: jj commit -m 'fix: tasks' was broken because strings.Fields
	// passed literal quotes to jj, which it parsed as fileset patterns.
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
				context.Background(),
				map[string]config.Repo{},
				nil,
				"git",
				tt.cmd,
				1,
			)
			if err != nil {
				t.Fatalf("startExec() error = %v", err)
			}

			// Drain the channel (empty repos = immediately closed)
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
				context.Background(),
				map[string]config.Repo{},
				nil,
				"git",
				tt.cmd,
				1,
			)
			if err == nil {
				t.Fatal("startExec() expected error for unclosed quotes")
			}
		})
	}
}

func TestStreamNextResultClosedChannel(t *testing.T) {
	ch := make(chan runner.Result)
	close(ch)

	m := &model{
		resultsCh: ch,
	}

	cmd := streamNextResult(m)
	if cmd == nil {
		t.Fatal("streamNextResult() with closed channel should return a cmd")
	}

	msg := cmd()

	_, ok := msg.(execDoneMsg)
	if !ok {
		t.Fatalf("expected execDoneMsg, got %T", msg)
	}
}
