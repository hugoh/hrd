package ui_test

import (
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/stretchr/testify/assert"
)

const testMainBookmark = "main"

func TestRenderStatusLine_JJ(t *testing.T) {
	st := backend.RepoStatus{
		Ref: "abc123def",
		Bookmarks: []backend.BookmarkStatus{
			{Name: testMainBookmark, State: backend.RefStateSynced},
			{Name: "feat", State: backend.RefStateAhead, Ahead: 2},
		},
	}

	out := ui.RenderStatusLine("dotfiles", "jj", st)
	assert.Contains(t, out, "dotfiles")
	assert.Contains(t, out, "jj")
	assert.Contains(t, out, "abc123def")
	assert.Contains(t, out, testMainBookmark)
	assert.Contains(t, out, "feat")
}

func TestRenderStatusLine_Git(t *testing.T) {
	st := backend.RepoStatus{
		Ref: testMainBookmark,
		Bookmarks: []backend.BookmarkStatus{
			{Name: testMainBookmark, State: backend.RefStateSynced},
		},
	}

	out := ui.RenderStatusLine("hrd", "git", st)
	assert.Contains(t, out, "hrd")
	assert.Contains(t, out, "git")
	assert.Contains(t, out, testMainBookmark)
}

func TestRenderStatusLine_DirtyAndConflict(t *testing.T) {
	st := backend.RepoStatus{
		Ref:       testMainBookmark,
		Bookmarks: []backend.BookmarkStatus{{Name: testMainBookmark, State: backend.RefStateSynced}},
		Dirty:     true,
		Conflict:  true,
	}

	out := ui.RenderStatusLine("repo", "git", st)
	assert.Contains(t, out, "repo")
	assert.Contains(t, out, "*")
}

func TestRenderStatusLine_NoBookmarks(t *testing.T) {
	st := backend.RepoStatus{
		Ref:       testMainBookmark,
		Bookmarks: []backend.BookmarkStatus{},
	}

	out := ui.RenderStatusLine("repo", "git", st)
	assert.Contains(t, out, "repo")
	assert.Contains(t, out, "(no bookmarks)")
}

func TestRenderDispatchResult_Success(t *testing.T) {
	res := runner.Result{RepoName: "tmhi-cli", ExitCode: 0}
	out := ui.RenderDispatchResult(res)
	assert.Contains(t, out, "tmhi-cli")
	assert.Contains(t, out, "✓")
}

func TestRenderDispatchResult_Error(t *testing.T) {
	res := runner.Result{RepoName: "bad", Err: assert.AnError}
	out := ui.RenderDispatchResult(res)
	assert.Contains(t, out, "bad")
	assert.Contains(t, out, "✗")
}

func TestRenderDispatchResult_NonZeroExit(t *testing.T) {
	res := runner.Result{RepoName: "fail", ExitCode: 1}
	out := ui.RenderDispatchResult(res)
	assert.Contains(t, out, "fail")
	assert.Contains(t, out, "✗")
	assert.Contains(t, out, "exit 1")
}

func TestRenderBookmarkBadge(t *testing.T) {
	assert.Contains(t, ui.RenderBookmarkBadge(backend.BookmarkStatus{
		Name:  testMainBookmark,
		State: backend.RefStateSynced,
	}), testMainBookmark)

	assert.Contains(t, ui.RenderBookmarkBadge(backend.BookmarkStatus{
		Name:  "feat",
		State: backend.RefStateAhead,
		Ahead: 3,
	}), "↑3")
}

func TestRenderBookmarkBadge_AllStates(t *testing.T) {
	tests := []struct {
		name       string
		bm         backend.BookmarkStatus
		wantSubstr string
	}{
		{"synced", backend.BookmarkStatus{Name: "main", State: backend.RefStateSynced}, "main ✓"},
		{"ahead", backend.BookmarkStatus{Name: "main", State: backend.RefStateAhead, Ahead: 2}, "main ↑2"},
		{"behind", backend.BookmarkStatus{Name: "main", State: backend.RefStateBehind, Behind: 3}, "main ↓3"},
		{"diverged", backend.BookmarkStatus{Name: "main", State: backend.RefStateDiverged, Ahead: 2, Behind: 1}, "main ↑2↓1"},
		{"diverged ahead only",
			backend.BookmarkStatus{Name: "main", State: backend.RefStateDiverged, Ahead: 1, Behind: 0},
			"main ↑1"},
		{"diverged behind only",
			backend.BookmarkStatus{Name: "main", State: backend.RefStateDiverged, Ahead: 0, Behind: 1},
			"main↓1"},
		{"gone", backend.BookmarkStatus{Name: "main", State: backend.RefStateGone}, "main ✗"},
		{"conflict", backend.BookmarkStatus{Name: "main", State: backend.RefStateSynced, Conflict: true}, "main !"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := ui.RenderBookmarkBadge(tt.bm)
			assert.Contains(t, out, tt.wantSubstr)
		})
	}
}

func TestRenderFlags_AllCombinations(t *testing.T) {
	tests := []struct {
		name string
		st   backend.RepoStatus
		want string
	}{
		{
			name: "clean",
			st:   backend.RepoStatus{Dirty: false, Conflict: false},
			want: "  ",
		},
		{
			name: "dirty only",
			st:   backend.RepoStatus{Dirty: true, Conflict: false},
			want: "*",
		},
		{
			name: "conflict only",
			st:   backend.RepoStatus{Dirty: false, Conflict: true},
			want: "‼",
		},
		{
			name: "dirty and conflict",
			st:   backend.RepoStatus{Dirty: true, Conflict: true},
			want: "*‼",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := ui.RenderFlags(tt.st)
			assert.Contains(t, out, tt.want)
		})
	}
}

func TestRenderVCS(t *testing.T) {
	assert.Contains(t, ui.RenderVCS("jj"), "jj")
	assert.Contains(t, ui.RenderVCS("git"), "git")
	assert.Contains(t, ui.RenderVCS("unknown"), "git")
}

func TestBadgeStyle_States(t *testing.T) {
	states := []backend.RefState{
		backend.RefStateSynced,
		backend.RefStateAhead,
		backend.RefStateBehind,
		backend.RefStateDiverged,
		backend.RefStateGone,
		backend.RefStateUnknown,
		backend.RefStateNoRemote,
	}

	for _, state := range states {
		t.Run(state.String(), func(t *testing.T) {
			bm := backend.BookmarkStatus{Name: "main", State: state}
			out := ui.RenderBookmarkBadge(bm)
			assert.Contains(t, out, "main")
		})
	}
}

func TestRenderStatusLine_WithError(t *testing.T) {
	st := backend.RepoStatus{
		Ref:       testMainBookmark,
		Bookmarks: []backend.BookmarkStatus{},
	}

	out := ui.RenderStatusLine("error-repo", "git", st)
	assert.Contains(t, out, "error-repo")
}

func TestRenderStatusLine_DivergedWithZeroAheadBehind(t *testing.T) {
	st := backend.RepoStatus{
		Ref: testMainBookmark,
		Bookmarks: []backend.BookmarkStatus{
			{Name: testMainBookmark, State: backend.RefStateDiverged, Ahead: 0, Behind: 0},
		},
	}

	out := ui.RenderStatusLine("repo", "git", st)
	assert.Contains(t, out, "repo")
}
