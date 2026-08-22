package cmd

import (
	"bytes"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zenizh/go-capturer"
)

func TestDispatchCommands(t *testing.T) { //nolint:funlen
	tests := []struct {
		name          string
		setup         func(t *testing.T) string
		args          []string
		expectError   bool
		expectErrorIs error
	}{
		{
			name: "TestShellCmdNoArgs",
			setup: func(t *testing.T) string {
				t.Helper()

				return cfgFakeRepoPath(t)
			},
			args:        []string{"shell"},
			expectError: true,
		},
		{
			name: "TestShellCmdWithCommand",
			setup: func(t *testing.T) string {
				t.Helper()

				return setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: t.TempDir()},
				}})
			},
			args: []string{"shell", "--", "echo hello"},
		},
		{
			name: "TestGitCmdNoReposWithBackend",
			setup: func(t *testing.T) string {
				t.Helper()

				return cfgFakeRepoPath(t)
			},
			args:          []string{"git", "--", "status"},
			expectError:   true,
			expectErrorIs: errNoReposWithBackend,
		},
		{
			name: "TestGitCmdWithRepos",
			setup: func(t *testing.T) string {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return cfgPath
			},
			args:          []string{"git", "--", "status"},
			expectError:   true,
			expectErrorIs: ErrReposFailed,
		},
		{
			name: "TestGitCmdWithExplicitRepo",
			setup: func(t *testing.T) string {
				t.Helper()

				return cfgGitRepoPlusMissing(t)
			},
			args:          []string{"git", "repo1", "--", "status"},
			expectError:   true,
			expectErrorIs: ErrReposFailed,
		},
		{
			name: "TestGitCmdInteractiveMultipleRepos",
			setup: func(t *testing.T) string {
				t.Helper()

				return cfgGitRepoPlusMissing(t)
			},
			args:          []string{"git", "-i", "repo1", "repo2", "--", "log"},
			expectError:   true,
			expectErrorIs: ErrReposFailed,
		},
		{
			name: "TestGitCmdNoArgsFmt",
			setup: func(t *testing.T) string {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return cfgPath
			},
			args:          []string{"git", "--"},
			expectError:   true,
			expectErrorIs: errNoArgsFmt,
		},
		{
			name: "TestJjCmd",
			setup: func(t *testing.T) string {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return cfgPath
			},
			args:          []string{"jj", "--", "status"},
			expectError:   true,
			expectErrorIs: errNoReposWithBackend,
		},
		{
			name: "TestJjCmdInteractiveMismatch",
			setup: func(t *testing.T) string {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return cfgPath
			},
			args:          []string{"jj", "-i", "--", "diff"},
			expectError:   true,
			expectErrorIs: errNoReposWithBackend,
		},
		{
			name: "TestFetchCmd",
			setup: func(t *testing.T) string {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return cfgPath
			},
			args:          []string{"fetch"},
			expectError:   true,
			expectErrorIs: ErrReposFailed,
		},
		{
			name: "TestFetchCmdWithExplicitRepo",
			setup: func(t *testing.T) string {
				t.Helper()

				return cfgGitRepoPlusMissing(t)
			},
			args:          []string{"fetch", "repo1"},
			expectError:   true,
			expectErrorIs: ErrReposFailed,
		},
		{
			name: "TestFetchCmdInteractive",
			setup: func(t *testing.T) string {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return cfgPath
			},
			args:          []string{"fetch", "-i"},
			expectError:   true,
			expectErrorIs: ErrReposFailed,
		},
		{
			name: "TestFetchCmdInteractiveNoBackend",
			setup: func(t *testing.T) string {
				t.Helper()

				gitDir := setupFakeGitRepo(t)

				return setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: gitDir},
				}})
			},
			args:          []string{"fetch", "-i"},
			expectError:   true,
			expectErrorIs: ErrReposFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := tt.setup(t)
			err := runHRD(t, cfgPath, tt.args)

			switch {
			case tt.expectErrorIs != nil:
				require.ErrorIs(t, err, tt.expectErrorIs)
			case tt.expectError:
				require.Error(t, err)
			default:
				assert.NoError(t, err)
			}
		})
	}
}

