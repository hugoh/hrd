package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefState_String(t *testing.T) {
	tests := []struct {
		state RefState
		want  string
	}{
		{RefStateSynced, "synced"},
		{RefStateAhead, "ahead"},
		{RefStateBehind, "behind"},
		{RefStateDiverged, "diverged"},
		{RefStateNoRemote, "local"},
		{RefStateGone, "gone"},
		{RefStateUnknown, "unknown"},
		{RefState(999), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.String())
		})
	}
}

func TestComputeBookmarkState(t *testing.T) {
	tests := []struct {
		name     string
		bookmark BookmarkStatus
		want     RefState
	}{
		{"no remote", BookmarkStatus{Remote: ""}, RefStateNoRemote},
		{"conflict overrides", BookmarkStatus{Conflict: true, Remote: "origin"}, RefStateDiverged},
		{
			"ahead and behind",
			BookmarkStatus{Remote: "origin", Ahead: 2, Behind: 1},
			RefStateDiverged,
		},
		{"ahead only", BookmarkStatus{Remote: "origin", Ahead: 3, Behind: 0}, RefStateAhead},
		{"behind only", BookmarkStatus{Remote: "origin", Ahead: 0, Behind: 5}, RefStateBehind},
		{"synced", BookmarkStatus{Remote: "origin", Ahead: 0, Behind: 0}, RefStateSynced},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := tt.bookmark
			ComputeBookmarkState(&bm)
			assert.Equal(t, tt.want, bm.State)
		})
	}
}

func TestRepoStatus_NeedsAttention(t *testing.T) {
	tests := []struct {
		name string
		st   RepoStatus
		want bool
	}{
		{"dirty", RepoStatus{Dirty: true}, true},
		{"local ahead (jj working copy)", RepoStatus{LocalAhead: 1}, true},
		{"trunk ahead (unmerged commits)", RepoStatus{TrunkAhead: 1}, true},
		{"not on trunk (zero ahead)", RepoStatus{NotOnTrunk: true}, true},
		{
			"bookmark ahead",
			RepoStatus{Bookmarks: []BookmarkStatus{{Name: "main", Ahead: 1}}},
			true,
		},
		{
			"bookmark behind",
			RepoStatus{Bookmarks: []BookmarkStatus{{Name: "main", Behind: 1}}},
			true,
		},
		{
			"diverged (ahead and behind)",
			RepoStatus{Bookmarks: []BookmarkStatus{{Name: "main", Ahead: 2, Behind: 1}}},
			true,
		},
		{
			"no remote, clean",
			RepoStatus{Bookmarks: []BookmarkStatus{{Name: "main", Remote: ""}}},
			false,
		},
		{
			"clean and synced",
			RepoStatus{Bookmarks: []BookmarkStatus{{Name: "main", Remote: "origin"}}},
			false,
		},
		{"zero value", RepoStatus{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.st.NeedsAttention())
		})
	}
}

func TestWorstState(t *testing.T) {
	tests := []struct {
		name        string
		bookmarks   []BookmarkStatus
		hasConflict bool
		want        RefState
	}{
		{"empty bookmarks", nil, false, RefStateNoRemote},
		{"single synced", []BookmarkStatus{{State: RefStateSynced}}, false, RefStateSynced},
		{
			"conflict overrides all",
			[]BookmarkStatus{{State: RefStateSynced}},
			true,
			RefStateDiverged,
		},
		{
			"multiple picks worst",
			[]BookmarkStatus{
				{State: RefStateSynced},
				{State: RefStateAhead},
				{State: RefStateBehind},
			},
			false,
			RefStateBehind,
		},
		{
			"diverged is worst",
			[]BookmarkStatus{{State: RefStateSynced}, {State: RefStateDiverged}},
			false,
			RefStateDiverged,
		},
		{
			"gone equals diverged rank",
			[]BookmarkStatus{{State: RefStateGone}, {State: RefStateSynced}},
			false,
			RefStateGone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorstState(tt.bookmarks, tt.hasConflict)
			assert.Equal(t, tt.want, got)
		})
	}
}

type mockBackend struct {
	name     string
	priority int
	detect   func(path string) (bool, error)
}

func (m *mockBackend) Name() string                                  { return m.name }
func (m *mockBackend) Priority() int                                 { return m.priority }
func (m *mockBackend) Detect(path string) (bool, error)              { return m.detect(path) }
func (*mockBackend) SubcommandArgs(op string) []string               { return []string{op} }
func (*mockBackend) Subcommands(_ context.Context) ([]string, error) { return nil, nil }

// withCleanRegistry resets the global registry to a clean state and restores it
// on test cleanup. Registers the given backends on the clean registry.
func withCleanRegistry(t *testing.T, backends ...*mockBackend) {
	t.Helper()

	orig := registry
	registry = nil

	for _, b := range backends {
		require.NoError(t, Register(b))
	}

	t.Cleanup(func() { registry = orig })
}

// withMatchingGitDir registers a "git" backend that matches only a fresh
// temp dir, plus a "jj" backend that never matches, and returns that dir —
// used by Detect/DetectAll tests asserting a single-backend match.
func withMatchingGitDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	withCleanRegistry(t,
		&mockBackend{
			name:   "git",
			detect: func(path string) (bool, error) { return path == dir, nil },
		},
		&mockBackend{name: "jj", detect: func(_ string) (bool, error) { return false, nil }},
	)

	return dir
}

func (*mockBackend) Status(_ context.Context, _ string) (RepoStatus, error) {
	return RepoStatus{}, nil
}

