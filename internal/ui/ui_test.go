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
