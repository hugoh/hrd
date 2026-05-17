package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
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

				return setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/repo1"},
				}})
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

				return setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: "/tmp/repo1"},
				}})
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
			args: []string{"git", "--", "status"},
		},
		{
			name: "TestGitCmdWithReposFlag",
			setup: func(t *testing.T) string {
				t.Helper()

				gitDir := setupFakeGitRepo(t)

				return setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: gitDir},
					"repo2": {Path: "/tmp/other"},
				}})
			},
			args: []string{"git", "--repos", "repo1", "--", "status"},
		},
		{
			name: "TestGitCmdInteractiveMultipleRepos",
			setup: func(t *testing.T) string {
				t.Helper()

				gitDir := setupFakeGitRepo(t)

				return setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: gitDir},
					"repo2": {Path: "/tmp/other"},
				}})
			},
			args: []string{"git", "-i", "repo1", "repo2", "--", "log"},
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
			args: []string{"fetch"},
		},
		{
			name: "TestFetchCmdWithReposFlag",
			setup: func(t *testing.T) string {
				t.Helper()

				gitDir := setupFakeGitRepo(t)

				return setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: gitDir},
					"repo2": {Path: "/tmp/other"},
				}})
			},
			args: []string{"fetch", "--repos", "repo1"},
		},
		{
			name: "TestFetchCmdInteractive",
			setup: func(t *testing.T) string {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return cfgPath
			},
			args: []string{"fetch", "-i"},
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
			args: []string{"fetch", "-i"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := tt.setup(t)
			err := runApp(t, cfgPath, tt.args)

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

			err := app.Run(context.Background(), tt.args)
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
						assert.Contains(t, stdout, "no repos tracked")
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
			name: "TestLsCmdWithMessage",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"ls", "-m"},
					assert: func(t *testing.T, stdout string) {
						t.Helper()
						assert.Contains(t, stdout, "repo1")
						assert.Contains(t, stdout, StatusLabel)
					},
				}
			},
		},
		{
			name: "TestLlCmd",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				cfgPath := cfgSingleGitRepo(t)

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"ll"},
					assert: func(t *testing.T, stdout string) {
						t.Helper()
						assert.Contains(t, stdout, "repo1")
						assert.Contains(t, stdout, StatusLabel)
					},
				}
			},
		},
		{
			name: "TestLsCmdWithReposFlag",
			setup: func(t *testing.T) setupResult {
				t.Helper()

				gitDir := setupFakeGitRepo(t)
				cfgPath := setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
					"repo1": {Path: gitDir},
					"repo2": {Path: "/tmp/other"},
				}})

				return setupResult{
					cfgPath: cfgPath,
					args:    []string{"ls", "--repos", "repo1"},
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
		t.Fatal("dispatch hung: resultCh was not closed after callback finished")
	}
}

func TestDispatch(t *testing.T) { //nolint:funlen
	tests := []struct {
		name        string
		names       []string
		results     []runner.Result
		checkStderr func(t *testing.T, stderr string)
	}{
		{
			name:    "TestDispatchEmptyOutput",
			names:   []string{"repo1"},
			results: []runner.Result{makeDispatchResult("repo1", "", 0, nil)},
		},
		{
			name:    "TestDispatchWithError",
			names:   []string{"repo1"},
			results: []runner.Result{makeDispatchResult("repo1", "", 0, assert.AnError)},
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
		},
		{
			name:    "TestDispatchNonZeroExitCode",
			names:   []string{"repo1"},
			results: []runner.Result{makeDispatchResult("repo1", "", 1, nil)},
		},
		{
			name:  "TestDispatchSummaryListsFailedRepos",
			names: []string{"repo1", "repo2", "repo3"},
			results: []runner.Result{
				makeDispatchResult("repo1", "ok", 0, nil),
				makeDispatchResult("repo2", "", 0, assert.AnError),
				makeDispatchResult("repo3", "", 1, nil),
			},
			checkStderr: func(t *testing.T, stderr string) {
				t.Helper()
				assert.Contains(t, stderr, "; failed: repo2, repo3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := func() {
				err := dispatch(tt.names, "test", func(resultCh chan<- runner.Result) {
					for _, result := range tt.results {
						resultCh <- result
					}
				})
				assert.NoError(t, err)
			}

			if tt.checkStderr != nil {
				stderr := capturer.CaptureStderr(run)
				tt.checkStderr(t, stderr)
			} else {
				run()
			}
		})
	}
}

func TestCmdArgsFilter(t *testing.T) { //nolint:funlen
	repos := map[string]config.Repo{
		"repo1": {Path: "/tmp/repo1"},
		"repo2": {Path: "/tmp/repo2"},
	}
	groups := map[string]config.Group{
		"work": {Repos: []string{"repo1"}},
	}

	t.Run("filter", func(t *testing.T) {
		for _, tt := range []struct {
			name   string
			args   []string
			repos  map[string]config.Repo
			groups map[string]config.Group
			want   []string
		}{
			{
				name:   "TestVcsArgs_FiltersRepoAndGroupNames",
				args:   []string{"repo1", "--", "status"},
				repos:  repos,
				groups: groups,
				want:   []string{"status"},
			},
			{
				name: "TestVcsArgs_HandlesDoubleDash",
				args: []string{"--", "log", "--oneline"},
				want: []string{"log", "--oneline"},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				args := cmdArgsFilter(tt.args, tt.repos, tt.groups)
				assert.Equal(t, tt.want, args)
			})
		}
	})

	t.Run("@-prefix variants", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			run  func(t *testing.T)
		}{
			{
				name: "TestVcsArgsFilterWithAtPrefix",
				run: func(t *testing.T) {
					t.Helper()

					args := cmdArgsFilter([]string{"@work", "--", "status"}, repos, groups)
					assert.Equal(t, []string{"status"}, args)
				},
			},
			{
				name: "TestVcsArgsFilterWithAtPrefixOnly",
				run: func(t *testing.T) {
					t.Helper()

					args := cmdArgsFilter([]string{"@work", "--"}, nil, groups)
					assert.Empty(t, args)
				},
			},
			{
				name: "TestResolveScopeWithAtPrefix",
				run: func(t *testing.T) {
					t.Helper()

					cfg := config.Config{Repos: repos, Groups: groups}
					names, err := cfg.ResolveScope([]string{"@work"})
					require.NoError(t, err)
					assert.Equal(t, []string{"repo1"}, names)
				},
			},
			{
				name: "TestResolveScopeWithAtPrefixNotGroupBecomesRepo",
				run: func(t *testing.T) {
					t.Helper()

					cfg := config.Config{Repos: repos, Groups: groups}
					names, err := cfg.ResolveScope([]string{"@work"})
					require.NoError(t, err)
					assert.Equal(t, []string{"repo1"}, names)
				},
			},
			{
				name: "TestVcsArgsFilterWithMixedAtAndPlain",
				run: func(t *testing.T) {
					t.Helper()

					args := cmdArgsFilter([]string{"repo1", "@work", "--", "status"}, repos, groups)
					assert.Equal(t, []string{"status"}, args)
				},
			},
		} {
			t.Run(tt.name, tt.run)
		}
	})
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