func (*mockBackend) Run(
	_ context.Context,
	_ string,
	_ []string,
	_ bool,
) (RunResult, error) {
	return RunResult{}, nil
}

func TestRegister(t *testing.T) {
	withCleanRegistry(t)

	b1 := &mockBackend{name: "low", priority: 10}
	require.NoError(t, Register(b1))
	assert.Len(t, registry, 1)
	assert.Equal(t, "low", registry[0].Name())

	b2 := &mockBackend{name: "high", priority: 20}
	require.NoError(t, Register(b2))
	assert.Len(t, registry, 2)
	assert.Equal(t, "high", registry[0].Name())
	assert.Equal(t, "low", registry[1].Name())

	b3 := &mockBackend{name: "mid", priority: 15}
	require.NoError(t, Register(b3))
	assert.Len(t, registry, 3)
	assert.Equal(t, "high", registry[0].Name())
	assert.Equal(t, "mid", registry[1].Name())
	assert.Equal(t, "low", registry[2].Name())

	assert.Error(t, Register(&mockBackend{name: "low"}))
}

func TestByName(t *testing.T) {
	withCleanRegistry(t)

	require.NoError(t, Register(&mockBackend{name: "git"}))
	require.NoError(t, Register(&mockBackend{name: "jj"}))

	b, err := ByName("git")
	require.NoError(t, err)
	assert.Equal(t, "git", b.Name())

	_, err = ByName("svn")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown backend")
}

func TestDetectAll_NoMatch(t *testing.T) {
	withCleanRegistry(t,
		&mockBackend{name: "git", detect: func(_ string) (bool, error) { return false, nil }},
		&mockBackend{name: "jj", detect: func(_ string) (bool, error) { return false, nil }},
	)

	_, err := DetectAll("/nonexistent")
	assert.Error(t, err)
}

func TestDetect_Match(t *testing.T) {
	dir := withMatchingGitDir(t)

	b, err := Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, "git", b.Name())
}

func TestDetect_NoMatch(t *testing.T) {
	withCleanRegistry(t,
		&mockBackend{name: "git", detect: func(_ string) (bool, error) { return false, nil }},
	)

	_, err := Detect("/nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no known VCS")
}

func TestDetect_ErrorFromDetect(t *testing.T) {
	withCleanRegistry(t,
		&mockBackend{
			name:   "git",
			detect: func(_ string) (bool, error) { return false, assert.AnError },
		},
	)

	_, err := Detect("/tmp")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "detect")
}

func TestDetectAll_Match(t *testing.T) {
	dir := withMatchingGitDir(t)

	matched, err := DetectAll(dir)
	require.NoError(t, err)
	assert.Len(t, matched, 1)
	assert.Equal(t, "git", matched[0].Name())
}

func TestDetectAll_MultipleMatches_JjFirst(t *testing.T) {
	dir := t.TempDir()

	withCleanRegistry(t,
		&mockBackend{
			name:     "git",
			priority: 10,
			detect:   func(path string) (bool, error) { return path == dir, nil },
		},
		&mockBackend{
			name:     "jj",
			priority: 20,
			detect:   func(path string) (bool, error) { return path == dir, nil },
		},
	)

	matched, err := DetectAll(dir)
	require.NoError(t, err)
	assert.Len(t, matched, 2)
	assert.Equal(t, "jj", matched[0].Name())
}

func TestDetectAll_ErrorFromDetect(t *testing.T) {
	withCleanRegistry(t,
		&mockBackend{
			name:   "git",
			detect: func(_ string) (bool, error) { return false, assert.AnError },
		},
	)

	_, err := DetectAll("/tmp")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "detect")
}

func TestDetect_PathResolution(t *testing.T) {
	withCleanRegistry(t,
		&mockBackend{name: "git", detect: func(_ string) (bool, error) { return false, nil }},
	)

	_, err := Detect("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no known VCS")
}

func TestDetectAll_PathResolution(t *testing.T) {
	withCleanRegistry(t,
		&mockBackend{name: "git", detect: func(_ string) (bool, error) { return false, nil }},
	)

	_, err := DetectAll("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no known VCS")
}

func TestDetectWithRealGitRepo(t *testing.T) {
	withCleanRegistry(t)

	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".git"), 0o750)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[core]\n"), 0o644)
	require.NoError(t, err)

	var gitDetected bool

	err = Register(&mockBackend{name: "git", detect: func(path string) (bool, error) {
		gitDetected = true

		return path == dir, nil
	}})
	require.NoError(t, err)

	b, err := Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, "git", b.Name())
	assert.True(t, gitDetected)
}

func TestBookmarkStatusDefaults(t *testing.T) {
	bm := BookmarkStatus{Name: "main"}
	assert.Equal(t, "main", bm.Name)
	assert.Empty(t, bm.Remote)
	assert.Equal(t, 0, bm.Ahead)
	assert.Equal(t, 0, bm.Behind)
	assert.False(t, bm.Conflict)
	assert.Equal(t, RefState(0), bm.State)
}

func TestRepoStatusDefaults(t *testing.T) {
	st := RepoStatus{}
	assert.Empty(t, st.Ref)
	assert.Nil(t, st.Bookmarks)
	assert.Equal(t, RefState(0), st.OverallState)
	assert.False(t, st.Dirty)
	assert.False(t, st.Conflict)
	assert.Empty(t, st.CommitMsg)
	assert.Empty(t, st.CommitTime)
}
