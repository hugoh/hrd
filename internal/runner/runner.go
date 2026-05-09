// Package runner executes VCS or shell commands across multiple repos in
// parallel, streaming results back through a channel as each completes.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// errRepoNotFound is returned when a repo is not found in the map.
var errRepoNotFound = errors.New("repo not found")

// forEachRepo calls fn concurrently for each repo, limiting parallelism with a
// semaphore. When a repo name isn't found in the map, fn is called synchronously
// with an empty Repo (so it can report the not-found error via its own channel).
func forEachRepo(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	concurrency int64,
	fn func(ctx context.Context, repo config.Repo, name string) error,
) {
	sem := semaphore.NewWeighted(concurrency)
	eg, ctx := errgroup.WithContext(ctx)

	for _, name := range names {
		repo, ok := repos[name]
		if !ok {
			// Repo not found — call fn synchronously (channel is buffered, won't block).
			fn(ctx, config.Repo{}, name)

			continue
		}

		name := name
		eg.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("acquiring semaphore: %w", err)
			}

			defer sem.Release(1)

			return fn(ctx, repo, name)
		})
	}

	_ = eg.Wait()
}

// Result is the outcome for a single repo, sent through the results channel.
type Result struct {
	RepoName string
	RepoPath string
	VCS      string
	Output   string
	ExitCode int
	Err      error
}

// StatusResult carries the live status for a single repo used by `ll`.
type StatusResult struct {
	RepoName string
	RepoPath string
	VCS      string
	Status   backend.RepoStatus
	Err      error
}

// forEachRepoChan runs fn across repos concurrently, streaming one T per repo.
func forEachRepoChan[T any](
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	concurrency int64,
	fn func(context.Context, config.Repo, string, chan<- T) error,
	errResult func(string) T,
) <-chan T {
	results := make(chan T, len(names))

	go func() {
		defer close(results)

		forEachRepo(ctx, repos, names, concurrency,
			func(ctx context.Context, repo config.Repo, name string) error {
				if repo.Path == "" {
					results <- errResult(name)

					return nil
				}

				return fn(ctx, repo, name, results)
			},
		)
	}()

	return results
}

// Dispatch runs args via backendName across the given repos, streaming
// one Result per repo to the returned channel. The channel is closed when
// all goroutines finish. The caller should drain the channel before
// inspecting the returned error (which reflects infrastructure failures only).
func Dispatch(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	backendName string,
	args []string,
	concurrency int64,
) (<-chan Result, error) {
	be, err := backend.ByName(backendName)
	if err != nil {
		return nil, fmt.Errorf("checking backend %q: %w", backendName, err)
	}

	return forEachRepoChan(ctx, repos, names, concurrency,
		func(ctx context.Context, repo config.Repo, name string, results chan<- Result) error {
			res, err := be.Run(ctx, repo.Path, args, false)
			results <- Result{
				RepoName: name,
				RepoPath: repo.Path,
				VCS:      backendName,
				Output:   res.Output,
				ExitCode: res.ExitCode,
				Err:      err,
			}

			return nil
		},
		func(name string) Result { return Result{RepoName: name, Err: errRepoNotFound} },
	), nil
}

// VCS runs `<vcs> <subcmd>` (e.g. `git status`, `jj diff`) for each repo,
// streaming one Result per repo to the returned channel. The VCS binary is
// determined per-repo from its active backend.
func VCS(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	subcmd string,
	concurrency int64,
) <-chan Result {
	return vcsRun(ctx, repos, names, subcmd, nil, concurrency)
}

// VCSArgs runs `<vcs> <subcmd> <args>...` for each repo.
func VCSArgs(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	subcmd string,
	args []string,
	concurrency int64,
) <-chan Result {
	return vcsRun(ctx, repos, names, subcmd, args, concurrency)
}

