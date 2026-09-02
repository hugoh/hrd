// Package backend defines the VCS backend interface and shared types.
// New VCS backends implement the Backend interface and register themselves
// via Register; the tool then uses them for detection, status, and dispatch.
package backend

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
)

// RefState describes the relationship between a local ref and its remote.
type RefState int

const (
	// RefStateUnknown means the bookmark has no tracking remote.
	RefStateUnknown RefState = iota
	// RefStateSynced means local matches remote.
	RefStateSynced
	// RefStateAhead means local is ahead of remote.
	RefStateAhead
	// RefStateBehind means local is behind remote.
	RefStateBehind
	// RefStateDiverged means local and remote have diverged.
	RefStateDiverged
	// RefStateNoRemote means no remote is configured.
	RefStateNoRemote
	// RefStateGone means remote ref was deleted.
	RefStateGone
)

const (
	stateStrSynced  = "synced"
	stateStrUnknown = "unknown"
)

func (s RefState) String() string {
	switch s {
	case RefStateSynced:
		return stateStrSynced
	case RefStateAhead:
		return "ahead"
	case RefStateBehind:
		return "behind"
	case RefStateDiverged:
		return "diverged"
	case RefStateNoRemote:
		return "local"
	case RefStateGone:
		return "gone"
	case RefStateUnknown:
		return stateStrUnknown
	default:
		return stateStrUnknown
	}
}

const (
	PriorityJj  = 20
	PriorityGit = 10

	rankDiverged = 4
	rankBehind   = 3
	rankAhead    = 2
	rankNoRemote = 1
	rankSynced   = 0
)

// Severity order: Conflict > Diverged > Behind > Ahead > NoRemote > Synced.
func (s RefState) Severity() int {
	switch s {
	case RefStateDiverged, RefStateGone:
		return rankDiverged
	case RefStateBehind:
		return rankBehind
	case RefStateAhead:
		return rankAhead
	case RefStateNoRemote:
		return rankNoRemote
	case RefStateSynced:
		return rankSynced
	case RefStateUnknown:
		return rankSynced
	default:
		return rankSynced
	}
}

// BookmarkStatus holds tracking information for a single bookmark (jj) or
// the current branch (git). Both backends populate this struct so the UI
// can render them identically.
type BookmarkStatus struct {
	Name string

	// Remote name (e.g. "origin"). Empty when no remote.
	Remote string

	Ahead  int
	Behind int

	// True when jj reports a bookmark conflict (diverged push).
	// For git this is always false; git represents this as a diverged state.
	Conflict bool

	// Computed sync state derived from Ahead/Behind/Remote.
	State RefState
}

// RepoStatus is the unified status schema populated by each backend.
type RepoStatus struct {
	// Ref is the human-readable current position: branch name for git,
	// change ID short for jj.
	Ref string

	// Bookmarks holds tracking state for all local bookmarks/branches.
	// Git populates exactly one entry (the current branch).
	// jj populates one entry per local bookmark at or near @.
	Bookmarks []BookmarkStatus

	// OverallState is the worst-case RefState across all Bookmarks,
	// used for sorting and the summary colour in the ll table.
	OverallState RefState

	Dirty bool

	// True when the repo has unresolved VCS conflicts
	// (jj conflict markers, git merge conflicts).
	Conflict bool

	CommitMsg string

	// Relative commit time (e.g. "3 days ago").
	CommitTime string

	// Working copy (@) commits ahead of HEAD bookmark. Only populated by
	// the jj backend (git's working copy is always at the branch tip).
	LocalAhead int

	// Commits reachable from the working copy/branch tip that trunk (jj's
	// trunk() revset, or git's resolved default branch) doesn't have yet.
	// Nonzero regardless of which bookmark/branch is checked out, so it
	// catches unmerged work on a feature branch even when that branch is
	// fully synced with its own remote.
	TrunkAhead int

	// True when the checked-out bookmark/branch is not trunk's, even if it
	// points at the same commit as trunk (so TrunkAhead is 0). Left false
	// when either name can't be determined.
	NotOnTrunk bool
}