func TestNoReposMatched(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{},
	})

	tests := []struct {
		name string
		args []string
	}{
		{name: "status", args: []string{"hrd", "--config", cfgPath, "status"}},
		{name: "diff", args: []string{"hrd", "--config", cfgPath, "diff"}},
		{name: "fetch", args: []string{"hrd", "--config", cfgPath, "fetch"}},
		{name: "shell", args: []string{"hrd", "--config", cfgPath, "shell", "--", "echo test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp()

			err := RunApp(t.Context(), app, tt.args)
			assert.ErrorIs(t, err, errNoReposMatched)
		})
	}
}

func TestStatusReadingCommands(t *testing.T) { //nolint:funlen
	type setupResult struct {
		cfgPath string
		args    []string
		assert  func(t *testing.T, stdout string)
	}

	assertRepo1AndStatusLabel := func(t *testing.T, stdout string) {
		t.Helper()
		assert.Contains(t, stdout, "repo1")
		assert.Contains(t, stdout, StatusLabel)
	}

	setupSingleRepoWithArgs := func(args []string) func(t *testing.T) setupResult {
		return func(t *testing.T) setupResult {
			t.Helper()

			return setupResult{
				cfgPath: cfgSingleGitRepo(t),
				args:    args,
				assert:  assertRepo1AndStatusLabel,
			}
		}
	}

	tests := []struct {
		name  string
		setup func(t *testing.T) setupResult
	}{
		{
			name: "TestLsCmdNoRepos",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				return setupResult{
					cfgPath: setupTestConfig(t, config.Config{Repos: map[string]config.Repo{}}),
					args:    []string{"ls"},
					assert: func(t *testing.T, stdout string) {
						t.Helper()
						assert.Empty(t, stdout)
					},
				}
			},
		},
		{
			name: "TestLsCmdWithRepos",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"ls"},
					assert:  assertContains("repo1"),
				}
			},
		},
		{
			name:  "TestLsCmdWithMessage",
			setup: setupSingleRepoWithArgs([]string{"ls", "-m"}),
		},
		{
			name:  "TestLlCmd",
			setup: setupSingleRepoWithArgs([]string{"ll"}),
		},
		{
			name: "TestLsCmdWithExplicitRepo",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				gitDir := setupFakeGitRepo(t)
				cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: gitDir},
					"repo2": {Path: "/tmp/other"},
				}})

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"ls", "repo1"},
					assert: func(t *testing.T, stdout string) {
						t.Helper()
						assert.Contains(t, stdout, "repo1")
						assert.NotContains(t, stdout, "repo2")
					},
				}
			},
		},
		{
			name: "TestLsCmdNamesOnly",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/r1"},
					"repo2": {Path: "/tmp/r2"},
				}})

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"ls", "-n"},
					assert:  assertEqualOutput("repo1\nrepo2\n"),
				}
			},
		},
		{
			name: "TestLsCmdNamesOnlyLongFlag",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/r1"},
				}})

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"ls", "--names"},
					assert:  assertEqualOutput("repo1\n"),
				}
			},
		},
		{
			name: "TestLsCmdDirsOnly",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				repo1Dir := t.TempDir()
				repo2Dir := t.TempDir()
				cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: repo1Dir},
					"repo2": {Path: repo2Dir},
				}})

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"ls", "-d"},
					assert:  assertEqualOutput(repo1Dir + "\n" + repo2Dir + "\n"),
				}
			},
		},
		{
			name: "TestLsCmdDirsOnlyLongFlag",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				repo1Dir := t.TempDir()
				cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: repo1Dir},
				}})

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"ls", "--dirs"},
					assert:  assertEqualOutput(repo1Dir + "\n"),
				}
			},
		},
		{
			name: "TestStatusCmd",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"status"},
					assert:  assertContains("repo1"),
				}
			},
		},
		{
			name: "TestDiffCmd",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"diff"},
					assert:  assertContains("repo1"),
				}
			},
		},
		{
			name: "TestFetchCmd",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"fetch"},
					assert:  assertContains("repo1"),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.setup(t)
			stdout := runAppCapture(t, result.cfgPath, result.args)
			result.assert(t, stdout)
		})
	}
}