// vcsRun is the shared implementation for VCS and VCSArgs.
func vcsRun(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	subcmd string,
	args []string,
	concurrency int64,
) <-chan Result {
	return forEachRepoChan(ctx, repos, names, concurrency,
		func(ctx context.Context, repo config.Repo, name string, results chan<- Result) error {
			bin := repo.ActiveBackend()
			cmdArgs := append([]string{subcmd}, args...)

			var buf bytes.Buffer

			//nolint:gosec // controlled command execution, args from user input
			execCmd := exec.CommandContext(ctx, bin, cmdArgs...)
			execCmd.Dir = repo.Path
			execCmd.Stdout = &buf
			execCmd.Stderr = &buf

			err := execCmd.Run()
			if ec, ok := backend.ExtractExitCode(err); ok {
				results <- Result{
					RepoName: name, RepoPath: repo.Path, VCS: bin,
					Output: buf.String(), ExitCode: ec,
				}
			} else if err != nil {
				results <- Result{RepoName: name, RepoPath: repo.Path, Err: err}
			} else {
				results <- Result{
					RepoName: name, RepoPath: repo.Path, VCS: bin,
					Output: buf.String(), ExitCode: 0,
				}
			}

			return nil
		},
		func(name string) Result { return Result{RepoName: name, Err: errRepoNotFound} },
	)
}

// Status runs `git status` or `jj status` for each repo, streaming
// one Result per repo to the returned channel.
func Status(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	concurrency int64,
) <-chan Result {
	return VCS(ctx, repos, names, "status", concurrency)
}

// Diff runs `git diff` or `jj diff` for each repo, streaming
// one Result per repo to the returned channel.
func Diff(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	concurrency int64,
) <-chan Result {
	return VCS(ctx, repos, names, "diff", concurrency)
}

// Shell runs an arbitrary shell command across repos. It does not route
// through a backend; it uses sh -c directly.
func Shell(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	shellCmd string,
	concurrency int64,
) <-chan Result {
	return forEachRepoChan(ctx, repos, names, concurrency,
		func(ctx context.Context, repo config.Repo, name string, results chan<- Result) error {
			var buf bytes.Buffer

			//nolint:gosec // user shell commands
			cmd := exec.CommandContext(ctx, "sh", "-c", shellCmd)
			cmd.Dir = repo.Path
			cmd.Stdout = &buf
			cmd.Stderr = &buf

			err := cmd.Run()
			if ec, ok := backend.ExtractExitCode(err); ok {
				results <- Result{
					RepoName: name, RepoPath: repo.Path, VCS: repo.ActiveBackend(),
					Output: buf.String(), ExitCode: ec,
				}
			} else if err != nil {
				results <- Result{RepoName: name, RepoPath: repo.Path, Err: err}
			} else {
				results <- Result{
					RepoName: name, RepoPath: repo.Path, VCS: repo.ActiveBackend(),
					Output: buf.String(), ExitCode: 0,
				}
			}

			return nil
		},
		func(name string) Result { return Result{RepoName: name, Err: errRepoNotFound} },
	)
}

// GatherStatus fetches the VCS status for each repo concurrently,
// streaming one StatusResult per repo to the returned channel.
func GatherStatus(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	concurrency int64,
) <-chan StatusResult {
	return forEachRepoChan(
		ctx,
		repos,
		names,
		concurrency,
		func(ctx context.Context, repo config.Repo, name string, results chan<- StatusResult) error {
			be, err := backend.ByName(repo.ActiveBackend())
			if err != nil {
				results <- StatusResult{RepoName: name, Err: fmt.Errorf("checking backend: %w", err)}

				return nil
			}

			st, err := be.Status(ctx, repo.Path)
			results <- StatusResult{
				RepoName: name,
				RepoPath: repo.Path,
				VCS:      repo.ActiveBackend(),
				Status:   st,
				Err:      err,
			}

			return nil
		},
		func(name string) StatusResult { return StatusResult{RepoName: name, Err: errRepoNotFound} },
	)
}
