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

	st := parseStatus(out)

	msgOut, _ := runGit(ctx, path, []string{"show-branch", "--no-name", "HEAD"})
	st.CommitMsg = strings.TrimSpace(msgOut)

	timeOut, _ := runGit(ctx, path, []string{"log", "-1", "--format=%cd", "--date=relative"})
	if t := strings.TrimSpace(timeOut); t != "" {
		st.CommitTime = "(" + t + ")"
	}

	return st, nil
}

// Run executes arbitrary git args in path.
func (b *Backend) Run(
	ctx context.Context,
	path string,
	args []string,
	interactive bool,
) (backend.RunResult, error) {
	if interactive {
		cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // git binary from trusted config
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
	var st backend.RepoStatus

	bm := backend.BookmarkStatus{}

	var ahead, behind int

	hasUpstream := false

	for line := range strings.SplitSeq(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			name := strings.TrimPrefix(line, "# branch.head ")
			st.Ref = name
			bm.Name = name

		case strings.HasPrefix(line, "# branch.upstream "):
			upstream := strings.TrimPrefix(line, "# branch.upstream ")
			// upstream is "origin/main" — split on first "/"
			if before, _, ok := strings.Cut(upstream, "/"); ok {
				bm.Remote = before
			} else {
				bm.Remote = upstream
			}

			hasUpstream = true

		case strings.HasPrefix(line, "# branch.ab "):
			// Format: +<ahead> -<behind>
			parts := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			if len(parts) == 2 {
				ahead, _ = strconv.Atoi(strings.TrimPrefix(parts[0], "+"))
				behind, _ = strconv.Atoi(strings.TrimPrefix(parts[1], "-"))
			}

		case len(line) > 0 && line[0] != '#':
			st.Dirty = true
		}
	}

	if hasUpstream {
		bm.Ahead = ahead
		bm.Behind = behind
	}

	if bm.Name != "" {
		backend.ComputeBookmarkState(&bm)
		st.Bookmarks = []backend.BookmarkStatus{bm}
	}
	st.OverallState = backend.WorstState(st.Bookmarks, st.Conflict)

	return st
}

// Register registers the git backend with the backend registry.
func Register() {
	backend.Register(&Backend{})
}