func TestLsCmdUnknownScope(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
		"repo1": {Path: "/tmp/repo1"},
	}})

	err := runHRD(t, cfgPath, []string{"ls", "@nonexistent"})
	require.ErrorIs(t, err, errUnknownScope)

	err = runHRD(t, cfgPath, []string{"status", "@nonexistent"})
	require.ErrorIs(t, err, errUnknownScope)
}

// A typo'd repo name must error instead of silently running on all repos.
func TestSubcmdUnknownRepoName(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
		"repo1": {Path: "/tmp/repo1"},
	}})

	for _, args := range [][]string{
		{"status", "rpeo1"},
		{"fetch", "rpeo1"},
		{"ls", "rpeo1"},
	} {
		err := runHRD(t, cfgPath, args)
		require.ErrorIs(t, err, errUnknownScope, "args: %v", args)
	}
}

func assertContains(needle string) func(t *testing.T, stdout string) { //nolint:unparam
	return func(t *testing.T, stdout string) {
		t.Helper()
		assert.Contains(t, stdout, needle)
	}
}

func assertEqualOutput(expected string) func(t *testing.T, stdout string) {
	return func(t *testing.T, stdout string) {
		t.Helper()
		assert.Equal(t, expected, stdout)
	}
}

func TestDispatchWithFewerResultsThanNames(t *testing.T) {
	names := []string{"repo1", "repo2", "repo3"}
	done := make(chan struct{})

	go func() {
		err := dispatch(names, "test", func(resultCh chan<- runner.Result) {
			resultCh <- runner.Result{RepoName: "repo1", Output: "ok", ExitCode: 0}
		})
		assert.NoError(t, err)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal(
			"dispatch hung: resultCh was not closed after callback finished",
		)
	}
}

func TestShellCmdInteractive(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
		"repo1": {Path: t.TempDir()},
	}})

	out := runAppCapture(t, cfgPath, []string{"shell", "-i", "--", "echo hello"})
	assert.Contains(t, out, "hello")
}

func TestShellCmdInteractiveFailure(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
		"repo1": {Path: t.TempDir()},
	}})

	err := runHRD(t, cfgPath, []string{"shell", "-i", "--", "exit 1"})
	require.Error(t, err)
}

