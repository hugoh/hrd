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
	"golang.org/x/sync/errgroup"
)

const (
	priorityGit  = 10
	abPartsCount = 2     // number of parts in "branch.ab +<ahead> -<behind>"
	logCmd       = "log" // log command name
	subCmdStatus = "status"
	// commitLogFormat combines the subject and relative date into one `git
	// log` call; %x00 is git's own escape for a literal NUL byte in its
	// output, which can't appear in either field, so splitting on it can't
	// misparse a subject line.
	commitLogFormat = "%s%x00%cd"
)

// commitLogFieldSep is the actual NUL byte commitLogFormat's %x00 renders
// as, used to split git log's output (as opposed to commitLogFormat itself,
// which must stay the literal 4-character escape git expects in argv).
const commitLogFieldSep = "\x00"

// Backend implements backend.Backend for git repositories.
type Backend struct {
	runGitFn func(ctx context.Context, path string, args []string) (string, error)
}

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
// tree cleanliness using a single `git status --porcelain=v2 --branch` call,
// run concurrently with the configured-remotes lookup since neither depends
// on the other.
func (b *Backend) Status(ctx context.Context, path string) (backend.RepoStatus, error) {
	var out string

	var remotes []string

	var trunkRef string

	var trunkAhead int

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error

		out, err = b.runGit(gctx, path, []string{subCmdStatus, "--porcelain=v2", "--branch"})

		return err
	})
	g.Go(func() error {
		remotes = knownRemotesUsing(gctx, path, b.runGit)

		return nil
	})
	g.Go(func() error {
		trunkRef, trunkAhead = b.trunkStatus(gctx, path)

		return nil
	})

	if err := g.Wait(); err != nil {
		return backend.RepoStatus{}, fmt.Errorf("git status: %w", err)
	}

	status := parseStatus(out, remotes)
	status.TrunkAhead = trunkAhead

	if trunkName := trunkLocalName(trunkRef); trunkName != "" && status.Ref != "" {
		status.NotOnTrunk = status.Ref != trunkName
	}

	logOut, _ := b.runGit(
		ctx,
		path,
		[]string{logCmd, "-1", "--format=" + commitLogFormat, "--date=relative"},
	)
	msg, relTime, _ := strings.Cut(logOut, commitLogFieldSep)
	status.CommitMsg = strings.TrimSpace(msg)

	if t := strings.TrimSpace(relTime); t != "" {
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
	//nolint:wrapcheck // RunTool already wraps with the binary name and subcommand
	return backend.RunTool(ctx, "git", path, args, interactive)
}

// runGit executes internal status queries, using runGitFn if set (for
// tests), falling back to the real git binary otherwise.
func (b *Backend) runGit(ctx context.Context, path string, args []string) (string, error) {
	if b.runGitFn != nil {
		return b.runGitFn(ctx, path, args)
	}

	return runGit(ctx, path, args)
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

// trunkOriginHEAD is the short name for-each-ref reports for
// refs/remotes/origin/HEAD.
const trunkOriginHEAD = "origin/HEAD"

// trunkRefPatterns are the refs resolveTrunkRef looks up in a single
// for-each-ref call: origin/HEAD's symref target (authoritative when set),
// plus candidate branch names tried in priority order as a fallback.
func trunkRefPatterns() []string {
	return []string{
		"refs/remotes/origin/HEAD",
		"refs/heads/main", "refs/heads/master",
		"refs/remotes/origin/main", "refs/remotes/origin/master",
	}
}

// trunkCandidates are tried in order when refs/remotes/origin/HEAD isn't set
// (e.g. never fetched, or origin/HEAD wasn't configured).
func trunkCandidates() []string {
	return []string{"origin/main", "origin/master", "main", "master"}
}

// trunkStatus resolves the repo's trunk ref and how many commits on HEAD
// aren't reachable from it, i.e. work not yet merged into trunk. Returns
// ("", 0) if no trunk ref can be resolved, or on any git error.
func (b *Backend) trunkStatus(ctx context.Context, path string) (string, int) {
	trunk := b.resolveTrunkRef(ctx, path)
	if trunk == "" {
		return "", 0
	}

	out, err := b.runGit(ctx, path, []string{"rev-list", "--count", trunk + "..HEAD"})
	if err != nil {
		return trunk, 0
	}

	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return trunk, 0
	}

	return trunk, n
}

// trunkLocalName strips a remote prefix (e.g. "origin/") from a trunk ref
// resolved by resolveTrunkRef, for comparison against the local branch name
// git status reports.
func trunkLocalName(trunk string) string {
	_, name, found := strings.Cut(trunk, "/")
	if !found {
		return trunk
	}

	return name
}

// resolveTrunkRef finds the repo's trunk ref in a single for-each-ref call:
// refs/remotes/origin/HEAD's symref target if set, otherwise the first of
// trunkCandidates that exists.
func (b *Backend) resolveTrunkRef(ctx context.Context, path string) string {
	args := append(
		[]string{"for-each-ref", "--format=%(refname:short) %(symref:short)"},
		trunkRefPatterns()...)

	out, err := b.runGit(ctx, path, args)
	if err != nil {
		return ""
	}

	found := make(map[string]bool)

	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		name, symref, _ := strings.Cut(line, " ")

		if name == trunkOriginHEAD && symref != "" {
			return symref
		}

		if name != "" {
			found[name] = true
		}
	}

	for _, candidate := range trunkCandidates() {
		if found[candidate] {
			return candidate
		}
	}

	return ""
}

// knownRemotesUsing returns configured remote names for the repo at path,
// using the given run function. Returns nil if the query fails (no remotes,
// or not a git repo).
func knownRemotesUsing(
	ctx context.Context,
	path string,
	run func(context.Context, string, []string) (string, error),
) []string {
	out, err := run(ctx, path, []string{"remote"})
	if err != nil {
		return nil
	}

	return strings.Fields(strings.TrimSpace(out))
}

// knownRemotes returns configured remote names for the repo at path.
func knownRemotes(ctx context.Context, path string) []string {
	return knownRemotesUsing(ctx, path, runGit)
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
