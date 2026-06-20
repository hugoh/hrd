// Package git implements the git VCS backend.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
)

const (
	priorityGit  = 10
	abPartsCount = 2      // number of parts in "branch.ab +<ahead> -<behind>"
	headRef      = "HEAD" // HEAD reference name
	logCmd       = "log"  // log command name
	subCmdStatus = "status"
)

// Backend implements backend.Backend for git repositories.
type Backend struct{}

var _ backend.Backend = (*Backend)(nil)

func (*Backend) Name() string { return "git" }

// Priority returns the git detection priority.
func (*Backend) Priority() int { return priorityGit }

// Detect checks for a .git directory.
func (*Backend) Detect(path string) (bool, error) {
	ok, err := backend.DetectDir(path, ".git")
	if err != nil {
		return false, fmt.Errorf("detect git: %w", err)
	}

	return ok, nil
}

// Status queries git for the current branch/remote relationship and working
// tree cleanliness using a single `git status --porcelain=v2 --branch` call.
func (*Backend) Status(ctx context.Context, path string) (backend.RepoStatus, error) {
	out, err := runGit(ctx, path, []string{subCmdStatus, "--porcelain=v2", "--branch"})
	if err != nil {
		return backend.RepoStatus{}, fmt.Errorf("git status: %w", err)
	}

	remotes := knownRemotes(ctx, path)
	status := parseStatus(out, remotes)

	msgOut, _ := runGit(ctx, path, []string{"show-branch", "--no-name", headRef})
	status.CommitMsg = strings.TrimSpace(msgOut)

	timeOut, _ := runGit(ctx, path, []string{logCmd, "-1", "--format=%cd", "--date=relative"})
	if t := strings.TrimSpace(timeOut); t != "" {
		status.CommitTime = "(" + t + ")"
	}

	return status, nil
}

func (*Backend) SubcommandArgs(op string) []string {
	return []string{op}
}

// Subcommands shells out to git help -a and returns available subcommands.
func (*Backend) Subcommands(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "git", "help", "-a").Output()
	if err != nil {
		return nil, fmt.Errorf("git help: %w", err)
	}

	return parseGitCmdList(string(out)), nil
}

func parseGitCmdList(help string) []string {
	seen := make(map[string]bool)

	var cmds []string

	for line := range strings.SplitSeq(help, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if !strings.HasPrefix(line, "   ") && !strings.HasPrefix(line, "\t") {
			continue
		}

		name, _, _ := strings.Cut(trimmed, " ")

		if !seen[name] {
			seen[name] = true
			cmds = append(cmds, name)
		}
	}

	slices.Sort(cmds)

	return cmds
}

// Run executes arbitrary git args in path.
func (*Backend) Run(
	ctx context.Context,
	path string,
	args []string,
	interactive bool,
) (backend.RunResult, error) {
	if len(args) == 0 {
		return backend.RunResult{}, backend.ErrNoArgs
	}

	res, err := backend.RunCommand(ctx, "git", path, args, interactive)
	if err != nil {
		return backend.RunResult{}, fmt.Errorf("git %s: %w", args[0], err)
	}

	return res, nil
}

// runGit is a helper for internal status queries.
func runGit(ctx context.Context, path string, args []string) (string, error) {
	var buf bytes.Buffer

	//nolint:gosec // internal git command execution
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = path
	cmd.Stdout = &buf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}

	return buf.String(), nil
}

// knownRemotes returns configured remote names for the repo at path.
// Returns nil if the query fails (no remotes, or not a git repo).
func knownRemotes(ctx context.Context, path string) []string {
	out, err := runGit(ctx, path, []string{"remote"})
	if err != nil {
		return nil
	}

	return strings.Fields(strings.TrimSpace(out))
}

// parseStatus parses `git status --porcelain=v2 --branch` output into a
// RepoStatus. The porcelain v2 format is stable across git versions.
//
// Relevant header lines:
//
//	# branch.head <name>          current branch (or "(detached)")
//	# branch.upstream <remote/branch>  remote tracking ref
//	# branch.ab +<ahead> -<behind>     commit delta
func parseStatus(raw string, remotes []string) backend.RepoStatus {
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
			handleUpstream(&bookmark, upstream, remotes)

			hasUpstream = true

			continue
		}

		if ab, ok := strings.CutPrefix(line, "# branch.ab "); ok {
			ahead, behind = handleAheadBehind(ab)

			continue
		}

		if line == "" || line[0] == '#' {
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

func handleUpstream(bookmark *backend.BookmarkStatus, upstream string, remotes []string) {
	// Match longest known remote prefix first (handles remote names with "/").
	var match string

	for _, r := range remotes {
		if strings.HasPrefix(upstream, r+"/") && len(r) > len(match) {
			match = r
		}
	}

	if match != "" {
		bookmark.Remote = match

		return
	}
	// Fallback: first segment before "/" (covers "origin/main").
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
	if err := backend.Register(&Backend{}); err != nil {
		panic(fmt.Sprintf("git: %v", err))
	}
}