func TestDispatch(t *testing.T) { //nolint:funlen
	tests := []struct {
		name        string
		names       []string
		results     []runner.Result
		wantFailed  bool
		checkStderr func(t *testing.T, stderr string)
	}{
		{
			name:    "TestDispatchEmptyOutput",
			names:   []string{"repo1"},
			results: []runner.Result{makeDispatchResult("repo1", "", 0, nil)},
		},
		{
			name:       "TestDispatchWithError",
			names:      []string{"repo1"},
			results:    []runner.Result{makeDispatchResult("repo1", "", 0, assert.AnError)},
			wantFailed: true,
		},
		{
			name:    "TestDispatchWithOutput",
			names:   []string{"repo1"},
			results: []runner.Result{makeDispatchResult("repo1", "line1\nline2\n", 0, nil)},
		},
		{
			name:  "TestDispatchMultipleRepos",
			names: []string{"repo1", "repo2"},
			results: []runner.Result{
				makeDispatchResult("repo1", "ok", 0, nil),
				makeDispatchResult("repo2", "ok", 0, nil),
			},
		},
		{
			name:  "TestDispatchMixedResults",
			names: []string{"repo1", "repo2", "repo3"},
			results: []runner.Result{
				makeDispatchResult("repo1", "ok", 0, nil),
				makeDispatchResult("repo2", "", 0, assert.AnError),
				makeDispatchResult("repo3", "ok", 0, nil),
			},
			wantFailed: true,
		},
		{
			name:       "TestDispatchNonZeroExitCode",
			names:      []string{"repo1"},
			results:    []runner.Result{makeDispatchResult("repo1", "", 1, nil)},
			wantFailed: true,
		},
		{
			name:  "TestDispatchSummaryListsFailedRepos",
			names: []string{"repo1", "repo2", "repo3"},
			results: []runner.Result{
				makeDispatchResult("repo1", "ok", 0, nil),
				makeDispatchResult("repo2", "", 0, assert.AnError),
				makeDispatchResult("repo3", "", 1, nil),
			},
			wantFailed: true,
			checkStderr: func(t *testing.T, stderr string) {
				t.Helper()
				assert.Contains(t, stderr, "; failed: repo2, repo3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			if tt.checkStderr != nil {
				ui.SetLogger(log.New(&buf))
				t.Cleanup(func() { ui.SetLogger(nil) })
			}

			err := dispatch(tt.names, "test", func(resultCh chan<- runner.Result) {
				for _, result := range tt.results {
					resultCh <- result
				}
			})
			if tt.wantFailed {
				require.ErrorIs(t, err, ErrReposFailed)
			} else {
				require.NoError(t, err)
			}

			if tt.checkStderr != nil {
				tt.checkStderr(t, buf.String())
			}
		})
	}
}

// clearLineSeq mirrors the unexported constant of the same name in
// internal/ui/progress_osc.go — the ANSI "erase current line" sequence the
// live progress bar redraws with.
const clearLineSeq = "\r\x1b[2K"

func TestDispatchReportsProgressOSC(t *testing.T) {
	var buf bytes.Buffer

	t.Cleanup(func() { ui.SetProgressOutput(nil, false) })
	ui.SetProgressOutput(&buf, true)

	names := []string{"repo1", "repo2"}
	err := dispatch(names, "test", func(resultCh chan<- runner.Result) {
		resultCh <- makeDispatchResult("repo1", "ok", 0, nil)

		resultCh <- makeDispatchResult("repo2", "", 0, assert.AnError)
	})
	require.ErrorIs(t, err, ErrReposFailed)

	out := buf.String()
	assert.Contains(
		t,
		out,
		ansi.SetProgressBar(50),
		"first result should report 50%% in normal state",
	)
	assert.Contains(
		t,
		out,
		ansi.SetErrorProgressBar(100),
		"failing result should switch to error state",
	)
	assert.Contains(
		t,
		out,
		ansi.ResetProgressBar,
		"progress indicator should be cleared when dispatch returns",
	)

	// The live progress bar shares the same TTY-gated writer as the OSC
	// sequences above, so it's exercised by the same run.
	assert.Contains(t, out, clearLineSeq, "bar line should be cleared/redrawn in place")
	assert.Contains(t, out, "[1/2]", "bar should show progress after the first result")
	assert.Contains(t, out, "[2/2]", "bar should show progress after the second result")
	assert.Contains(t, out, "✓1", "should count the first, successful result")
	assert.Contains(t, out, "✗1", "should count the second, failed result")
}

func TestDispatchNoLiveBarWhenNotTTY(t *testing.T) {
	var buf bytes.Buffer

	t.Cleanup(func() { ui.SetProgressOutput(nil, false) })
	ui.SetProgressOutput(&buf, false)

	names := []string{"repo1"}
	err := dispatch(names, "test", func(resultCh chan<- runner.Result) {
		resultCh <- makeDispatchResult("repo1", "ok", 0, nil)
	})
	require.NoError(t, err)

	assert.Empty(
		t,
		buf.String(),
		"no OSC or live-bar output should be written when stdout isn't a terminal",
	)
}

