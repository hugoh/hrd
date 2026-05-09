package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellCmdNoArgs(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "shell"})
	assert.Error(t, err)
}

func TestShellCmdWithCommand(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: t.TempDir(), Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "shell", "--", "echo hello"},
	)
	assert.NoError(t, err)
}

func TestShellCmdNoReposMatched(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "shell", "--", "echo hello"},
	)
	assert.ErrorIs(t, err, errNoReposMatched)
}

func TestLsCmdNoRepos(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	stdout := captureStdout(t, func() {
		err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "ls"})
		assert.NoError(t, err)
	})
	assert.Contains(t, stdout, "no repos tracked")
}

func TestLsCmdWithRepos(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	stdout := captureStdout(t, func() {
		err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "ls"})
		assert.NoError(t, err)
	})
	assert.Contains(t, stdout, "repo1")
}

func TestLsCmdWithDetails(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	stdout := captureStdout(t, func() {
		err := app.Run(
			context.Background(),
			[]string{"hrd", "--config", cfgPath, "ls", "-l"},
		)
		assert.NoError(t, err)
	})
	assert.Contains(t, stdout, "repo1")
	assert.Contains(t, stdout, "MSG")
}

func TestLlCmd(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	stdout := captureStdout(t, func() {
		err := app.Run(
			context.Background(),
			[]string{"hrd", "--config", cfgPath, "ll"},
		)
		assert.NoError(t, err)
	})
	assert.Contains(t, stdout, "repo1")
	assert.Contains(t, stdout, "MSG")
}

func TestLsCmdWithReposFlag(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
			"repo2": {Path: "/tmp/other", Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	stdout := captureStdout(t, func() {
		err := app.Run(
			context.Background(),
			[]string{"hrd", "--config", cfgPath, "ls", "--repos", "repo1"},
		)
		assert.NoError(t, err)
	})
	assert.Contains(t, stdout, "repo1")
	assert.NotContains(t, stdout, "repo2")
}

func TestStatusCmd(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	stdout := captureStdout(t, func() {
		err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "status"})
		assert.NoError(t, err)
	})
	assert.Contains(t, stdout, "repo1")
}

func TestDiffCmd(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	stdout := captureStdout(t, func() {
		err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "diff"})
		assert.NoError(t, err)
	})
	assert.Contains(t, stdout, "repo1")
}

func TestGitCmdNoReposWithBackend(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Backends: []string{"jj"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "git", "--", "status"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoReposWithBackend)
}

func TestGitCmdWithRepos(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "git", "--", "status"},
	)
	assert.NoError(t, err)
}

func TestGitCmdWithReposFlag(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
			"repo2": {Path: "/tmp/other", Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "git", "--repos", "repo1", "--", "status"},
	)
	assert.NoError(t, err)
}

func TestGitCmdInteractiveMultipleRepos(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
			"repo2": {Path: "/tmp/other", Backends: []string{"git"}},
		},
		Settings: config.Settings{
			Concurrency:         8,
			InteractiveCommands: []string{"log"},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "git", "repo1", "repo2", "--", "log"},
	)
	assert.ErrorIs(t, err, errInteractiveSingle)
}

func TestGitCmdNoArgsFmt(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "git", "--"})
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoArgsFmt)
}

func TestGroupListWithPanel(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
			"repo2": {Path: "/tmp/repo2", Backends: []string{"git"}},
		},
		Groups: map[string]config.Group{
			"work": {Repos: []string{"repo1", "repo2"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	stdout := captureStdout(t, func() {
		err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "group", "ls"})
		assert.NoError(t, err)
	})
	assert.Contains(t, stdout, "work")
}

func TestGroupListNoGroups(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	stdout := captureStdout(t, func() {
		err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "group", "ls"})
		assert.NoError(t, err)
	})
	assert.Contains(t, stdout, "no groups defined")
}

