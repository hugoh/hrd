package ui_test

import (
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/zenizh/go-capturer"
)

func TestRenderDispatchResult_Success(t *testing.T) {
	res := runner.Result{RepoName: "tmhi-cli", VCS: "git", ExitCode: 0}
	out := ui.RenderDispatchResult(res)
	assert.Contains(t, out, "tmhi-cli")
	assert.Contains(t, out, "git")
	assert.Contains(t, out, "✓")
}

func TestRenderDispatchResult_Error(t *testing.T) {
	res := runner.Result{RepoName: "bad", VCS: "jj", Err: assert.AnError}
	out := ui.RenderDispatchResult(res)
	assert.Contains(t, out, "bad")
	assert.Contains(t, out, "jj")
	assert.Contains(t, out, "✗")
}

func TestRenderDispatchResult_NonZeroExit(t *testing.T) {
	res := runner.Result{RepoName: "fail", VCS: "git", ExitCode: 1}
	out := ui.RenderDispatchResult(res)
	assert.Contains(t, out, "fail")
	assert.Contains(t, out, "git")
	assert.Contains(t, out, "✗")
	assert.Contains(t, out, "exit 1")
}

func TestRenderDispatchResult_EmptyVCS(t *testing.T) {
	res := runner.Result{RepoName: "bare", ExitCode: 0}
	out := ui.RenderDispatchResult(res)
	assert.Contains(t, out, "bare")
	assert.NotContains(t, out, "git")
	assert.NotContains(t, out, "jj")
}

func TestFormatDispatchHeader(t *testing.T) {
	t.Run("with name and vcs", func(t *testing.T) {
		got := ui.FormatDispatchHeader("my-repo", "git")
		assert.Equal(t, " my-repo         git", got)
	})

	t.Run("empty vcs", func(t *testing.T) {
		got := ui.FormatDispatchHeader("my-repo", "")
		assert.Equal(t, " my-repo            ", got)
	})

	t.Run("empty name", func(t *testing.T) {
		got := ui.FormatDispatchHeader("", "jj")
		assert.Equal(t, "                 jj ", got)
	})

	t.Run("long name exceeding field width", func(t *testing.T) {
		got := ui.FormatDispatchHeader("very-long-repo-name", "jj")
		assert.Equal(t, " very-long-repo-name jj ", got)
	})
}

func TestColorSprint(t *testing.T) {
	out := ui.ColorSprint("green", "hello")
	assert.Contains(t, out, "hello")
}

func TestComputeRemainderWidth(t *testing.T) {
	// termWidth=100, minWidth=10, used=[20, 30] (2 fixed widths)
	// total used = 50, numSeparators = 2, separatorWidth = 2
	// remainder = 100 - 50 - 2*2 = 46
	w := ui.ComputeRemainderWidth(100, 10, 20, 30)
	assert.Equal(t, 46, w)

	// When remainder < min, should return min
	w2 := ui.ComputeRemainderWidth(10, 20, 20, 30)
	assert.Equal(t, 20, w2)
}

func TestGetTermWidth(t *testing.T) {
	w := ui.GetTermWidth()
	assert.Positive(t, w)
}

func TestOutf(t *testing.T) {
	out := capturer.CaptureStdout(func() {
		ui.Outf("test %d", 42)
	})
	assert.Contains(t, out, "test 42")
}

func TestErrf(t *testing.T) {
	out := capturer.CaptureStderr(func() {
		ui.Errf("error %s", "msg")
	})
	assert.Contains(t, out, "error msg")
}

func TestSuccess(t *testing.T) {
	out := capturer.CaptureStderr(func() {
		ui.Success("done %s", "ok")
	})
	assert.Contains(t, out, "done ok")
}

func TestWarn(t *testing.T) {
	out := capturer.CaptureStderr(func() {
		ui.Warn("warning %d", 1)
	})
	assert.Contains(t, out, "warning 1")
}

func TestFail(t *testing.T) {
	out := capturer.CaptureStderr(func() {
		ui.Fail("fail %s", "err")
	})
	assert.Contains(t, out, "fail err")
}

