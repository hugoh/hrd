// Package jj implements the jj (Jujutsu) VCS backend.
package jj

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"golang.org/x/sync/errgroup"
)

const (
	priorityJj           = 20
	colorNeverFlag       = "--color=never"
	ignoreWorkingCopyArg = "--ignore-working-copy"
	noGraphFlag          = "--no-graph"
	templateFlag         = "--template"
	cmdNameLog           = "log"
	cmdGit               = "git"
	subCmdBookmark       = "bookmark"
	subCmdList           = "list"
	opFetch              = "fetch"
	opPull               = "pull"
	opPush               = "push"
	subCmdRebase         = "rebase"
)

// detailTmpl renders a working-copy/ancestor commit as a JSON object. jj has
// no whole-object JSON serializer, so the object is built by hand-joining
// "key": json(value) segments — json() guarantees each value is safely
// escaped, unlike the \x1f-separator splitting this replaced.
const detailTmpl = `"{\"changeId\":" ++ json(change_id.short(8)) ++ ` +
	`",\"dirty\":" ++ json(diff.stat().files().len() > 0) ++ ` +
	`",\"conflict\":" ++ json(conflict) ++ ` +
	`",\"description\":" ++ json(description.first_line()) ++ ` +
	`",\"ago\":" ++ json(committer.timestamp().ago()) ++ "}"`

// bookmarkTmpl renders one CommitRef (jj bookmark list -T) per line as JSON.
//
// ahead/behind are intentionally read from the opposite-named jj function:
// jj reports tracking_ahead_count/tracking_behind_count from the remote
// ref's own perspective, e.g. a remote ref that is "ahead" of the local
// tracking ref means the local bookmark is that many commits *behind*.
//
// json() must wrap each branch of if(tracked, ...) individually rather than
// the whole if() — wrapping the whole expression makes jj eagerly evaluate
// both branches (erroring on tracking_*_count for untracked/local refs)
// instead of lazily rendering only the matching one.
const bookmarkTmpl = `"{\"name\":" ++ json(name) ++ ` +
	`",\"remote\":" ++ json(remote) ++ ` +
	`",\"tracked\":" ++ json(tracked) ++ ` +
	`",\"present\":" ++ json(present) ++ ` +
	`",\"conflict\":" ++ json(conflict) ++ ` +
	`",\"ahead\":" ++ if(tracked, json(tracking_behind_count.lower()), json(0)) ++ ` +
	`",\"behind\":" ++ if(tracked, json(tracking_ahead_count.lower()), json(0)) ++ "}\n"`

// wcDetail mirrors detailTmpl's JSON shape.
type wcDetail struct {
	ChangeID    string `json:"changeId"`
	Dirty       bool   `json:"dirty"`
	Conflict    bool   `json:"conflict"`
	Description string `json:"description"`
	Ago         string `json:"ago"`
}

// bookmarkRef mirrors bookmarkTmpl's JSON shape — one CommitRef entry.
type bookmarkRef struct {
	Name     string `json:"name"`
	Remote   string `json:"remote"`
	Tracked  bool   `json:"tracked"`
	Present  bool   `json:"present"`
	Conflict bool   `json:"conflict"`
	Ahead    int    `json:"ahead"`
	Behind   int    `json:"behind"`
}

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

func (*Backend) Name() string { return "jj" }

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
//  2. jj log -r ::@ & bookmarks() → nearest bookmark name (independent of #1,
//     runs concurrently with it)
//  3. jj bookmark list --all-remotes <head> → structured bookmark tracking data.
//     jj itself computes ahead/behind for tracked remotes; for a present but
//     untracked remote (colocated repos where the counterpart isn't an
//     explicit tracking remote), ahead/behind is computed with an extra
//     jj log --count call per direction (see countRevs).
//  4. jj log --count <head>..@ → local-ahead count (independent of #3, runs
//     concurrently with it)
func (b *Backend) Status(ctx context.Context, path string) (backend.RepoStatus, error) {
	status, headName, err := b.fetchWorkingCopyAndHead(ctx, path)
	if err != nil {
		return backend.RepoStatus{}, err
	}

	b.fillBookmarkTracking(ctx, path, headName, &status)

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
	if len(args) == 0 {
		return backend.RunResult{}, backend.ErrNoArgs
	}

	if steps, ok := multiStepOps[args[0]]; ok {
		return runSteps(ctx, path, args[0], steps, interactive)
	}

	//nolint:wrapcheck // RunTool already wraps with the binary name and subcommand
	return backend.RunTool(ctx, "jj", path, args, interactive)
}

