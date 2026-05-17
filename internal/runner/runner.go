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

var errRepoNotFound = errors.New("repo not found")

// forEachRepo calls fn concurrently for each repo, limiting parallelism with a
// semaphore. When a repo name isn't found in the map, fn is called synchronously
// with an empty Repo (so it can report the not-found error via its own channel).
func forEachRepo(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	concurrency int64,
	workFn func(ctx context.Context, repo config.Repo, name string) error,
) {
	sem := semaphore.NewWeighted(concurrency)
	group, ctx := errgroup.WithContext(ctx)

	for _, name := range names {
		repo, ok := repos[name]
		if !ok {
			// Repo not found — call fn synchronously (channel is buffered, won't block).
			_ = workFn(ctx, config.Repo{}, name)

			continue
		}

		group.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("acquiring semaphore: %w", err)
			}

			defer sem.Release(1)

			return workFn(ctx, repo, name)
		})
	}

	_ = group.Wait()
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

func resultFrom(name, path, vcs string, buf bytes.Buffer, runErr error) Result {
	if ec, ok := backend.ExtractExitCode(runErr); ok {
		return Result{
			RepoName: name, RepoPath: path, VCS: vcs,
			Output: buf.String(), ExitCode: ec,
		}
	}

	if runErr != nil {
		return Result{RepoName: name, RepoPath: path, Err: runErr}
	}

	return Result{
		RepoName: name, RepoPath: path, VCS: vcs,
		Output: buf.String(), ExitCode: 0,
	}
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
	taskFn func(context.Context, config.Repo, string, chan<- T) error,
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

				return taskFn(ctx, repo, name, results)
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
	bck, err := backend.ByName(backendName)
	if err != nil {
		return nil, fmt.Errorf("checking backend %q: %w", backendName, err)
	}

	return forEachRepoChan(ctx, repos, names, concurrency,
		func(ctx context.Context, repo config.Repo, name string, results chan<- Result) error {
			res, err := bck.Run(ctx, repo.Path, args, false)
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

// VCSSubcmd runs a VCS subcommand (status, diff, log, fetch, push, pull, etc.)
// across repos, resolving backend-specific arg prefixes automatically.
// For example, "fetch" becomes ["fetch"] for git but ["git", "fetch"] for jj.
func VCSSubcmd(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	op string,
	concurrency int64,
) <-chan Result {
	return forEachRepoChan(ctx, repos, names, concurrency,
		func(ctx context.Context, repo config.Repo, name string, results chan<- Result) error {
			bck, err := backend.ByName(repo.ActiveBackend())
			if err != nil {
				results <- Result{RepoName: name, Err: fmt.Errorf("checking backend: %w", err)}

				return nil
			}

			args := bck.SubcommandArgs(op)

			res, runErr := bck.Run(ctx, repo.Path, args, false)
			results <- Result{
				RepoName: name,
				RepoPath: repo.Path,
				VCS:      bck.Name(),
				Output:   res.Output,
				ExitCode: res.ExitCode,
				Err:      runErr,
			}

			return nil
		},
		func(name string) Result { return Result{RepoName: name, Err: errRepoNotFound} },
	)
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

			//nolint:gosec // intentional: user shell commands
			cmd := exec.CommandContext(ctx, "/bin/sh", "-c", shellCmd)
			cmd.Dir = repo.Path
			cmd.Stdout = &buf
			cmd.Stderr = &buf

			results <- resultFrom(name, repo.Path, repo.ActiveBackend(), buf, cmd.Run())

			return nil
		},
		func(name string) Result { return Result{RepoName: name, Err: errRepoNotFound} },
	)
}

// ResultColor returns the display color ("red" or "green") for a result
// based on whether it errored or had a non-zero exit code.
func ResultColor(res Result) string {
	if res.Err != nil || res.ExitCode != 0 {
		return "red"
	}

	return "green"
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
			bck, err := backend.ByName(repo.ActiveBackend())
			if err != nil {
				results <- StatusResult{RepoName: name, Err: fmt.Errorf("checking backend: %w", err)}

				return nil
			}

			st, err := bck.Status(ctx, repo.Path)
			results <- StatusResult{
				RepoName: name,
				RepoPath: repo.Path,
				VCS:      bck.Name(),
				Status:   st,
				Err:      err,
			}

			return nil
		},
		func(name string) StatusResult { return StatusResult{RepoName: name, Err: errRepoNotFound} },
	)
}