func TestApplyColor(t *testing.T) {
	tests := []struct {
		name  string
		color string
		input string
	}{
		{"green", "green", "✓"},
		{"red", "red", "✗"},
		{"unknown", "unknown", "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ui.ApplyColor(tt.color, tt.input)
			assert.Contains(t, result, tt.input)
		})
	}
}

func TestFormatDispatchStatusLine(t *testing.T) {
	t.Run("synced bookmark", func(t *testing.T) {
		status := backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
			OverallState: backend.RefStateSynced,
		}
		line := ui.FormatDispatchStatusLine(status, false)
		assert.Contains(t, line, "main")
		assert.Contains(t, line, "✓")
	})

	t.Run("with details", func(t *testing.T) {
		status := backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
			OverallState: backend.RefStateSynced,
			CommitMsg:    "fix bug",
			CommitTime:   "2h ago",
		}
		line := ui.FormatDispatchStatusLine(status, true)
		assert.Contains(t, line, "main")
		assert.Contains(t, line, "fix bug")
		assert.Contains(t, line, "2h ago")
	})

	t.Run("no ref yields empty", func(t *testing.T) {
		status := backend.RepoStatus{}
		line := ui.FormatDispatchStatusLine(status, false)
		assert.Empty(t, line)
	})

	t.Run("dirty marker", func(t *testing.T) {
		status := backend.RepoStatus{
			Ref:          "main",
			Bookmarks:    []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
			OverallState: backend.RefStateSynced,
			Dirty:        true,
		}
		line := ui.FormatDispatchStatusLine(status, false)
		assert.Contains(t, line, "*")
	})
}

func TestFormatStatusLine(t *testing.T) {
	t.Run("with bookmark", func(t *testing.T) {
		status := backend.RepoStatus{
			Ref:       "main",
			Bookmarks: []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
		}
		parts := ui.FormatStatusLine(status, "✓", "")
		assert.Equal(t, "main", parts.Ref)
		assert.Equal(t, "✓", parts.Symbols)
		assert.True(t, parts.HasRef)
	})

	t.Run("no bookmark uses ref", func(t *testing.T) {
		status := backend.RepoStatus{Ref: "abc123"}
		parts := ui.FormatStatusLine(status, "", "")
		assert.Equal(t, "abc123", parts.Ref)
		assert.True(t, parts.HasRef)
	})

	t.Run("empty status", func(t *testing.T) {
		status := backend.RepoStatus{}
		parts := ui.FormatStatusLine(status, "sym", "detail")
		assert.Empty(t, parts.Ref)
		assert.False(t, parts.HasRef)
	})
}

func TestFormatSummary(t *testing.T) {
	t.Run("all succeeded", func(t *testing.T) {
		s := ui.FormatSummary(5, nil)
		assert.Equal(t, "5/5 repos completed successfully", s)
	})

	t.Run("some failed", func(t *testing.T) {
		s := ui.FormatSummary(5, []string{"repo1", "repo2"})
		assert.Equal(t, "3/5 repos completed successfully; failed: repo1, repo2", s)
	})
}

func TestFormatDetail(t *testing.T) {
	t.Run("both msg and time", func(t *testing.T) {
		s := ui.FormatDetail("fix bug", "2h ago")
		assert.Equal(t, "fix bug 2h ago", s)
	})

	t.Run("msg only", func(t *testing.T) {
		s := ui.FormatDetail("fix bug", "")
		assert.Equal(t, "fix bug", s)
	})

	t.Run("time only", func(t *testing.T) {
		s := ui.FormatDetail("", "2h ago")
		assert.Equal(t, "2h ago", s)
	})

	t.Run("both empty", func(t *testing.T) {
		s := ui.FormatDetail("", "")
		assert.Empty(t, s)
	})
}

func TestStateStyle(t *testing.T) {
	style := ui.StateStyle(backend.RefStateSynced)
	assert.NotEmpty(t, style.Render("test"))

	style = ui.StateStyle(backend.RefState(999))
	assert.NotEmpty(t, style.Render("test"))
}

func TestMutedStyle(t *testing.T) {
	style := ui.MutedStyle()
	assert.NotEmpty(t, style.Render("test"))

	assert.NotEmpty(t, ui.Muted("test"))
}
