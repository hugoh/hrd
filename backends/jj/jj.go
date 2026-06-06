// Package jj implements the jj (Jujutsu) VCS backend.
package jj

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
	priorityJj           = 20
	colorNeverFlag       = "--color=never"
	ignoreWorkingCopyArg = "--ignore-working-copy"
	noGraphFlag          = "--no-graph"
	templateFlag         = "--template"
	separator            = "\x1f"
	cmdNameLog           = "log"
	cmdGit               = "git"
	subCmdBookmark       = "bookmark"
	subCmdList           = "list"
	opFetch              = "fetch"
	opPull               = "pull"
	opPush               = "push"
	subCmdRebase         = "rebase"
)

//nolint:gochecknoglobals // common jj log flags shared across Status calls
var logBaseArgs = []string{cmdNameLog, noGraphFlag, colorNeverFlag}

//nolint:gochecknoglobals // lookup table for jj git-prefixed subcommands
var jjPrefixedOps = map[string]bool{
	opFetch: true,
	opPush:  true,
}

//nolint:gochecknoglobals // table of multi-step operations (op → sequence of arg lists)
var multiStepOps = map[string][][]string{
	opPull: {{cmdGit, opFetch}, {subCmdRebase, "-d", "trunk()"}},
}

// Backend implements backend.Backend for jj repositories.
type Backend struct {
	runJJFn func(ctx context.Context, path string, args []string) (string, error)
}

var _ backend.Backend = (*Backend)(nil)

// Name returns the backend identifier "jj".
func (*Backend) Name() string { return "jj" }

// Priority returns the jj detection priority.
func (*Backend) Priority() int { return priorityJj }

// Detect returns true if path contains a .jj directory.
func (*Backend) Detect(path string) (bool, error) {
	ok, err := backend.DetectDir(path, ".jj")
	if err != nil {
		return false, fmt.Errorf("detect jj: %w", err)
	}

	return ok, nil
}

func (*Backend) SubcommandArgs(op string) []string {
	if jjPrefixedOps[op] {
		return []string{cmdGit, op}
	}

	return []string{op}
}

// Subcommands shells out to jj util completion bash and returns available subcommands.
func (*Backend) Subcommands(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "jj", "util", "completion", "bash").Output()
	if err != nil {
		return nil, fmt.Errorf("jj completion: %w", err)
	}

	return parseJjCmdList(string(out)), nil
}

func parseJjCmdList(completion string) []string {
	seen := make(map[string]bool)

	var cmds []string

	for line := range strings.SplitSeq(completion, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "jj,") {
			continue
		}

		rest, _, _ := strings.Cut(line, ")")

		_, sub, _ := strings.Cut(rest, ",")
		if sub == "" {
			continue
		}

		if !seen[sub] {
			seen[sub] = true
			cmds = append(cmds, sub)
		}
	}

	slices.Sort(cmds)

	return cmds
}

// Status queries jj for the current change, all local bookmark tracking
// states, working-copy cleanliness, and conflicts.
//
// Subprocess calls:
//  1. jj log -r @ → change ID, dirty flag, conflict flag
//  2. jj bookmark list --all-remotes <head> → structured bookmark tracking data
//  3. jj bookmark list --all-remotes + jj log ×2 (only when HEAD bookmark
//     has no tracking remote but a @remote counterpart exists — colocated repos)
func (b *Backend) Status(ctx context.Context, path string) (backend.RepoStatus, error) {
	const sep = "\x1f"

	const detailTmpl = `change_id.short(8) ++ "` + sep + `" ++ ` +
		`if(diff.stat().files().len() > 0, "dirty", "") ++ "` + sep + `" ++ ` +
		`if(conflict, "conflict", "") ++ "` + sep + `" ++ ` +
		`description.first_line() ++ "` + sep + `" ++ ` +
		`committer.timestamp().ago()`

	wcArgs := append([]string{}, logBaseArgs...)
	wcArgs = append(wcArgs, "-r", "@", templateFlag, detailTmpl)

	wcOut, err := b.runJJ(ctx, path, wcArgs)
	if err != nil {
		return backend.RepoStatus{}, fmt.Errorf("jj log: %w", err)
	}

	status := parseWorkingCopy(wcOut)

	if status.CommitMsg == "" {
		b.fillCommitMsgFromAncestors(ctx, path, &status)
	}

	headArgs := append([]string{}, logBaseArgs...)
	headArgs = append(headArgs, "-r", "::@ & bookmarks()", "-n", "1", ignoreWorkingCopyArg,
		templateFlag, "bookmarks.first().name()")

	headOut, _ := b.runJJ(ctx, path, headArgs)

	headName := strings.TrimSpace(headOut)
	if headName != "" {
		bmOut, _ := b.runJJ(ctx, path, []string{
			subCmdBookmark, subCmdList, "--all-remotes", headName,
		})
		status.Bookmarks = parseBookmarks(bmOut)

		// For colocated repos the HEAD bookmark may have @git tracking
		// but no @origin — look for a @remote counterpart.
		if len(status.Bookmarks) > 0 && status.Bookmarks[0].Remote == "" {
			b.enrichWithRemoteBookmark(ctx, path, headName, &status.Bookmarks[0])
		}
	}

	if headName != "" {
		status.LocalAhead = b.countRevs(ctx, path, headName+"..@")
		if status.LocalAhead > 0 {
			status.LocalAhead--
		}
	}

	status.OverallState = backend.WorstState(status.Bookmarks, status.Conflict)

	return status, nil
}