// NeedsAttention reports whether the repo has uncommitted changes, its
// branch differs from its remote (ahead, behind, diverged, or gone), it has
// commits not yet merged into trunk, or it's checked out on a bookmark/
// branch other than trunk's. Repos with no remote configured do not qualify
// on sync grounds alone.
func (st RepoStatus) NeedsAttention() bool {
	if st.Dirty || st.LocalAhead > 0 || st.TrunkAhead > 0 || st.NotOnTrunk {
		return true
	}

	for _, bm := range st.Bookmarks {
		if bm.Ahead > 0 || bm.Behind > 0 {
			return true
		}
	}

	return false
}

// ComputeBookmarkState derives the RefState for a single BookmarkStatus from
// its Ahead/Behind counts and whether a remote is configured.
func ComputeBookmarkState(bookmark *BookmarkStatus) {
	if bookmark.Conflict {
		bookmark.State = RefStateDiverged

		return
	}

	switch {
	case bookmark.Remote == "":
		bookmark.State = RefStateNoRemote
	case bookmark.Ahead > 0 && bookmark.Behind > 0:
		bookmark.State = RefStateDiverged
	case bookmark.Ahead > 0:
		bookmark.State = RefStateAhead
	case bookmark.Behind > 0:
		bookmark.State = RefStateBehind
	default:
		bookmark.State = RefStateSynced
	}
}

// WorstState returns the most severe RefState across all bookmarks.
// Severity order: Conflict > Diverged > Behind > Ahead > NoRemote > Synced.
func WorstState(bookmarks []BookmarkStatus, hasConflict bool) RefState {
	if hasConflict {
		return RefStateDiverged // surface conflict as diverged at repo level
	}

	if len(bookmarks) == 0 {
		return RefStateNoRemote
	}

	worst := bookmarks[0].State
	for _, bm := range bookmarks[1:] {
		if bm.State.Severity() > worst.Severity() {
			worst = bm.State
		}
	}

	return worst
}

// RunResult holds the outcome of a single repo dispatch.
type RunResult struct {
	// Output is the combined stdout+stderr of the subprocess, captured
	// when running in parallel. In interactive mode this is empty.
	Output   string
	ExitCode int
}

// Backend is the interface every VCS backend must implement.
// Backends are stateless; all repo-specific state is passed per call.
type Backend interface {
	// Name returns the canonical backend identifier, e.g. "git" or "jj".
	Name() string

	// Priority returns the detection priority. Higher values are checked first.
	// For colocated repos, the backend with the highest priority wins.
	Priority() int

	// Detect returns true if the directory at path is managed by this VCS.
	// It must not return an error for non-matching dirs; only real I/O
	// failures warrant an error.
	Detect(path string) (bool, error)

	// Status returns the unified status for the repo at path.
	Status(ctx context.Context, path string) (RepoStatus, error)

	// SubcommandArgs returns the full argument list for a VCS operation.
	// For example, git returns ["fetch"] but jj returns ["git", "fetch"]
	// because jj wraps git operations under the "git" subcommand.
	SubcommandArgs(op string) []string

	// Subcommands returns available subcommands for the VCS tool, e.g.
	// ["add", "branch", "log", "status"] for git. Returns an empty slice
	// when subcommands cannot be loaded (tool not found, parse error, etc).
	Subcommands(ctx context.Context) ([]string, error)

	// Run executes the given args using this VCS tool in path.
	// When interactive is true, the caller has already arranged for the
	// subprocess to inherit the terminal; Run must not capture output.
	// When interactive is false, implementations going through
	// backend.RunCommand/RunTool get credential and host-key prompting
	// disabled and a bounded timeout for free — non-interactive runs must
	// fail fast rather than block waiting on a prompt nothing can answer.
	Run(ctx context.Context, path string, args []string, interactive bool) (RunResult, error)
}