func TestGroupAddTooFewArgs(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "group", "add", "work"},
	)
	assert.ErrorIs(t, err, errGroupAddUsage)
}

func TestGroupAddUnknownRepo(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "group", "add", "work", "unknown"},
	)
	assert.Error(t, err)
}

func TestGroupRemoveTooFewArgs(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "group", "rm"})
	assert.ErrorIs(t, err, errGroupRmUsage)
}

func TestGroupRemoveClearsContext(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
		},
		Groups: map[string]config.Group{
			"work": {Repos: []string{"repo1"}},
		},
		Context: config.Context{Current: "work"},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "group", "rm", "work"},
	)
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Empty(t, cfg.Context.Current)
}

func TestContextSetTooFewArgs(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Groups: map[string]config.Group{
			"work": {Repos: []string{}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "context", "set"})
	assert.ErrorIs(t, err, errContextSetUsage)
}

func TestContextSetUnknownGroup(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "context", "set", "unknown"},
	)
	assert.ErrorIs(t, err, errUnknownGroup)
}

func TestContextShowEmpty(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	stdout := captureStdout(t, func() {
		err := app.Run(
			context.Background(),
			[]string{"hrd", "--config", cfgPath, "context", "show"},
		)
		assert.NoError(t, err)
	})
	assert.Contains(t, stdout, "all repos")
}

func TestResolveScopeWithCommand(t *testing.T) {
	cfg := config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
		},
		Groups: map[string]config.Group{
			"work": {Repos: []string{"repo1"}},
		},
	}

	names, err := cfg.ResolveScope([]string{"repo1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"repo1"}, names)

	names, err = cfg.ResolveScope([]string{"work"})
	require.NoError(t, err)
	assert.Equal(t, []string{"repo1"}, names)
}

func TestDispatchEmptyOutput(t *testing.T) {
	names := []string{"repo1"}
	err := dispatch(names, "test", func(resultCh chan<- runner.Result) {
		resultCh <- runner.Result{RepoName: "repo1", Output: "", ExitCode: 0}
	})
	assert.NoError(t, err)
}

func TestDispatchWithError(t *testing.T) {
	names := []string{"repo1"}
	err := dispatch(names, "test", func(resultCh chan<- runner.Result) {
		resultCh <- runner.Result{RepoName: "repo1", Err: assert.AnError}
	})
	assert.NoError(t, err)
}

func TestDispatchWithOutput(t *testing.T) {
	names := []string{"repo1"}
	err := dispatch(names, "test", func(resultCh chan<- runner.Result) {
		resultCh <- runner.Result{RepoName: "repo1", Output: "line1\nline2\n", ExitCode: 0}
	})
	assert.NoError(t, err)
}

func TestDispatchMultipleRepos(t *testing.T) {
	names := []string{"repo1", "repo2"}
	err := dispatch(names, "test", func(resultCh chan<- runner.Result) {
		resultCh <- runner.Result{RepoName: "repo1", Output: "ok", ExitCode: 0}

		resultCh <- runner.Result{RepoName: "repo2", Output: "ok", ExitCode: 0}
	})
	assert.NoError(t, err)
}

func TestDispatchMixedResults(t *testing.T) {
	names := []string{"repo1", "repo2", "repo3"}
	err := dispatch(names, "test", func(resultCh chan<- runner.Result) {
		resultCh <- runner.Result{RepoName: "repo1", Output: "ok", ExitCode: 0}

		resultCh <- runner.Result{RepoName: "repo2", Err: assert.AnError}

		resultCh <- runner.Result{RepoName: "repo3", Output: "ok", ExitCode: 0}
	})
	assert.NoError(t, err)
}

func TestDispatchNonZeroExitCode(t *testing.T) {
	names := []string{"repo1"}
	err := dispatch(names, "test", func(resultCh chan<- runner.Result) {
		resultCh <- runner.Result{RepoName: "repo1", Output: "", ExitCode: 1}
	})
	assert.NoError(t, err)
}

func TestVcsArgs_FiltersRepoAndGroupNames(t *testing.T) {
	cfg := config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1", Backends: []string{"git"}},
		},
		Groups: map[string]config.Group{
			"work": {Repos: []string{"repo1"}},
		},
	}

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	args := vcsArgsForTest(cfg, []string{"repo1", "--", "status"})
	assert.Equal(t, []string{"status"}, args)
}

