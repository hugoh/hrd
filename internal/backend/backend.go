// Package backend defines the VCS backend interface and shared types.
// New VCS backends implement the Backend interface and register themselves
// via Register; the tool then uses them for detection, status, and dispatch.
package backend

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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
	// PriorityJj is the detection priority for the jj backend.
	PriorityJj = 20
	// PriorityGit is the detection priority for the git backend.
	PriorityGit = 10

	// Severity ranks for RefState.
	rankDiverged = 4
	rankBehind   = 3
	rankAhead    = 2
	rankNoRemote = 1
	rankSynced   = 0
)

// Severity returns the relative severity of the state (higher is worse).
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
	// Name is the local bookmark/branch name.
	Name string

	// Remote is the remote name (e.g. "origin"). Empty when no remote.
	Remote string

	// Ahead is the number of local commits not present on the remote.
	Ahead int

	// Behind is the number of remote commits not present locally.
	Behind int

	// Conflict is true when jj reports a bookmark conflict (diverged push).
	// For git this is always false; git represents this as a diverged state.
	Conflict bool

	// State is the computed sync state derived from Ahead/Behind/Remote.
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

	// Dirty is true when there are uncommitted changes in the working copy.
	Dirty bool

	// Conflict is true when the repo itself has unresolved VCS conflicts
	// (jj conflict markers, git merge conflicts).
	Conflict bool

	// CommitMsg is the last commit/change message.
	CommitMsg string

	// CommitTime is the relative commit time (e.g. "3 days ago").
	CommitTime string
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

	// Run executes the given args using this VCS tool in path.
	// When interactive is true, the caller has already arranged for the
	// subprocess to inherit the terminal; Run must not capture output.
	Run(ctx context.Context, path string, args []string, interactive bool) (RunResult, error)
}

// Detection priority follows registration order.
//
//nolint:gochecknoglobals // intentional: plugin registry
var registry []Backend

// Register adds a backend to the global registry.
// Detection priority follows Backend.Priority() order (higher first).
// It panics on duplicate names to catch wiring mistakes at startup.
func Register(backend Backend) {
	for _, existing := range registry {
		if existing.Name() == backend.Name() {
			panic(fmt.Sprintf("backend %q already registered", backend.Name()))
		}
	}

	// Insert in priority order (descending)
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
}

// All returns a copy of all registered backends in priority order.
func All() []Backend {
	out := make([]Backend, len(registry))
	copy(out, registry)

	return out
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

// Detect returns the highest-priority backend that claims the directory.
// Priority follows DetectAll semantics — jj wins over git for colocated repos.
//
//nolint:ireturn // returning interface is intentional for plugin architecture
func Detect(path string) (Backend, error) {
	matched, err := DetectAll(path)
	if err != nil {
		return nil, err
	}

	return matched[0], nil
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