// Detection priority follows registration order.
//
//nolint:gochecknoglobals // intentional: plugin registry
var (
	errDuplicateBackend = errors.New("duplicate backend")
	registry            []Backend
)

// Register adds a backend to the global registry.
// Detection priority follows Backend.Priority() order (higher first).
func Register(backend Backend) error {
	for _, existing := range registry {
		if existing.Name() == backend.Name() {
			return fmt.Errorf("%w: %q", errDuplicateBackend, backend.Name())
		}
	}

	idx := len(registry)
	for i, existing := range registry {
		if backend.Priority() > existing.Priority() {
			idx = i

			break
		}
	}

	registry = append(registry, nil)
	copy(registry[idx+1:], registry[idx:])
	registry[idx] = backend

	return nil
}

// ClearRegisteredBackends removes all registered backends.
// Used by tests that need a clean registry.
func ClearRegisteredBackends() {
	registry = nil
}

// Names returns all registered backend names, in priority order.
func Names() []string {
	names := make([]string, len(registry))
	for i, be := range registry {
		names[i] = be.Name()
	}

	return names
}

// ByName returns the backend with the given name, or an error if not found.
//
//nolint:ireturn // returning interface is intentional for plugin architecture
func ByName(name string) (Backend, error) {
	for _, be := range registry {
		if be.Name() == name {
			return be, nil
		}
	}

	return nil, fmt.Errorf("unknown backend %q: %w", name, errUnknownBackend)
}

var errUnknownBackend = errors.New("unknown backend")

// detectCache memoizes Detect results per absolute path: detection stats
// the filesystem for every status row and dispatch target, and a path's
// VCS does not change within a single invocation.
//
//nolint:gochecknoglobals // process-lifetime cache
var detectCache sync.Map

type detectResult struct {
	backend Backend
	err     error
}

// Detect returns the highest-priority backend that claims the directory.
// Priority follows DetectAll semantics — jj wins over git for colocated repos.
// Results (including failures) are memoized per absolute path; use
// ResetDetectCache to invalidate.
//
//nolint:ireturn // returning interface is intentional for plugin architecture
func Detect(path string) (Backend, error) {
	abs, err := filepath.Abs(path)
	if err != nil { // coverage-ignore — only fails on nil, caller controls input
		return nil, fmt.Errorf("resolving path %q: %w", path, err)
	}

	if v, ok := detectCache.Load(abs); ok {
		res, _ := v.(detectResult)

		return res.backend, res.err
	}

	var res detectResult

	matched, err := DetectAll(abs)
	if err != nil {
		res.err = err
	} else {
		res.backend = matched[0]
	}

	detectCache.Store(abs, res)

	return res.backend, res.err
}

// ResetDetectCache clears the memoized Detect results. Used by tests and
// by long-lived callers (TUI refresh) that want fresh detection.
func ResetDetectCache() {
	detectCache.Clear()
}

var errNoKnownVCS = errors.New("no known VCS detected")

// DetectAll returns all backends that claim the directory, in priority order
// with jj first when both jj and git are present.
func DetectAll(path string) ([]Backend, error) {
	abs, err := filepath.Abs(path)
	if err != nil { // coverage-ignore — only fails on nil, caller controls input
		return nil, fmt.Errorf("resolving path %q: %w", path, err)
	}

	var matched []Backend

	for _, backendEntry := range registry {
		ok, err := backendEntry.Detect(abs)
		if err != nil {
			return nil, fmt.Errorf("backend %q detect: %w", backendEntry.Name(), err)
		}

		if ok {
			matched = append(matched, backendEntry)
		}
	}

	if len(matched) == 0 {
		return nil, fmt.Errorf("no known VCS detected at %q: %w", abs, errNoKnownVCS)
	}

	return matched, nil
}