func TestFilterMatchingUnknownBackend(t *testing.T) {
	result := filterMatching([]string{"repo1"}, map[string]config.Repo{"repo1": {}}, "nonexistent")
	assert.Nil(t, result)
}

func TestDispatchCommandsBadConfig(t *testing.T) {
	dir := t.TempDir()
	badPath := dir + "/"

	tests := []struct {
		name string
		args []string
	}{
		{name: "ls", args: []string{"ls"}},
		{name: "git", args: []string{"git", "--", "status"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runHRD(t, badPath, tt.args)
			assert.Error(t, err)
		})
	}
}

func TestSplitScopeArgs(t *testing.T) { //nolint:funlen // table-driven
	fixtureCfg := cfgTwoReposOneGroup()
	cfg := &fixtureCfg

	for _, tt := range []struct {
		name      string
		args      []string
		tail      []string
		wantScope []string
		wantArgs  []string
		wantErr   bool
	}{
		{
			name:      "scope before separator",
			args:      []string{"repo1"},
			tail:      []string{"status"},
			wantScope: []string{"repo1"},
			wantArgs:  []string{"status"},
		},
		{
			name:     "separator only",
			tail:     []string{"log", "--oneline"},
			wantArgs: []string{"log", "--oneline"},
		},
		{
			name:      "repo name in tail passes through",
			tail:      []string{"echo", "repo1", "hello"},
			wantArgs:  []string{"echo", "repo1", "hello"},
			wantScope: nil,
		},
		{
			name:      "group with @ prefix",
			args:      []string{"@work"},
			tail:      []string{"status"},
			wantScope: []string{"work"},
			wantArgs:  []string{"status"},
		},
		{
			name:      "mixed repo and group scope",
			args:      []string{"repo1", "@work"},
			tail:      []string{"status"},
			wantScope: []string{"repo1", "work"},
			wantArgs:  []string{"status"},
		},
		{
			name:    "unknown name before separator errors",
			args:    []string{"nope"},
			tail:    []string{"status"},
			wantErr: true,
		},
		{
			name:      "no separator: leading scope then args",
			args:      []string{"repo1", "log", "--oneline"},
			wantScope: []string{"repo1"},
			wantArgs:  []string{"log", "--oneline"},
		},
		{
			name:     "no separator: repo name after first non-scope arg is an arg",
			args:     []string{"echo", "repo1"},
			wantArgs: []string{"echo", "repo1"},
		},
		{
			name:    "no separator: unknown @-name errors",
			args:    []string{"@nope", "status"},
			wantErr: true,
		},
		{
			name:      "separator with nothing after",
			args:      []string{"repo1"},
			tail:      []string{},
			wantScope: []string{"repo1"},
			wantArgs:  []string{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fullArgs := tt.args
			dashIdx := -1

			if tt.tail != nil {
				dashIdx = len(tt.args)
				fullArgs = append(append([]string{}, tt.args...), tt.tail...)
			}

			cmd := cmdWithArgsLenAtDash(t, dashIdx)

			scope, args, err := splitScopeArgs(cmd, fullArgs, cfg)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantScope, scope)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

// cmdWithArgsLenAtDash returns a bare *cobra.Command whose
// cmd.ArgsLenAtDash() reports dashIdx, by feeding its flag parser just
// enough placeholder positionals to land the "--" terminator at that
// index (or none at all, for dashIdx < 0). splitScopeArgs only reads this
// derived state plus an explicitly-passed args slice, so the placeholder
// content itself is irrelevant to the assertions.
func cmdWithArgsLenAtDash(t *testing.T, dashIdx int) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{}

	var raw []string

	if dashIdx >= 0 {
		for range dashIdx {
			raw = append(raw, "x")
		}

		raw = append(raw, "--")
	}

	require.NoError(t, cmd.Flags().Parse(raw))

	return cmd
}

func TestShellJoin(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{name: "single arg passes verbatim", args: []string{"echo $(pwd) | wc -c"}, want: "echo $(pwd) | wc -c"},
		{name: "plain tokens", args: []string{"git", "log"}, want: "git log"},
		{name: "token with spaces is quoted", args: []string{"echo", "a  b"}, want: "echo 'a  b'"},
		{name: "single quotes survive", args: []string{"echo", "it's"}, want: `echo 'it'\''s'`},
		{name: "empty token is quoted", args: []string{"echo", ""}, want: "echo ''"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shellJoin(tt.args))
		})
	}
}

