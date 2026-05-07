package ui_test

import (
	"io"
	"os"
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Ref: testMainBookmark,
		Bookmarks: []backend.BookmarkStatus{
			{Name: testMainBookmark, State: backend.RefStateSynced},
		},
		Dirty:    true,
		Conflict: true,
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
		{
			"ahead",
			backend.BookmarkStatus{Name: "main", State: backend.RefStateAhead, Ahead: 2},
			"main ↑2",
		},
		{
			"behind",
			backend.BookmarkStatus{Name: "main", State: backend.RefStateBehind, Behind: 3},
			"main ↓3",
		},
		{
			"diverged",
			backend.BookmarkStatus{
				Name:   "main",
				State:  backend.RefStateDiverged,
				Ahead:  2,
				Behind: 1,
			},
			"main ↑2↓1",
		},
		{
			"diverged ahead only",
			backend.BookmarkStatus{
				Name:   "main",
				State:  backend.RefStateDiverged,
				Ahead:  1,
				Behind: 0,
			},
			"main ↑1",
		},
		{
			"diverged behind only",
			backend.BookmarkStatus{
				Name:   "main",
				State:  backend.RefStateDiverged,
				Ahead:  0,
				Behind: 1,
			},
			"main↓1",
		},
		{"gone", backend.BookmarkStatus{Name: "main", State: backend.RefStateGone}, "main ✗"},
		{
			"conflict",
			backend.BookmarkStatus{Name: "main", State: backend.RefStateSynced, Conflict: true},
			"main !",
		},
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

func TestColorSprint(t *testing.T) {
	out := ui.ColorSprint(text.Colors{text.FgGreen}, "hello")
	assert.Contains(t, out, "hello")
}

func TestTableStyle(t *testing.T) {
	s := ui.TableStyle()
	assert.NotNil(t, s)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hel", ui.Truncate("hello world", 3))
	assert.Equal(t, "hello world", ui.Truncate("hello world", 99))
}

func TestWrap(t *testing.T) {
	wrapped := ui.Wrap("hello world", 5)
	assert.Contains(t, wrapped, "hello")
}

func TestComputeRemainderWidth(t *testing.T) {
	// termWidth=100, minWidth=10, separators=1, used=[20, 30]
	// total used = 50, separator width = 2 * 1 = 2
	// remainder = 100 - 50 - 2 = 48
	w := ui.ComputeRemainderWidth(100, 10, 1, 20, 30)
	assert.Equal(t, 48, w)

	// When remainder < min, should return min
	w2 := ui.ComputeRemainderWidth(10, 20, 1, 20, 30)
	assert.Equal(t, 20, w2)
}

func TestNewTable(t *testing.T) {
	tbl := ui.NewTable()
	assert.NotNil(t, tbl)
}

func TestGetTermWidth(t *testing.T) {
	w := ui.GetTermWidth()
	assert.Positive(t, w)
}

func TestBadgeStyleDefault(t *testing.T) {
	// default case (unknown state, no conflict)
	out := ui.RenderBookmarkBadge(backend.BookmarkStatus{
		Name: "main", State: backend.RefState(99),
	})
	assert.Contains(t, out, "main")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w

	fn()

	_ = w.Close()

	os.Stdout = old

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	return string(out)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stderr = w

	fn()

	_ = w.Close()

	os.Stderr = old

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	return string(out)
}

func TestOutf(t *testing.T) {
	out := captureStdout(t, func() {
		ui.Outf("test %d", 42)
	})
	assert.Contains(t, out, "test 42")
}

func TestErrf(t *testing.T) {
	out := captureStderr(t, func() {
		ui.Errf("error %s", "msg")
	})
	assert.Contains(t, out, "error msg")
}

func TestSuccess(t *testing.T) {
	out := captureStderr(t, func() {
		ui.Success("done %s", "ok")
	})
	assert.Contains(t, out, "done ok")
}

func TestWarn(t *testing.T) {
	out := captureStderr(t, func() {
		ui.Warn("warning %d", 1)
	})
	assert.Contains(t, out, "warning 1")
}

func TestInfo(t *testing.T) {
	out := captureStderr(t, func() {
		ui.Info("info %s", "test")
	})
	assert.Contains(t, out, "info test")
}

func TestFail(t *testing.T) {
	out := captureStderr(t, func() {
		ui.Fail("fail %s", "err")
	})
	assert.Contains(t, out, "fail err")
}
