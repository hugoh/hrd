package cmd

import (
	"context"
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

// runApp creates a new test app and runs it with the given args.
func runApp(t *testing.T, cfgPath string, args []string) error {
	t.Helper()

	app := newTestApp()

	fullArgs := append([]string{"hrd", "--config", cfgPath}, args...)

	return app.Run(context.Background(), fullArgs) //nolint:wrapcheck
}

// runAppCapture runs the app and returns stdout output.
func runAppCapture(t *testing.T, cfgPath string, args []string) string {
	t.Helper()

	app := newTestApp()

	fullArgs := append([]string{"hrd", "--config", cfgPath}, args...)

	return capturer.CaptureStdout(func() {
		err := app.Run(context.Background(), fullArgs)
		require.NoError(t, err)
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
