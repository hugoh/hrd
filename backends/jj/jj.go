// Package jj implements the jj (Jujutsu) VCS backend.
package jj

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
	colorNeverFlag       = "--color=never"
	ignoreWorkingCopyArg = "--ignore-working-copy"
	noGraphFlag          = "--no-graph"
	templateFlag         = "--template"
	separator            = "\x1f"
	partsCountMin        = 2 // minimum parts for working copy parsing
	partsCountSecond     = 2 // index for second part check
	workingCopyPartCount = 5 // number of parts in working copy output
	idxDirty             = 2 // index for dirty flag in working copy
	idxConflict          = 3 // index for conflict flag in working copy
	idxDescription       = 4 // index for description in working copy
	idxTimeAgo           = 5 // index for time ago in working copy
	cmdNameLog           = "log"
)

//nolint:gochecknoglobals // common jj log flags shared across Status calls
var logBaseArgs = []string{cmdNameLog, noGraphFlag, colorNeverFlag}

// Backend implements backend.Backend for jj repositories.
type Backend struct{}

var _ backend.Backend = (*Backend)(nil)

// Name returns the backend identifier "jj".
func (b *Backend) Name() string { return "jj" }

// Detect returns true if path contains a .jj directory.
// jj is registered before git so colocated repos (jj + .git) are claimed by jj.
func (b *Backend) Detect(path string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, ".jj"))
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("stating .jj: %w", err)
	}

	return true, nil
}

// Status queries jj for the current change, all local bookmark tracking
// states, working-copy cleanliness, and conflicts.
//
// Two subprocess calls are made:
//  1. jj log -r @ → change ID, dirty flag, conflict flag
//  2. jj bookmark list --all-remotes → structured bookmark tracking data
func (b *Backend) Status(ctx context.Context, path string) (backend.RepoStatus, error) {
	const sep = "\x1f"

	const detailTmpl = `change_id.short(8) ++ "` + sep + `" ++ ` +
		`if(diff.stat().files().len() > 0, "dirty", "") ++ "` + sep + `" ++ ` +
		`if(conflict, "conflict", "") ++ "` + sep + `" ++ ` +
		`description.first_line() ++ "` + sep + `" ++ ` +
		`committer.timestamp().ago()`

	wcArgs := append([]string{}, logBaseArgs...)
	wcArgs = append(wcArgs, "-r", "@", templateFlag, detailTmpl, ignoreWorkingCopyArg)

	wcOut, err := runJJ(ctx, path, wcArgs)
	if err != nil {
		return backend.RepoStatus{}, fmt.Errorf("jj log: %w", err)
	}

	status := parseWorkingCopy(wcOut)

	if status.CommitMsg == "" {
		fillCommitMsgFromAncestors(ctx, path, &status)
	}

	headArgs := append([]string{}, logBaseArgs...)
	headArgs = append(headArgs, "-r", "::@ & bookmarks()", "-n", "1", ignoreWorkingCopyArg,
		templateFlag, "bookmarks.first().name()")

	headOut, _ := runJJ(ctx, path, headArgs)

	headName := strings.TrimSpace(headOut)
	if headName != "" {
		bmOut, _ := runJJ(ctx, path, []string{
			"bookmark", "list", "--all-remotes", headName,
		})
		status.Bookmarks = parseBookmarks(bmOut)
	}

	status.OverallState = backend.WorstState(status.Bookmarks, status.Conflict)

	return status, nil
}

// Run executes arbitrary jj args in path.
func (b *Backend) Run(
	ctx context.Context,
	path string,
	args []string,
	interactive bool,
) (backend.RunResult, error) {
	if interactive {
		cmd := exec.CommandContext(ctx, "jj", args...) //nolint:gosec // user-invoked jj command
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
				err = nil
			}
		}

		return backend.RunResult{ExitCode: code}, err
	}

	var buf bytes.Buffer

	cmd := exec.CommandContext(ctx, "jj", args...) //nolint:gosec // user-invoked jj command
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
			return backend.RunResult{}, fmt.Errorf("jj run: %w", err)
		}
	}

	return backend.RunResult{Output: buf.String(), ExitCode: exitCode}, nil
}

//nolint:gochecknoglobals // swapped in tests to simulate jj failures
var runJJ = func(ctx context.Context, path string, args []string) (string, error) {
	var buf bytes.Buffer

	//nolint:gosec // controlled command execution, args from user input
	cmd := exec.CommandContext(
		ctx,
		"jj",
		args...,
	)
	cmd.Dir = path
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("jj %s: %w\n%s", args[0], err, buf.String())
	}

	return buf.String(), nil
}

// parseWorkingCopy parses unit-separated log template output.
//
// Fields: changeID \x1f "dirty"|"" \x1f "conflict"|"" \x1f description \x1f time_ago.
func parseWorkingCopy(raw string) backend.RepoStatus {
	parts := strings.SplitN(
		strings.TrimRight(raw, "\n"),
		separator,
		workingCopyPartCount,
	)

	var status backend.RepoStatus
	if len(parts) >= 1 {
		status.Ref = strings.TrimSpace(parts[0])
	}

	if len(parts) >= idxDirty {
		status.Dirty = strings.TrimSpace(parts[1]) == "dirty"
	}

	if len(parts) >= idxConflict {
		status.Conflict = strings.TrimSpace(parts[2]) == "conflict"
	}

	if len(parts) >= idxDescription {
		status.CommitMsg = strings.TrimSpace(parts[3])
	}

	if len(parts) >= idxTimeAgo {
		if t := strings.TrimSpace(parts[4]); t != "" {
			status.CommitTime = "(" + t + ")"
		}
	}

	return status
}