// Run executes jj args in path. Multi-step ops (pull, etc.) run each step
// sequentially, stopping on the first failure.
func (*Backend) Run(
	ctx context.Context,
	path string,
	args []string,
	interactive bool,
) (backend.RunResult, error) {
	if len(args) > 0 {
		if steps, ok := multiStepOps[args[0]]; ok {
			return runSteps(ctx, path, args[0], steps, interactive)
		}
	}

	res, err := backend.RunCommand(ctx, "jj", path, args, interactive)
	if err != nil {
		return backend.RunResult{}, fmt.Errorf("jj run: %w", err)
	}

	return res, nil
}

func (b *Backend) runJJ(ctx context.Context, path string, args []string) (string, error) {
	if b.runJJFn != nil {
		return b.runJJFn(ctx, path, args)
	}

	return defaultRunJJ(ctx, path, args)
}

// enrichWithRemoteBookmark looks for a @remote bookmark (e.g. main@origin)
// matching headName and, if found, computes ahead/behind against it.
//
// This handles colocated jj/git repos where bookmarks don't have explicit
// tracking remotes configured — the remote bookmark exists as a separate
// @remote entry in jj's bookmark list.
func (b *Backend) enrichWithRemoteBookmark(
	ctx context.Context,
	path, headName string,
	bm *backend.BookmarkStatus,
) {
	out, err := b.runJJ(ctx, path, []string{subCmdBookmark, subCmdList, "--all-remotes"})
	if err != nil || strings.TrimSpace(out) == "" {
		return
	}

	for line := range strings.SplitSeq(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, headName+"@") || !strings.Contains(trimmed, ":") {
			continue
		}

		before, _, _ := strings.Cut(trimmed, ":")

		remoteName := strings.TrimPrefix(strings.TrimSpace(before), headName+"@")
		if remoteName == "" || remoteName == cmdGit {
			continue
		}

		remoteBm := headName + "@" + remoteName
		behind := b.countRevs(ctx, path, headName+".."+remoteBm)
		ahead := b.countRevs(ctx, path, remoteBm+".."+headName)

		bm.Remote = remoteName
		bm.Ahead = ahead
		bm.Behind = behind
		backend.ComputeBookmarkState(bm)

		return
	}
}

// countRevs runs jj log -r <revset> --count and returns the number.
func (b *Backend) countRevs(ctx context.Context, path, revset string) int {
	out, err := b.runJJ(ctx, path, []string{
		cmdNameLog, colorNeverFlag, ignoreWorkingCopyArg,
		"-r", revset, "--count",
	})
	if err != nil {
		return 0
	}

	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return 0
	}

	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0
	}

	return n
}

// fillCommitMsgFromAncestors walks back through ancestors to find first commit with description.
func (b *Backend) fillCommitMsgFromAncestors(
	ctx context.Context, path string, status *backend.RepoStatus,
) {
	const maxAncestors = 10

	for i := 1; i <= maxAncestors; i++ {
		rev := "@" + strings.Repeat("-", i)

		const tmpl = `description.first_line() ++ "` + separator + `" ++ committer.timestamp().ago()`

		args := append([]string{}, logBaseArgs...)
		args = append(args, "-r", rev, ignoreWorkingCopyArg, templateFlag, tmpl)

		out, err := b.runJJ(ctx, path, args)
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

// runSteps executes each step sequentially, returning on the first non-zero
// exit or infrastructure error.
func runSteps(
	ctx context.Context,
	path, op string,
	steps [][]string,
	interactive bool,
) (backend.RunResult, error) {
	for _, step := range steps {
		res, err := backend.RunCommand(ctx, "jj", path, step, interactive)
		if err != nil {
			return backend.RunResult{}, fmt.Errorf("jj %s: %w", op, err)
		}

		if res.ExitCode != 0 {
			return res, nil
		}
	}

	return backend.RunResult{}, nil
}

func defaultRunJJ(ctx context.Context, path string, args []string) (string, error) {
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
	const (
		idxDirty = iota + 2
		idxConflict
		idxDescription
		idxTimeAgo
	)

	parts := strings.SplitN(
		strings.TrimRight(raw, "\n"),
		separator,
		5, //nolint:mnd
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

func extractCommitMsg(out string) string {
	parts := strings.SplitN(strings.TrimRight(out, "\n"), separator, 2) //nolint:mnd

	return strings.TrimSpace(parts[0])
}

func extractCommitTime(out string) string {
	parts := strings.SplitN(strings.TrimRight(out, "\n"), separator, 2) //nolint:mnd

	if len(parts) >= 2 { //nolint:mnd
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
	if remote == cmdGit {
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

	current.Ahead = extractCount(trimmed, "behind by")
	current.Behind = extractCount(trimmed, "ahead by")
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
	if err := backend.Register(&Backend{runJJFn: defaultRunJJ}); err != nil {
		panic(fmt.Sprintf("jj: %v", err))
	}
}
