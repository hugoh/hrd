package cmd

import (
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/stretchr/testify/require"
	"github.com/zenizh/go-capturer"
)

// cfgSingleGitRepo creates a temp dir with a fake git repo and returns the config path.
func cfgSingleGitRepo(t *testing.T) string {
	t.Helper()

	repoPath := setupFakeGitRepo(t)

	return setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: repoPath},
		},
	})
}

// cfgTwoReposOneGroup builds a fixture config with two repos ("repo1",
// "repo2", neither path required to exist) and a group ("work") containing
// only "repo1" — used by scope-resolution tests.
func cfgTwoReposOneGroup() config.Config {
	return config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: "/tmp/repo1"},
			"repo2": {Path: "/tmp/repo2"},
		},
		Groups: map[string]config.Group{
			"work": {Repos: []string{"repo1"}},
		},
	}
}

// cfgRepoNameTaken creates a config where "repo" is already registered
// (pointing at an unrelated path), for tests asserting a name collision.
func cfgRepoNameTaken(t *testing.T) string {
	t.Helper()

	return setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
		"repo": {Path: "/tmp/other"},
	}})
}

// cfgFakeRepoPath creates a config with a single repo ("repo1") pointing at a
// path that doesn't need to exist — for tests that fail before touching disk.
func cfgFakeRepoPath(t *testing.T) string {
	t.Helper()

	return setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
		"repo1": {Path: "/tmp/repo1"},
	}})
}

// cfgGitRepoPlusMissing creates a temp dir with a fake git repo ("repo1") plus
// a second repo ("repo2") whose configured path doesn't exist, and returns
// the config path. Used by dispatch tests that expect a partial-repo failure.
func cfgGitRepoPlusMissing(t *testing.T) string {
	t.Helper()

	gitDir := setupFakeGitRepo(t)

	return setupTestConfig(t, config.Config{Repos: map[string]config.Repo{
		"repo1": {Path: gitDir},
		"repo2": {Path: "/tmp/other"},
	}})
}

// runApp creates a new test app and runs it with the given args, applying
// the same "--" pre-split and alias registration as production (main.Run).
func runApp(t *testing.T, cfgPath string, args []string) error {
	t.Helper()

	fullArgs := append([]string{"hrd", "--config", cfgPath}, args...)

	app, head := NewAppForArgs(fullArgs)
	bufferWriters(app)

	return app.Run(t.Context(), head) //nolint:wrapcheck
}

// runAppCapture runs the app and returns stdout output.
func runAppCapture(t *testing.T, cfgPath string, args []string) string {
	t.Helper()

	fullArgs := append([]string{"hrd", "--config", cfgPath}, args...)

	app, head := NewAppForArgs(fullArgs)
	bufferWriters(app)

	return capturer.CaptureStdout(func() {
		err := app.Run(t.Context(), head)
		// Fake test repos make the underlying VCS command fail; only
		// repo-level failures are tolerated here, not usage errors.
		if err != nil {
			require.ErrorIs(t, err, ErrReposFailed)
		}
	})
}

// makeDispatchResult creates a runner.Result for table-driven dispatch tests.
func makeDispatchResult(repoName, output string, exitCode int, err error) runner.Result {
	return runner.Result{
		RepoName: repoName,
		VCS:      "git",
		Output:   output,
		ExitCode: exitCode,
		Err:      err,
	}
}

// makeStatusError creates a runner.StatusResult with an error.
func makeStatusError(repoName, vcs string, err error) runner.StatusResult {
	return runner.StatusResult{
		RepoName: repoName,
		VCS:      vcs,
		Err:      err,
	}
}

// makeStatusResult creates a runner.StatusResult for table-driven gatherStatus tests.
func makeStatusResult(vcs string, status backend.RepoStatus) runner.StatusResult {
	return runner.StatusResult{
		RepoName: "repo1",
		VCS:      vcs,
		Status:   status,
	}
}
