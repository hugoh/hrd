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
	RefStateUnknown  RefState = iota
	RefStateSynced            // local == remote
	RefStateAhead             // local ahead of remote
	RefStateBehind            // local behind remote
	RefStateDiverged          // local and remote have diverged
	RefStateNoRemote          // no remote configured
	RefStateGone              // remote ref deleted; local bookmark orphaned
)

func (s RefState) String() string {
	switch s {
	case RefStateSynced:
		return "synced"
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
		return "unknown"
	default:
		return "unknown"
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

	const (
		rankSynced   = 0
		rankNoRemote = 1
		rankAhead    = 2
		rankBehind   = 3
		rankDiverged = 4
		rankGone     = 4
		rankUnknown  = 0
	)

	rank := map[RefState]int{
		RefStateSynced:   rankSynced,
		RefStateNoRemote: rankNoRemote,
		RefStateAhead:    rankAhead,
		RefStateBehind:   rankBehind,
		RefStateDiverged: rankDiverged,
		RefStateGone:     rankGone,
		RefStateUnknown:  rankUnknown,
	}

	var best RefState

	if len(bookmarks) == 0 {
		return RefStateNoRemote
	}

	best = bookmarks[0].State
	for _, bm := range bookmarks[1:] {
		if rank[bm.State] > rank[best] {
			best = bm.State
		}
	}

	return best
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

	// Detect returns true if the directory at path is managed by this VCS.
	// It must not return an error for non-matching dirs; only real I/O
	// failures warrant an error.
	Detect(path string) (bool, error)

	// Status returns the unified status for the repo at path.
	Status(ctx context.Context, path string) (RepoStatus, error)

	// Run executes the given args using this VCS tool in path.
	// When interactive is true, the caller has already arranged for the
	// subprocess to inherit the terminal; Run must not capture output.
	Run(ctx context.Context, path string, args []string, interactive bool) (RunResult, error)
}

// registry holds all registered backends in registration order.
// Detection priority follows registration order.
//
//nolint:gochecknoglobals // intentional: plugin registry
var registry []Backend

// Register adds a backend to the global registry.
// It panics on duplicate names to catch wiring mistakes at startup.
func Register(backend Backend) {
	for _, existing := range registry {
		if existing.Name() == backend.Name() {
			panic(fmt.Sprintf("backend %q already registered", backend.Name()))
		}
	}

	registry = append(registry, backend)
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
	for _, b := range registry {
		if b.Name() == name {
			return b, nil
		}
	}

	return nil, fmt.Errorf("unknown backend %q: %w", name, errUnknownBackend)
}

var errUnknownBackend = errors.New("unknown backend")

// Detect walks the registry in order and returns the first backend that
// claims the directory.
//
//nolint:ireturn // returning interface is intentional for plugin architecture
func Detect(path string) (Backend, error) {
	abs, err := filepath.Abs(path)
	if err != nil { // coverage-ignore — only fails on nil, caller controls input
		return nil, fmt.Errorf("resolving path %q: %w", path, err)
	}

	for _, backend := range registry {
		ok, err := backend.Detect(abs)
		if err != nil {
			return nil, fmt.Errorf("backend %q detect: %w", backend.Name(), err)
		}

		if ok {
			return backend, nil
		}
	}

	return nil, fmt.Errorf("no known VCS detected at %q: %w", abs, errNoKnownVCS)
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

	for _, b := range registry {
		ok, err := b.Detect(abs)
		if err != nil {
			return nil, fmt.Errorf("backend %q detect: %w", b.Name(), err)
		}

		if ok {
			matched = append(matched, b)
		}
	}

	if len(matched) == 0 {
		return nil, fmt.Errorf("no known VCS detected at %q: %w", abs, errNoKnownVCS)
	}

	if len(matched) > 1 {
		for i, b := range matched {
			if b.Name() == "jj" {
				matched[0], matched[i] = matched[i], matched[0]

				break
			}
		}
	}

	return matched, nil
}
