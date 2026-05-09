// Package git implements the git VCS backend.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
)

const (
	abPartsCount = 2      // number of parts in "branch.ab +<ahead> -<behind>"
	gitBin       = "git"  // git binary name
	headRef      = "HEAD" // HEAD reference name
	logCmd       = "log"  // log command name
)

// Backend implements backend.Backend for git repositories.
type Backend struct{}

var _ backend.Backend = (*Backend)(nil)

// Name returns the backend identifier "git".
func (b *Backend) Name() string { return "git" }

// Detect returns true if path contains a .git directory.
func (b *Backend) Detect(path string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, ".git"))
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("stating .git: %w", err)
	}

	return true, nil
}

// Status queries git for the current branch/remote relationship and working
// tree cleanliness using a single `git status --porcelain=v2 --branch` call.
func (b *Backend) Status(ctx context.Context, path string) (backend.RepoStatus, error) {
	out, err := runGit(ctx, path, []string{"status", "--porcelain=v2", "--branch"})
	if err != nil {
		return backend.RepoStatus{}, fmt.Errorf("git status: %w", err)
	}

	status := parseStatus(out)

	msgOut, _ := runGit(ctx, path, []string{"show-branch", "--no-name", headRef})
	status.CommitMsg = strings.TrimSpace(msgOut)

	timeOut, _ := runGit(ctx, path, []string{logCmd, "-1", "--format=%cd", "--date=relative"})
	if t := strings.TrimSpace(timeOut); t != "" {
		status.CommitTime = "(" + t + ")"
	}

	return status, nil
}

// Run executes arbitrary git args in path.
func (b *Backend) Run(
	ctx context.Context,
	path string,
	args []string,
	interactive bool,
) (backend.RunResult, error) {
	if interactive {
		//nolint:gosec // controlled command execution, args from user input
		cmd := exec.CommandContext(
			ctx,
			gitBin,
			args...)
		cmd.Dir = path
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		code := 0

		if err != nil {
			ee := &exec.ExitError{}
			if errors.As(err, &ee) {
				code = ee.ExitCode()
				err = nil // non-zero exit is not an infrastructure error
			}
		}

		return backend.RunResult{ExitCode: code}, err
	}

	var buf bytes.Buffer

	//nolint:gosec // controlled command execution, args from user input
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = path
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	exitCode := 0

	err := cmd.Run()
	if err != nil {
		ee := &exec.ExitError{}
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			return backend.RunResult{}, fmt.Errorf("git %s: %w", args[0], err)
		}
	}

	return backend.RunResult{Output: buf.String(), ExitCode: exitCode}, nil
}

// runGit is a helper for internal status queries.
func runGit(ctx context.Context, path string, args []string) (string, error) {
	var buf bytes.Buffer

	//nolint:gosec // internal git command execution
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = path
	cmd.Stdout = &buf

	cmd.Stderr = &buf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}

	return buf.String(), nil
}

// parseStatus parses `git status --porcelain=v2 --branch` output into a
// RepoStatus. The porcelain v2 format is stable across git versions.
//
// Relevant header lines:
//
//	# branch.head <name>          current branch (or "(detached)")
//	# branch.upstream <remote/branch>  remote tracking ref
//	# branch.ab +<ahead> -<behind>     commit delta
func parseStatus(raw string) backend.RepoStatus {
	var status backend.RepoStatus

	bookmark := backend.BookmarkStatus{}

	var ahead, behind int

	hasUpstream := false

	for line := range strings.SplitSeq(raw, "\n") {
		if name, ok := strings.CutPrefix(line, "# branch.head "); ok {
			status.Ref = name
			bookmark.Name = name

			continue
		}

		if upstream, ok := strings.CutPrefix(line, "# branch.upstream "); ok {
			handleUpstream(&bookmark, upstream)

			hasUpstream = true

			continue
		}

		if ab, ok := strings.CutPrefix(line, "# branch.ab "); ok {
			ahead, behind = handleAheadBehind(ab)

			continue
		}

		if len(line) == 0 || line[0] == '#' {
			continue
		}

		// Porcelain v2 unmerged entries start with 'u'.
		if line[0] == 'u' {
			status.Conflict = true
		}

		status.Dirty = true
	}

	if hasUpstream {
		bookmark.Ahead = ahead
		bookmark.Behind = behind
	}

	if bookmark.Name != "" {
		backend.ComputeBookmarkState(&bookmark)
		status.Bookmarks = []backend.BookmarkStatus{bookmark}
	}

	status.OverallState = backend.WorstState(status.Bookmarks, status.Conflict)

	return status
}

func handleUpstream(bookmark *backend.BookmarkStatus, upstream string) {
	// upstream is "origin/main" — split on first "/"
	if before, _, ok := strings.Cut(upstream, "/"); ok {
		bookmark.Remote = before
	} else {
		bookmark.Remote = upstream
	}
}

func handleAheadBehind(ab string) (int, int) {
	// Format: +<ahead> -<behind>
	var ahead, behind int

	parts := strings.Fields(ab)
	if len(parts) == abPartsCount {
		ahead, _ = strconv.Atoi(strings.TrimPrefix(parts[0], "+"))
		behind, _ = strconv.Atoi(strings.TrimPrefix(parts[1], "-"))
	}

	return ahead, behind
}

// Register registers the git backend with the backend registry.
func Register() {
	backend.Register(&Backend{})
}