func TestGatherStatus(t *testing.T) { //nolint:funlen
	tests := []struct {
		name        string
		vcs         string
		details     bool
		result      runner.StatusResult
		checkStdout func(t *testing.T, stdout string)
	}{
		{
			name:   "TestGatherStatus_WithError",
			vcs:    "git",
			result: makeStatusError("repo1", "git", assert.AnError),
		},
		{
			name:    "TestGatherStatus_WithDetails",
			vcs:     "git",
			details: true,
			result: makeStatusResult("git", backend.RepoStatus{
				Ref:        "main",
				CommitMsg:  "initial commit",
				CommitTime: "2 days ago",
			}),
		},
		{
			name: "TestGatherStatus_Synced",
			vcs:  "git",
			result: makeStatusResult("git", backend.RepoStatus{
				Ref: "main",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateSynced},
				},
				OverallState: backend.RefStateSynced,
			}),
		},
		{
			name: "TestGatherStatus_Ahead",
			vcs:  "git",
			result: makeStatusResult("git", backend.RepoStatus{
				Ref: "main",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateAhead, Ahead: 2},
				},
				OverallState: backend.RefStateAhead,
			}),
		},
		{
			name: "TestGatherStatus_ConflictFlag",
			vcs:  "git",
			result: makeStatusResult("git", backend.RepoStatus{
				Ref: "main",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateSynced},
				},
				OverallState: backend.RefStateSynced,
				Conflict:     true,
			}),
			checkStdout: func(t *testing.T, stdout string) {
				t.Helper()
				assert.Contains(t, stdout, "‼", "repo-level conflict should display ‼ flag")
			},
		},
		{
			name: "TestGatherStatus_Dirty",
			vcs:  "git",
			result: makeStatusResult("git", backend.RepoStatus{
				Ref: "main",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateSynced},
				},
				OverallState: backend.RefStateSynced,
				Dirty:        true,
			}),
		},
		{
			name: "TestGatherStatus_Conflict",
			vcs:  "jj",
			result: makeStatusResult("jj", backend.RepoStatus{
				Ref: "abc123",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateSynced, Conflict: true},
				},
				OverallState: backend.RefStateDiverged,
				Conflict:     true,
			}),
		},
		{
			name: "TestGatherStatus_Gone",
			vcs:  "jj",
			result: makeStatusResult("jj", backend.RepoStatus{
				Ref:          "abc123",
				Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateGone}},
				OverallState: backend.RefStateGone,
			}),
		},
		{
			name: "TestGatherStatus_NoRemote",
			vcs:  "jj",
			result: makeStatusResult("jj", backend.RepoStatus{
				Ref: "abc123",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "feat", State: backend.RefStateNoRemote},
				},
				OverallState: backend.RefStateNoRemote,
			}),
		},
		{
			name: "TestGatherStatus_Unknown",
			vcs:  "jj",
			result: makeStatusResult("jj", backend.RepoStatus{
				Ref: "abc123",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateUnknown},
				},
				OverallState: backend.RefStateUnknown,
			}),
		},
		{
			name: "TestGatherStatus_Diverged",
			vcs:  "git",
			result: makeStatusResult("git", backend.RepoStatus{
				Ref: "main",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateDiverged, Ahead: 2, Behind: 1},
				},
				OverallState: backend.RefStateDiverged,
			}),
		},
		{
			name: "TestGatherStatus_Behind",
			vcs:  "git",
			result: makeStatusResult("git", backend.RepoStatus{
				Ref: "main",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateBehind, Behind: 3},
				},
				OverallState: backend.RefStateBehind,
			}),
		},
		{
			name: "TestGatherStatus_NoBookmarks",
			vcs:  "git",
			result: makeStatusResult("git", backend.RepoStatus{
				Ref:          "main",
				Bookmarks:    []backend.BookmarkStatus{},
				OverallState: backend.RefStateNoRemote,
			}),
		},
		{
			name: "TestGatherStatus_JJRef",
			vcs:  "jj",
			result: makeStatusResult("jj", backend.RepoStatus{
				Ref: "abc123def",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateSynced},
				},
				OverallState: backend.RefStateSynced,
			}),
		},
		{
			name: "TestGatherStatus_DefaultState",
			vcs:  "git",
			result: makeStatusResult("git", backend.RepoStatus{
				Ref: "main",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefState(999)},
				},
				OverallState: backend.RefState(999),
			}),
		},
		{
			name:    "TestGatherStatus_DetailsTimeOnly",
			vcs:     "git",
			details: true,
			result: makeStatusResult("git", backend.RepoStatus{
				Ref: "main",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateSynced},
				},
				OverallState: backend.RefStateSynced,
				CommitTime:   "2 days ago",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names := []string{"repo1"}
			vcsByName := map[string]string{"repo1": tt.vcs}
			run := func() {
				err := gatherStatus(
					names,
					vcsByName,
					tt.details,
					func(resultCh chan<- runner.StatusResult) {
						resultCh <- tt.result
					},
				)
				assert.NoError(t, err)
			}

			if tt.checkStdout != nil {
				stdout := capturer.CaptureStdout(run)
				tt.checkStdout(t, stdout)
			} else {
				run()
			}
		})
	}
}

