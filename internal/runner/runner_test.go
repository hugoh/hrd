package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testEmail  = "test@example.com"
	testName   = "Test"
	initMain   = "-b"
	branchMain = "main"
)

func TestMain(m *testing.M) {
	if err := backend.Register(&gitBackend{}); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

type gitBackend struct{}

func (*gitBackend) Name() string  { return "git" }
func (*gitBackend) Priority() int { return 10 }
func (*gitBackend) Detect(path string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, ".git"))

	return err == nil, nil
}

func (*gitBackend) Status(_ context.Context, _ string) (backend.RepoStatus, error) {
	return backend.RepoStatus{}, nil
}
func (*gitBackend) SubcommandArgs(op string) []string               { return []string{op} }
func (*gitBackend) Subcommands(_ context.Context) ([]string, error) { return nil, nil }
func (*gitBackend) Run(
	ctx context.Context,
	path string,
	args []string,
	interactive bool,
) (backend.RunResult, error) {
	res, err := backend.RunCommand(ctx, "git", path, args, interactive)
	if err != nil {
		return backend.RunResult{}, fmt.Errorf("git %s: %w", args[0], err)
	}

	return res, nil
}

func TestDispatch(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1"},
	}

	ch, err := Dispatch(context.Background(), repos, []string{"r1"}, "git", []string{"status"}, 1)
	require.NoError(t, err)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 1, count)
}

func TestDispatch_UnknownBackend(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1"},
	}

	_, err := Dispatch(
		context.Background(),
		repos,
		[]string{"r1"},
		"unknown",
		[]string{"status"},
		1,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown backend")
}

func TestDispatch_RepoNotFound(t *testing.T) {
	repos := map[string]config.Repo{}

	ch, err := Dispatch(
		context.Background(),
		repos,
		[]string{"nonexistent"},
		"git",
		[]string{"status"},
		1,
	)
	require.NoError(t, err)

	results := collectResults(ch)
	assert.Len(t, results, 1)
	assert.Equal(t, "nonexistent", results[0].RepoName)
	assert.Error(t, results[0].Err)
}

func TestVCSSubcmd(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1"},
	}

	ch := VCSSubcmd(context.Background(), repos, []string{"r1"}, "status", 1)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 1, count)
}

func TestVCSSubcmd_RepoNotFound(t *testing.T) {
	repos := map[string]config.Repo{}

	ch := VCSSubcmd(context.Background(), repos, []string{"nonexistent"}, "status", 1)
	results := collectResults(ch)
	assert.Len(t, results, 1)
	assert.Equal(t, "nonexistent", results[0].RepoName)
	assert.Error(t, results[0].Err)
}

func TestVCSSubcmd_RunsInRepoDir(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", initMain, branchMain)
	runGit(t, dir, "config", "user.email", testEmail)
	runGit(t, dir, "config", "user.name", testName)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644))
	runGit(t, dir, "add", "test.txt")
	runGit(t, dir, "commit", "-m", "initial")

	repos := map[string]config.Repo{
		"r1": {Path: dir},
	}

	ch := VCSSubcmd(
		context.Background(),
		repos,
		[]string{"r1"},
		"status",
		1,
	)
	results := collectResults(ch)
	require.Len(t, results, 1)

	require.NoError(t, results[0].Err)
	assert.Contains(t, results[0].Output, branchMain)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	require.NoError(t, cmd.Run())
}

func TestVCSSubcmd_Ops(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1"},
	}

	for _, op := range []string{"diff", "fetch"} {
		t.Run(op, func(t *testing.T) {
			ch := VCSSubcmd(context.Background(), repos, []string{"r1"}, op, 1)

			count := 0
			for range ch {
				count++
			}

			assert.Equal(t, 1, count)
		})
	}
}

func TestVCSSubcmd_FetchRepoNotFound(t *testing.T) {
	repos := map[string]config.Repo{}

	ch := VCSSubcmd(context.Background(), repos, []string{"nonexistent"}, "fetch", 1)
	results := collectResults(ch)
	assert.Len(t, results, 1)
	assert.Equal(t, "nonexistent", results[0].RepoName)
	assert.Error(t, results[0].Err)
}

func TestVCSSubcmd_FetchRunsInRepoDir(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", initMain, branchMain)
	runGit(t, dir, "config", "user.email", testEmail)
	runGit(t, dir, "config", "user.name", testName)

	repos := map[string]config.Repo{
		"r1": {Path: dir},
	}

	ch := VCSSubcmd(context.Background(), repos, []string{"r1"}, "fetch", 1)
	results := collectResults(ch)
	require.Len(t, results, 1)
	require.NoError(t, results[0].Err)
	assert.Equal(t, "r1", results[0].RepoName)
}

func TestVCSSubcmd_MultipleRepos(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1"},
		"r2": {Path: "/tmp/r2"},
	}

	ch := VCSSubcmd(context.Background(), repos, []string{"r1", "r2"}, "status", 2)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 2, count)
}

func TestShell(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: t.TempDir()},
	}

	ch := Shell(context.Background(), repos, []string{"r1"}, "echo hello", 1)
	results := collectResults(ch)
	assert.Len(t, results, 1)
	assert.Equal(t, "r1", results[0].RepoName)
	assert.Contains(t, results[0].Output, "hello")
}

func TestShell_RepoNotFound(t *testing.T) {
	repos := map[string]config.Repo{}

	ch := Shell(context.Background(), repos, []string{"nonexistent"}, "echo hello", 1)
	results := collectResults(ch)
	assert.Len(t, results, 1)
	assert.Equal(t, "nonexistent", results[0].RepoName)
	assert.Error(t, results[0].Err)
}

func TestShell_WithExitCode(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp"},
	}

	ch := Shell(context.Background(), repos, []string{"r1"}, "exit 42", 1)
	results := collectResults(ch)
	assert.Len(t, results, 1)
	assert.Equal(t, 42, results[0].ExitCode)
}

func TestGatherStatus(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1"},
	}

	ch := GatherStatus(context.Background(), repos, []string{"r1"}, 1)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 1, count)
}

func TestGatherStatus_RepoNotFound(t *testing.T) {
	repos := map[string]config.Repo{}

	ch := GatherStatus(context.Background(), repos, []string{"nonexistent"}, 1)
	results := collectStatusResults(ch)
	assert.Len(t, results, 1)
	assert.Equal(t, "nonexistent", results[0].RepoName)
	assert.Error(t, results[0].Err)
}

func TestErrRepoNotFound(t *testing.T) {
	err := errRepoNotFound
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo not found")
}

func TestDispatch_MultipleRepos(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1"},
		"r2": {Path: "/tmp/r2"},
	}

	ch, err := Dispatch(
		context.Background(),
		repos,
		[]string{"r1", "r2"},
		"git",
		[]string{"status"},
		2,
	)
	require.NoError(t, err)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 2, count)
}

func TestShell_MultipleRepos(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp"},
		"r2": {Path: "/tmp"},
	}

	ch := Shell(context.Background(), repos, []string{"r1", "r2"}, "echo test", 2)
	results := collectResults(ch)
	assert.Len(t, results, 2)
}

func collectResults(ch <-chan Result) []Result {
	var results []Result
	for r := range ch {
		results = append(results, r)
	}

	return results
}

func collectStatusResults(ch <-chan StatusResult) []StatusResult {
	var results []StatusResult
	for r := range ch {
		results = append(results, r)
	}

	return results
}