func TestVcsArgs_HandlesDoubleDash(t *testing.T) {
	cfg := config.Config{
		Repos: map[string]config.Repo{},
	}

	args := vcsArgsForTest(cfg, []string{"--", "log", "--oneline"})
	assert.Equal(t, []string{"log", "--oneline"}, args)
}

func vcsArgsForTest(cfg config.Config, args []string) []string {
	var out []string

	for _, arg := range args {
		if arg == "--" {
			continue
		}

		if _, ok := cfg.Repos[arg]; !ok {
			if _, ok := cfg.Groups[arg]; !ok {
				out = append(out, arg)
			}
		}
	}

	return out
}

func TestGatherStatus_WithError(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Err: assert.AnError}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_WithDetails(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}
	err := gatherStatus(names, vcsByName, true, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:        "main",
			CommitMsg:  "initial commit",
			CommitTime: "2 days ago",
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_Synced(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
			OverallState: backend.RefStateSynced,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_Ahead(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateAhead, Ahead: 2}},
			OverallState: backend.RefStateAhead,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_Dirty(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
			OverallState: backend.RefStateSynced,
			Dirty:        true,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_Conflict(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "jj"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "abc123",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced, Conflict: true}},
			OverallState: backend.RefStateDiverged,
			Conflict:     true,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_Gone(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "jj"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "abc123",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateGone}},
			OverallState: backend.RefStateGone,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_NoRemote(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "jj"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "abc123",
			Bookmarks:    []backend.BookmarkStatus{{Name: "feat", State: backend.RefStateNoRemote}},
			OverallState: backend.RefStateNoRemote,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_Unknown(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "jj"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "abc123",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateUnknown}},
			OverallState: backend.RefStateUnknown,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_Diverged(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateDiverged, Ahead: 2, Behind: 1}},
			OverallState: backend.RefStateDiverged,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_Behind(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateBehind, Behind: 3}},
			OverallState: backend.RefStateBehind,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_NoBookmarks(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{},
			OverallState: backend.RefStateNoRemote,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_JJRef(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "jj"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "abc123def",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
			OverallState: backend.RefStateSynced,
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_DefaultState(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}
	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefState(999)}},
			OverallState: backend.RefState(999),
		}}
	})
	assert.NoError(t, err)
}

func TestGatherStatus_DetailsTimeOnly(t *testing.T) {
	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}
	err := gatherStatus(names, vcsByName, true, func(resultCh chan<- runner.StatusResult) {
		resultCh <- runner.StatusResult{RepoName: "repo1", Status: backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
			OverallState: backend.RefStateSynced,
			CommitTime:   "2 days ago",
		}}
	})
	assert.NoError(t, err)
}

func TestStatusCmdNoRepos(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "status"})
	assert.ErrorIs(t, err, errNoReposMatched)
}

func TestDiffCmdNoRepos(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "diff"})
	assert.ErrorIs(t, err, errNoReposMatched)
}

func TestShellCmdNoRepos(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(
		context.Background(),
		[]string{"hrd", "--config", cfgPath, "shell", "--", "echo test"},
	)
	assert.ErrorIs(t, err, errNoReposMatched)
}

func TestJjCmd(t *testing.T) {
	gitDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: gitDir, Backends: []string{"git"}},
		},
	})

	app := NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(context.Background(), []string{"hrd", "--config", cfgPath, "jj", "--", "status"})
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoReposWithBackend)
}