func TestGatherStatusReportsProgressOSC(t *testing.T) {
	var buf bytes.Buffer

	t.Cleanup(func() { ui.SetProgressOutput(nil, false) })
	ui.SetProgressOutput(&buf, true)

	names := []string{"repo1", "repo2"}
	vcsByName := map[string]string{"repo1": "git", "repo2": "git"}

	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- makeStatusResult("git", backend.RepoStatus{Ref: "main"})

		resultCh <- makeStatusError("repo2", "git", assert.AnError)
	})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(
		t,
		out,
		ansi.SetProgressBar(50),
		"first result should report 50%% in normal state",
	)
	assert.Contains(
		t,
		out,
		ansi.SetErrorProgressBar(100),
		"errored result should switch to error state",
	)
	assert.Contains(
		t,
		out,
		ansi.ResetProgressBar,
		"progress indicator should be cleared when gatherStatus returns",
	)

	// The live progress bar shares the same TTY-gated writer as the OSC
	// sequences above, so it's exercised by the same run.
	assert.Contains(t, out, clearLineSeq, "bar line should be cleared/redrawn in place")
	assert.Contains(t, out, "[1/2]", "bar should show progress after the first result")
	assert.Contains(t, out, "[2/2]", "bar should show progress after the second result")
}

func TestGatherStatusNoLiveBarWhenNotTTY(t *testing.T) {
	var buf bytes.Buffer

	t.Cleanup(func() { ui.SetProgressOutput(nil, false) })
	ui.SetProgressOutput(&buf, false)

	names := []string{"repo1"}
	vcsByName := map[string]string{"repo1": "git"}

	err := gatherStatus(names, vcsByName, false, func(resultCh chan<- runner.StatusResult) {
		resultCh <- makeStatusResult("git", backend.RepoStatus{Ref: "main"})
	})
	require.NoError(t, err)

	assert.Empty(
		t,
		buf.String(),
		"no OSC or live-bar output should be written when stdout isn't a terminal",
	)
}
