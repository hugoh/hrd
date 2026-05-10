package runner

import (
	"context"
	"os"
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if len(backend.All()) == 0 {
		backend.Register(&gitBackend{})
	}

	os.Exit(m.Run())
}

type gitBackend struct{}

func (g *gitBackend) Name() string                  { return "git" }
func (g *gitBackend) Detect(_ string) (bool, error) { return false, nil }
func (g *gitBackend) Status(_ context.Context, _ string) (backend.RepoStatus, error) {
	return backend.RepoStatus{}, nil
}

func (g *gitBackend) Run(
	_ context.Context,
	_ string,
	_ []string,
	_ bool,
) (backend.RunResult, error) {
	return backend.RunResult{}, nil
}

func TestResultFields(t *testing.T) {
	r := Result{RepoName: "test", RepoPath: "/test", VCS: "git", Output: "ok", ExitCode: 0}
	assert.Equal(t, "test", r.RepoName)
	assert.Equal(t, "/test", r.RepoPath)
}

func TestStatusResultFields(t *testing.T) {
	sr := StatusResult{RepoName: "test", RepoPath: "/test", VCS: "git"}
	assert.Equal(t, "test", sr.RepoName)
}

func TestDispatch(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1", Backends: []string{"git"}},
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
		"r1": {Path: "/tmp/r1", Backends: []string{"git"}},
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

func TestVCS(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1", Backends: []string{"git"}},
	}

	ch := VCS(context.Background(), repos, []string{"r1"}, "status", 1)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 1, count)
}

func TestVCS_RepoNotFound(t *testing.T) {
	repos := map[string]config.Repo{}

	ch := VCS(context.Background(), repos, []string{"nonexistent"}, "status", 1)
	results := collectResults(ch)
	assert.Len(t, results, 1)
	assert.Equal(t, "nonexistent", results[0].RepoName)
	assert.Error(t, results[0].Err)
}

func TestVCSArgs(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1", Backends: []string{"git"}},
	}

	ch := VCSArgs(context.Background(), repos, []string{"r1"}, "log", []string{"--oneline"}, 1)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 1, count)
}

func TestStatus(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1", Backends: []string{"git"}},
	}

	ch := Status(context.Background(), repos, []string{"r1"}, 1)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 1, count)
}

func TestDiff(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1", Backends: []string{"git"}},
	}

	ch := Diff(context.Background(), repos, []string{"r1"}, 1)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 1, count)
}

func TestShell(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: t.TempDir(), Backends: []string{"git"}},
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
		"r1": {Path: "/tmp", Backends: []string{"git"}},
	}

	ch := Shell(context.Background(), repos, []string{"r1"}, "exit 42", 1)
	results := collectResults(ch)
	assert.Len(t, results, 1)
	assert.Equal(t, 42, results[0].ExitCode)
}

func TestGatherStatus(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1", Backends: []string{"git"}},
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
		"r1": {Path: "/tmp/r1", Backends: []string{"git"}},
		"r2": {Path: "/tmp/r2", Backends: []string{"git"}},
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

func TestVCS_MultipleRepos(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp/r1", Backends: []string{"git"}},
		"r2": {Path: "/tmp/r2", Backends: []string{"git"}},
	}

	ch := VCS(context.Background(), repos, []string{"r1", "r2"}, "status", 2)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 2, count)
}

func TestShell_MultipleRepos(t *testing.T) {
	repos := map[string]config.Repo{
		"r1": {Path: "/tmp", Backends: []string{"git"}},
		"r2": {Path: "/tmp", Backends: []string{"git"}},
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