func (b *Backend) runJJ(ctx context.Context, path string, args []string) (string, error) {
	if b.runJJFn != nil {
		return b.runJJFn(ctx, path, args)
	}

	return defaultRunJJ(ctx, path, args)
}

// fetchWorkingCopyAndHead runs the working-copy detail query and the
// nearest-bookmark-name query concurrently, since neither depends on the
// other's output.
func (b *Backend) fetchWorkingCopyAndHead(
	ctx context.Context,
	path string,
) (backend.RepoStatus, string, error) {
	wcArgs := append([]string{}, logBaseArgs...)
	wcArgs = append(wcArgs, "-r", "@", templateFlag, detailTmpl)

	headArgs := append([]string{}, logBaseArgs...)
	headArgs = append(headArgs, "-r", "::@ & bookmarks()", "-n", "1", ignoreWorkingCopyArg,
		templateFlag, "bookmarks.first().name()")

	var wcOut, headOut string

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error

		wcOut, err = b.runJJ(gctx, path, wcArgs)

		return err
	})
	g.Go(func() error {
		headOut, _ = b.runJJ(gctx, path, headArgs)

		return nil
	})

	if err := g.Wait(); err != nil {
		return backend.RepoStatus{}, "", fmt.Errorf("jj log: %w", err)
	}

	status, err := parseWorkingCopy(wcOut)
	if err != nil {
		return backend.RepoStatus{}, "", fmt.Errorf("jj log: %w", err)
	}

	return status, strings.TrimSpace(headOut), nil
}

// fillBookmarkTracking runs the ancestor commit-message fill, bookmark-list
// tracking query, and local-ahead count concurrently — each depends only on
// headName/status.CommitMsg, not on each other's output.
func (b *Backend) fillBookmarkTracking(
	ctx context.Context,
	path, headName string,
	status *backend.RepoStatus,
) {
	var g errgroup.Group

	if status.CommitMsg == "" {
		g.Go(func() error {
			b.fillCommitMsgFromAncestors(ctx, path, status)

			return nil
		})
	}

	if headName != "" {
		g.Go(func() error {
			bmOut, _ := b.runJJ(ctx, path, []string{
				subCmdBookmark, subCmdList, "--all-remotes", headName, templateFlag, bookmarkTmpl,
			})
			status.Bookmarks = parseBookmarkRefs(bmOut, func(name, remote string) (int, int) {
				remoteRef := name + "@" + remote
				ahead := b.countRevs(ctx, path, remoteRef+".."+name)
				behind := b.countRevs(ctx, path, name+".."+remoteRef)

				return ahead, behind
			})

			return nil
		})
		g.Go(func() error {
			status.LocalAhead = b.countRevs(ctx, path, headName+"..@")
			if status.LocalAhead > 0 {
				status.LocalAhead--
			}

			return nil
		})
	}

	_ = g.Wait()
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

// ancestorSearchDepth bounds how far back fillCommitMsgFromAncestors looks
// for a described commit — @ itself (always empty here) plus up to 10
// ancestors, matching the previous ancestor-walk's loop bound.
const ancestorSearchDepth = 11

// fillCommitMsgFromAncestors finds the nearest ancestor with a non-empty
// description in a single jj call, instead of walking @-, @--, ... one
// subprocess at a time.
func (b *Backend) fillCommitMsgFromAncestors(
	ctx context.Context, path string, status *backend.RepoStatus,
) {
	rev := fmt.Sprintf(`heads(ancestors(@, %d) & description(glob:"?*"))`, ancestorSearchDepth)

	args := append([]string{}, logBaseArgs...)
	args = append(args, "-r", rev, "-n", "1", ignoreWorkingCopyArg, templateFlag, detailTmpl)

	out, err := b.runJJ(ctx, path, args)
	if err != nil {
		return
	}

	detail, err := parseWorkingCopy(out)
	if err != nil {
		return
	}

	if detail.CommitMsg != "" {
		status.CommitMsg = detail.CommitMsg
		status.CommitTime = detail.CommitTime
	}
}

// runSteps executes each step sequentially, stopping on the first non-zero
// exit or infrastructure error. Output from all executed steps is
// accumulated so the caller sees the full transcript, not just the last step.
func runSteps(
	ctx context.Context,
	path, op string,
	steps [][]string,
	interactive bool,
) (backend.RunResult, error) {
	var out strings.Builder

	for _, step := range steps {
		res, err := backend.RunCommand(ctx, "jj", path, step, interactive)
		if err != nil {
			return backend.RunResult{}, fmt.Errorf("jj %s: %w", op, err)
		}

		out.WriteString(res.Output)

		if res.ExitCode != 0 {
			return backend.RunResult{Output: out.String(), ExitCode: res.ExitCode}, nil
		}
	}

	return backend.RunResult{Output: out.String()}, nil
}

func defaultRunJJ(ctx context.Context, path string, args []string) (string, error) {
	var stdout, stderr bytes.Buffer

	//nolint:gosec // controlled command execution, args from user input
	cmd := exec.CommandContext(
		ctx,
		"jj",
		args...,
	)
	cmd.Dir = path
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("jj %s: %w\n%s", args[0], err, stderr.String())
	}

	return stdout.String(), nil
}

