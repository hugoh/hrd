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
	backend, err := backend.ByName(backendName)
	if err != nil {
		return nil, fmt.Errorf("checking backend %q: %w", backendName, err)
	}

	results := make(chan Result, len(names))

	go func() {
		defer close(results)

		sem := semaphore.NewWeighted(concurrency)
		errGroup, ctx := errgroup.WithContext(ctx)

		for _, name := range names {
			repo, ok := repos[name] // repos is a map; caller passes config.Config.Repos
			if !ok {
				results <- Result{RepoName: name, Err: errRepoNotFound}

				continue
			}

			errGroup.Go(func() error {
				if err := sem.Acquire(ctx, 1); err != nil {
					return fmt.Errorf("acquiring semaphore: %w", err)
				}
				defer sem.Release(1)

				res, err := backend.Run(ctx, repo.Path, args, false)
				results <- Result{
					RepoName: name,
					RepoPath: repo.Path,
					VCS:      backendName,
					Output:   res.Output,
					ExitCode: res.ExitCode,
					Err:      err,
				}

				return nil // per-repo errors go through the channel, not errgroup
			})
		}

		_ = errGroup.Wait()
	}()

	return results, nil
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
	results := make(chan Result, len(names))

	go func() {
		defer close(results)

		sem := semaphore.NewWeighted(concurrency)
		errGroup, ctx := errgroup.WithContext(ctx)

		for _, name := range names {
			repo, ok := repos[name]
			if !ok {
				results <- Result{RepoName: name, Err: errRepoNotFound}

				continue
			}

			errGroup.Go(func() error {
				return runVCSCore(ctx, sem, results, repo, name, subcmd, args)
			})
		}

		_ = errGroup.Wait()
	}()

	return results
}

// runVCSCore executes a VCS subcommand for a single repo.
func runVCSCore(
	ctx context.Context,
	sem *semaphore.Weighted,
	results chan<- Result,
	repo config.Repo,
	name string,
	subcmd string,
	args []string,
) error {
	if err := sem.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("acquiring semaphore: %w", err)
	}
	defer sem.Release(1)

	bin := repo.ActiveBackend()

	cmdArgs := append([]string{subcmd}, args...)

	var buf bytes.Buffer

	//nolint:gosec // controlled command execution, args from user input
	execCmd := exec.CommandContext(
		ctx,
		bin,
		cmdArgs...,
	)
	execCmd.Dir = repo.Path
	execCmd.Stdout = &buf
	execCmd.Stderr = &buf

	exitCode := 0

	err := execCmd.Run()
	if err != nil {
		ee := &exec.ExitError{}
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			results <- Result{RepoName: name, RepoPath: repo.Path, Err: err}

			return nil
		}
	}

	results <- Result{
		RepoName: name,
		RepoPath: repo.Path,
		VCS:      bin,
		Output:   buf.String(),
		ExitCode: exitCode,
	}

	return nil
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
	results := make(chan Result, len(names))

	go func() {
		defer close(results)

		sem := semaphore.NewWeighted(concurrency)
		errGroup, ctx := errgroup.WithContext(ctx)

		for _, name := range names {
			repo, ok := repos[name]
			if !ok {
				results <- Result{RepoName: name, Err: errRepoNotFound}

				continue
			}

			errGroup.Go(func() error {
				return runShellCore(ctx, sem, results, repo, name, shellCmd)
			})
		}

		_ = errGroup.Wait()
	}()

	return results
}

// runShellCore executes a shell command for a single repo.
func runShellCore(
	ctx context.Context,
	sem *semaphore.Weighted,
	results chan<- Result,
	repo config.Repo,
	name string,
	shellCmd string,
) error {
	if err := sem.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("acquiring semaphore: %w", err)
	}
	defer sem.Release(1)

	var buf bytes.Buffer

	//nolint:gosec // shell command execution, intentionally runs user shell commands
	cmd := exec.CommandContext(
		ctx,
		"sh",
		"-c",
		shellCmd,
	)
	cmd.Dir = repo.Path
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	exitCode := 0

	err := cmd.Run()
	if err != nil {
		ee := &exec.ExitError{}
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			results <- Result{RepoName: name, RepoPath: repo.Path, Err: err}

			return nil
		}
	}

	results <- Result{
		RepoName: name,
		RepoPath: repo.Path,
		VCS:      repo.ActiveBackend(),
		Output:   buf.String(),
		ExitCode: exitCode,
	}

	return nil
}

// GatherStatus fetches the VCS status for each repo concurrently,
// streaming one StatusResult per repo to the returned channel.
func GatherStatus(
	ctx context.Context,
	repos map[string]config.Repo,
	names []string,
	concurrency int64,
) <-chan StatusResult {
	results := make(chan StatusResult, len(names))

	go func() {
		defer close(results)

		sem := semaphore.NewWeighted(concurrency)
		errGroup, ctx := errgroup.WithContext(ctx)

		for _, name := range names {
			repo, ok := repos[name]
			if !ok {
				results <- StatusResult{RepoName: name, Err: errRepoNotFound}

				continue
			}

			errGroup.Go(func() error {
				return gatherStatusCore(ctx, sem, results, repo, name)
			})
		}

		_ = errGroup.Wait()
	}()

	return results
}

// gatherStatusCore fetches VCS status for a single repo.
func gatherStatusCore(
	ctx context.Context,
	sem *semaphore.Weighted,
	results chan<- StatusResult,
	repo config.Repo,
	name string,
) error {
	if err := sem.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("acquiring semaphore: %w", err)
	}
	defer sem.Release(1)

	backend, err := backend.ByName(repo.ActiveBackend())
	if err != nil {
		results <- StatusResult{RepoName: name, Err: fmt.Errorf("checking backend: %w", err)}

		return nil
	}

	st, err := backend.Status(ctx, repo.Path)
	results <- StatusResult{
		RepoName: name,
		RepoPath: repo.Path,
		VCS:      repo.ActiveBackend(),
		Status:   st,
		Err:      err,
	}

	return nil
}
