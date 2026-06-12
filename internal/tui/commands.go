package tui

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/google/shlex"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
)

var errEmptyCommand = errors.New("empty command")

// Cmd closures returned below must not touch the model: Bubble Tea runs
// them on their own goroutines, concurrently with Update and View. They
// only read from a captured channel and return messages; all model
// mutation happens in Update handlers.

func waitForStatus(ch <-chan runner.StatusResult) tea.Cmd {
	return func() tea.Msg {
		res, ok := <-ch
		if !ok {
			return statusDoneMsg{}
		}

		return statusUpdateMsg{result: res}
	}
}

func loadStatusesCmd(m *model) tea.Cmd {
	names := m.filteredRepos()
	if len(names) == 0 {
		return func() tea.Msg { return statusDoneMsg{} }
	}

	m.statusCh = runner.GatherStatus(
		m.ctx,
		m.cfg.Repos,
		names,
		m.cfg.Settings.Concurrency,
	)

	return waitForStatus(m.statusCh)
}

func streamNextStatusCmd(m *model) tea.Cmd {
	if m.statusCh == nil {
		return nil
	}

	return waitForStatus(m.statusCh)
}

func startExec(
	ctx context.Context,
	repos map[string]config.Repo,
	selected []string,
	prefix, cmdStr string,
	concurrency int,
) (<-chan runner.Result, error) {
	if prefix == "" {
		op, extra, err := splitSubcmd(cmdStr)
		if err != nil {
			return nil, err
		}

		return runner.VCSSubcmd(ctx, repos, selected, op, extra, concurrency), nil
	}

	if _, err := backend.ByName(prefix); err == nil {
		args, shErr := shlex.Split(cmdStr)
		if shErr != nil {
			return nil, fmt.Errorf("parse command: %w", shErr)
		}

		res, dispatchErr := runner.Dispatch(ctx, repos, selected, prefix, args, concurrency)
		if dispatchErr != nil {
			return nil, fmt.Errorf("dispatch: %w", dispatchErr)
		}

		return res, nil
	}

	return runner.Shell(ctx, repos, selected, cmdStr, concurrency), nil
}

// splitSubcmd tokenizes a per-repo VCS command into the subcommand and its
// extra args, so "pull --rebase" runs as the backend's pull plus flags.
func splitSubcmd(cmdStr string) (string, []string, error) {
	tokens, err := shlex.Split(cmdStr)
	if err != nil {
		return "", nil, fmt.Errorf("parse command: %w", err)
	}

	if len(tokens) == 0 {
		return "", nil, errEmptyCommand
	}

	return tokens[0], tokens[1:], nil
}

func waitForResult(ch <-chan runner.Result) tea.Cmd {
	return func() tea.Msg {
		res, ok := <-ch
		if !ok {
			return execDoneMsg{}
		}

		return execResultMsg{result: execResult{name: res.RepoName, result: res}}
	}
}

// execCmd starts execution with the given prefix and command string.
// The prefix determines the runner: "git"/"jj" → runner.Dispatch with args,
// "" → runner.VCSSubcmd (VCS subcommands), anything else → runner.Shell.
//
// VCS shortcuts (s/l/d/f) should pass "" so they always use VCS routing
// regardless of the current command-bar prefix.
func execCmd(m *model, selected []string, prefix, cmdStr string) tea.Cmd {
	m.execCancelAll()

	ctx, cancel := context.WithCancel(context.Background())
	m.execCancel = cancel
	m.execTotal = len(selected)
	m.execResults = nil

	concurrency := m.cfg.Settings.Concurrency

	resultsCh, err := startExec(ctx, m.cfg.Repos, selected, prefix, cmdStr, concurrency)
	if err != nil {
		m.executing = false
		m.resultsCh = nil

		return func() tea.Msg { return execResultMsg{err: err} }
	}

	m.executing = true
	m.resultsCh = resultsCh

	return waitForResult(resultsCh)
}

func streamNextResult(m *model) tea.Cmd {
	if m.resultsCh == nil {
		return nil
	}

	return waitForResult(m.resultsCh)
}