// parseWorkingCopy decodes detailTmpl's JSON output. An error means the jj
// invocation succeeded but its output wasn't the JSON we expected — the
// caller must treat this as a failure, not as "nothing to report".
func parseWorkingCopy(raw string) (backend.RepoStatus, error) {
	var detail wcDetail
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &detail); err != nil {
		return backend.RepoStatus{}, fmt.Errorf("decode jj log output: %w", err)
	}

	status := backend.RepoStatus{
		Ref:       detail.ChangeID,
		Dirty:     detail.Dirty,
		Conflict:  detail.Conflict,
		CommitMsg: detail.Description,
	}

	if detail.Ago != "" {
		status.CommitTime = "(" + detail.Ago + ")"
	}

	return status, nil
}

// parseBookmarkRefs decodes bookmarkTmpl's output — one JSON CommitRef per
// line — and groups entries by name into backend.BookmarkStatus, picking the
// first non-@git remote for ahead/behind/gone state. For a present remote
// that isn't tracked, resolveUntracked (may be nil) computes real ahead/behind
// counts, since jj's own tracking_ahead_count/tracking_behind_count are only
// meaningful for tracked remotes.
func parseBookmarkRefs(
	raw string,
	resolveUntracked func(name, remote string) (ahead, behind int),
) []backend.BookmarkStatus {
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
		if strings.TrimSpace(line) == "" {
			continue
		}

		var ref bookmarkRef
		if err := json.Unmarshal([]byte(line), &ref); err != nil {
			continue
		}

		if ref.Remote == "" {
			flush()

			current = newBookmarkStatus(ref)

			continue
		}

		applyRemoteRef(current, ref, resolveUntracked)
	}

	flush()

	return bookmarks
}

func newBookmarkStatus(ref bookmarkRef) *backend.BookmarkStatus {
	bm := &backend.BookmarkStatus{Name: ref.Name, Conflict: ref.Conflict}
	if ref.Conflict {
		bm.State = backend.RefStateDiverged
	}

	return bm
}

// applyRemoteRef folds a remote CommitRef entry into the bookmark it
// belongs to, skipping the synthetic @git remote (colocation bookmark, not
// a real remote), entries for a different bookmark, and an already-set remote.
// For a present but untracked remote, resolveUntracked (may be nil) computes
// real ahead/behind counts, since jj only reports tracking_ahead_count/
// tracking_behind_count for tracked remotes.
func applyRemoteRef(
	current *backend.BookmarkStatus,
	ref bookmarkRef,
	resolveUntracked func(name, remote string) (ahead, behind int),
) {
	if current == nil || current.Name != ref.Name || ref.Remote == cmdGit || current.Remote != "" {
		return
	}

	current.Remote = ref.Remote

	if !ref.Present {
		// A bookmark conflict (RefStateDiverged, set in newBookmarkStatus)
		// is more specific than "remote gone" — don't clobber it.
		if !current.Conflict {
			current.State = backend.RefStateGone
		}

		return
	}

	switch {
	case ref.Tracked:
		current.Ahead = ref.Ahead
		current.Behind = ref.Behind
	case resolveUntracked != nil:
		current.Ahead, current.Behind = resolveUntracked(ref.Name, ref.Remote)
	}

	backend.ComputeBookmarkState(current)
}

// Register adds the jj backend to the global registry.
func Register() {
	if err := backend.Register(&Backend{runJJFn: defaultRunJJ}); err != nil {
		panic(fmt.Sprintf("jj: %v", err))
	}
}
