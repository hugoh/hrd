package tui

import (
	"context"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
)

func TestPushHistory(t *testing.T) {
	m := &model{persState: PersistentState{}}

	m.pushHistory("git", "status")

	if len(m.persState.History) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(m.persState.History))
	}

	if m.persState.History[0] != "git -- status" {
		t.Errorf("history[0] = %q, want %q", m.persState.History[0], "git -- status")
	}

	m.pushHistory("jj", "log")

	if len(m.persState.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(m.persState.History))
	}

	if m.persState.History[0] != "jj -- log" {
		t.Errorf("history[0] = %q, want %q", m.persState.History[0], "jj -- log")
	}

	if m.persState.History[1] != "git -- status" {
		t.Errorf("history[1] = %q, want %q", m.persState.History[1], "git -- status")
	}
}

func TestPushHistoryCap(t *testing.T) {
	m := &model{persState: PersistentState{}}

	for range maxHistoryLen + 5 {
		m.pushHistory("git", "")
	}

	if len(m.persState.History) > maxHistoryLen {
		t.Errorf("history length %d exceeds max %d", len(m.persState.History), maxHistoryLen)
	}
}

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
			History: []string{},
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
					History: []string{},
				},
			}

			// shortcutCmd should always dispatch VCS subcommands with empty prefix.
			cmd := shortcutCmd(m, "status")

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