// fillCommitMsgFromAncestors walks back through ancestors to find first commit with description.
func fillCommitMsgFromAncestors(ctx context.Context, path string, status *backend.RepoStatus) {
	const maxAncestors = 10

	for i := 1; i <= maxAncestors; i++ {
		rev := "@" + strings.Repeat("-", i)

		const tmpl = `description.first_line() ++ "` + separator + `" ++ committer.timestamp().ago()`

		args := append([]string{}, logBaseArgs...)
		args = append(args, "-r", rev, ignoreWorkingCopyArg, templateFlag, tmpl)

		out, err := runJJ(ctx, path, args)
		if err != nil {
			break
		}

		if msg := extractCommitMsg(out); msg != "" {
			status.CommitMsg = msg

			if time := extractCommitTime(out); time != "" {
				status.CommitTime = "(" + time + ")"
			}

			break
		}
	}
}

func extractCommitMsg(out string) string {
	parts := strings.SplitN(strings.TrimRight(out, "\n"), separator, partsCountMin)

	return strings.TrimSpace(parts[0])
}

func extractCommitTime(out string) string {
	parts := strings.SplitN(strings.TrimRight(out, "\n"), separator, partsCountMin)

	if len(parts) >= partsCountSecond {
		return strings.TrimSpace(parts[1])
	}

	return ""
}

// parseBookmarks parses `jj bookmark list --all-remotes` output.
//
// jj output format (jj 0.21+):
//
//	main: rlkvwrto 9f3a1b2c commit message
//	  @origin (ahead by 2 commits, behind by 1 commit)
//	feature: qpvuntop 1a2b3c4d another commit
//	  (no tracking remote)
//	conflicted: (conflicted)
//	  @origin (tracking)
//	deleted@origin: (gone)
//
// - Non-indented lines are bookmark headers.
// - Indented @<remote> lines are tracking entries with ahead/behind info.
// - "(gone)" means the remote ref was deleted.
// - "(conflicted)" on the header means the bookmark is in conflict.
func parseBookmarks(
	raw string,
) []backend.BookmarkStatus {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var (
		bookmarks []backend.BookmarkStatus
		current   *backend.BookmarkStatus
	)

	flush := func() {
		if current == nil {
			return
		}

		if current.Remote == "" && current.State == backend.RefStateUnknown {
			current.State = backend.RefStateNoRemote
		}

		bookmarks = append(bookmarks, *current)
		current = nil
	}

	for line := range strings.SplitSeq(raw, "\n") {
		if line == "" {
			continue
		}

		if line[0] != ' ' && line[0] != '\t' {
			flush()

			current = handleBookmarkLine(line)

			continue
		}

		if current == nil {
			continue
		}

		handleRemoteLine(current, line)
	}

	flush()

	return bookmarks
}

func handleBookmarkLine(line string) *backend.BookmarkStatus {
	// Name is everything before the first ":"
	name := line
	if before, _, ok := strings.Cut(line, ":"); ok {
		name = before
	}

	name = strings.TrimSpace(name)

	if strings.Contains(name, "@") {
		return nil
	}

	current := &backend.BookmarkStatus{Name: name}

	if strings.Contains(line, "(conflicted)") {
		current.Conflict = true
		current.State = backend.RefStateDiverged
	}

	return current
}

func handleRemoteLine(current *backend.BookmarkStatus, line string) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "@") {
		return
	}

	// "@origin (ahead by 2 commits, behind by 1 commit)"
	remotePart := strings.TrimPrefix(trimmed, "@")

	remote := remotePart
	if idx := strings.IndexAny(remotePart, " ("); idx >= 0 {
		remote = remotePart[:idx]
	}

	remote = strings.TrimSuffix(remote, ":")

	// Skip synthetic @git remote (internal colocation bookmark, not a real remote).
	if remote == "git" {
		return
	}

	// Use the first tracking remote only.
	if current.Remote != "" {
		return
	}

	current.Remote = remote

	if strings.Contains(trimmed, "(gone)") {
		current.State = backend.RefStateGone

		return
	}

	current.Ahead = extractCount(trimmed, "ahead by")
	current.Behind = extractCount(trimmed, "behind by")
	backend.ComputeBookmarkState(current)
}

// extractCount finds "keyword N" in s and returns N.
func extractCount(s, keyword string) int {
	_, after, ok := strings.Cut(s, keyword)
	if !ok {
		return 0
	}

	rest := strings.TrimSpace(after)
	numStr, _, _ := strings.Cut(rest, " ")
	n, _ := strconv.Atoi(numStr)

	return n
}

// Register registers the jj backend with the backend registry.
func Register() {
	backend.Register(&Backend{})
}
