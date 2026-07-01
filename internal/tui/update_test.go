package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/backends/jj"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	git.Register()
	jj.Register()
	goleak.VerifyTestMain(m,
		// teatest.NewTestModel spawns an output-forwarding goroutine that
		// outlives WaitFinished; it's internal to the third-party library,
		// not something callers can drain or cancel.
		goleak.IgnoreTopFunction("github.com/charmbracelet/x/exp/teatest/v2.NewTestModel.func2"),
	)
}

// initGitRepo initializes a real git repository in dir, isolated from the
// developer's global git config, and skips the test if git is unavailable.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)

	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.email", "test@test.com")
	runGitCmd(t, dir, "config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content\n"), 0o644))
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "initial commit")
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()

	c := exec.CommandContext(t.Context(), "git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, out)
}

func TestTableShowsRepoStatus(t *testing.T) {
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	err := os.WriteFile(cfgPath, []byte(
		`[repos.testrepo]
path = "`+repoDir+`"
backends = ["git"]
`), 0o644)
	require.NoError(t, err)

	ctx := t.Context()
	m, err := newTestModel(ctx, t, Options{
		ConfigPath: cfgPath,
	})
	require.NoError(t, err)

	m.selected["testrepo"] = true

	m.statuses["testrepo"] = runner.StatusResult{
		RepoName: "testrepo",
		Status:   backend.RepoStatus{Ref: "main", CommitMsg: "initial commit"},
	}
	m.updateTableRows()

	rows := m.repoTable.Rows()
	require.NotEmpty(t, rows, "table should have rows after updateTableRows")

	found := false

	for _, row := range rows {
		if strings.Contains(row[1], "testrepo") {
			found = true

			break
		}
	}

	require.True(t, found, "table should contain testrepo")
}

func TestHandleStatusUpdate(t *testing.T) {
	m := &model{
		repoOrder: []string{"alpha"},
		selected:  map[string]bool{"alpha": true},
		statuses:  make(map[string]runner.StatusResult),
	}
	m.initTable()
	m.updateTableRows()

	rows := m.repoTable.Rows()
	require.Len(t, rows, 1, "table should show repo even before status arrives")
	require.Contains(t, rows[0][3], "...", "status column should show placeholder before update")

	msg := statusUpdateMsg{
		result: runner.StatusResult{
			RepoName: "alpha",
			Status:   backend.RepoStatus{Ref: "main"},
		},
	}

	_, cmd := m.handleStatusUpdate(msg)
	assert.Nil(t, cmd, "handleStatusUpdate with no filtered repos should return nil cmd")

	rows = m.repoTable.Rows()
	require.Len(t, rows, 1)
	require.Contains(t, rows[0][1], "alpha", "name column should show repo name")
}

func TestHandleStatusUpdateUnknownRepo(t *testing.T) {
	m := &model{
		repoOrder: []string{},
		selected:  map[string]bool{},
		statuses:  make(map[string]runner.StatusResult),
	}
	m.initTable()

	msg := statusUpdateMsg{
		result: runner.StatusResult{
			RepoName: "ghost",
			Status:   backend.RepoStatus{Ref: "main"},
		},
	}

	_, cmd := m.handleStatusUpdate(msg)
	assert.Nil(t, cmd, "handleStatusUpdate with no filtered repos should return nil cmd")
}

func TestHandleStatusDone(t *testing.T) {
	m := &model{
		loading:   true,
		statusCh:  make(chan runner.StatusResult),
		repoOrder: []string{"alpha"},
		selected:  map[string]bool{"alpha": true},
	}
	m.initTable()

	_, cmd := m.handleStatusDone()
	assert.Nil(t, cmd, "handleStatusDone should return nil cmd")
	assert.False(t, m.loading, "loading should be false after handleStatusDone")
	assert.Nil(t, m.statusCh, "statusCh should be nil after handleStatusDone")
}

func TestRefColumnWidthAfterWindowSize(t *testing.T) {
	m, err := newTestModel(t.Context(), t, Options{})
	require.NoError(t, err)

	m.cfg = config.Config{
		Repos: map[string]config.Repo{
			"test": {Path: t.TempDir()},
		},
		Settings: config.Settings{Concurrency: 4},
	}
	m.repoOrder = []string{"test"}
	m.selected = map[string]bool{"test": true}

	m.width = 100
	m.height = 30
	m.ready = true
	m.repoTable.SetHeight(m.contentHeight())
	m.repoTable.SetColumns([]table.Column{
		{Title: "", Width: checkboxColW},
		{Title: colName, Width: maxNameWidth},
		{Title: colVCS, Width: listVCSWidth},
		{Title: colStatus, Width: 100 - checkboxColW - maxNameWidth - listVCSWidth - 6},
	})

	cols := m.repoTable.Columns()
	require.Len(t, cols, 4, "table should have 4 columns")
	require.Positive(t, cols[3].Width, "STATUS column width should be > 0 after WindowSizeMsg")
}

func TestRowAlignmentNoSelectMode(t *testing.T) {
	m, err := newTestModel(t.Context(), t, Options{})
	require.NoError(t, err)

	m.cfg = config.Config{
		Repos: map[string]config.Repo{
			"alpha": {Path: t.TempDir()},
			"beta":  {Path: t.TempDir()},
		},
		Settings: config.Settings{Concurrency: 4},
	}
	m.repoOrder = []string{"alpha", "beta"}

	m.statuses["alpha"] = runner.StatusResult{
		RepoName: "alpha",
		Status:   backend.RepoStatus{Ref: "main", CommitMsg: "test msg"},
	}
	m.statuses["beta"] = runner.StatusResult{
		RepoName: "beta",
		Status:   backend.RepoStatus{Ref: "dev", CommitMsg: "wip"},
	}

	t.Run("normal mode shows selected repos", func(t *testing.T) {
		m.mode = modeNormal
		m.selected = map[string]bool{"alpha": true, "beta": true}
		m.cursor = 0
		m.updateTableRows()

		rows := m.repoTable.Rows()
		require.Len(t, rows, 2)
		require.Empty(t, rows[0][0], "checkbox column should be empty in normal mode")
		require.Empty(t, rows[1][0], "checkbox column should be empty in normal mode")
		require.Equal(t, "alpha", rows[0][1], "name should have no prefix in normal mode")
		require.Equal(t, "beta", rows[1][1], "name should have no prefix in normal mode")
	})

	t.Run("empty table when no repos selected", func(t *testing.T) {
		m.mode = modeNormal
		m.selected = map[string]bool{}
		m.updateTableRows()

		rows := m.repoTable.Rows()
		require.Empty(t, rows, "table should be empty when no repos selected")
	})

	t.Run("select mode shows checkbox", func(t *testing.T) {
		m.mode = modeSelect
		m.selected = map[string]bool{"alpha": true, "beta": false}
		m.updateTableRows()

		rows := m.repoTable.Rows()
		require.Len(t, rows, 2)
		require.NotEmpty(t, rows[0][0], "checkbox column should show symbol in select mode")
		require.NotEmpty(t, rows[1][0], "checkbox column should show symbol in select mode")
		require.Equal(t, "alpha", rows[0][1], "name should still be plain (no prefix)")
		require.Equal(t, "beta", rows[1][1], "name should still be plain (no prefix)")
	})
}

func TestCursorRowNoWidthShift(t *testing.T) {
	ss := tableStyles(true)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)

	// Simulate 4-column row: checkbox + name + vcs + status
	cells := []string{
		cellStyle.Render(""),
		cellStyle.Render("alpha"),
		cellStyle.Render("git"),
		cellStyle.Render("main ✓"),
	}
	nonCursor := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	cursor := ss.Selected.Render(nonCursor)

	assert.Equal(t, lipgloss.Width(nonCursor), lipgloss.Width(cursor),
		"cursor row width differs from non-cursor")
}

func TestNameNotTruncatedWhenSelected(t *testing.T) {
	// A name that fits within maxNameWidth (24) must not be passed
	// with embedded ANSI codes, because runewidth.Truncate treats ANSI
	// escapes as printable chars, causing premature truncation → mangled output.
	m, err := newTestModel(t.Context(), t, Options{})
	require.NoError(t, err)

	m.cfg = config.Config{
		Repos: map[string]config.Repo{
			"AppBadgeWatcher.spoon": {Path: t.TempDir()},
		},
		Settings: config.Settings{Concurrency: 4},
	}
	m.repoOrder = []string{"AppBadgeWatcher.spoon"}
	m.selected = map[string]bool{"AppBadgeWatcher.spoon": true}

	m.updateTableRows()
	rows := m.repoTable.Rows()
	require.Len(t, rows, 1)
	require.Equal(t, "AppBadgeWatcher.spoon", rows[0][1],
		"name cell must be plain text, not wrapped in ANSI codes")
}

func TestCheckboxPlainText(t *testing.T) {
	// Checkbox values must be plain characters, not ANSI-styled strings,
	// because the table passes cell values through runewidth.Truncate
	// which counts ANSI escape bytes as printable characters.
	m, err := newTestModel(t.Context(), t, Options{})
	require.NoError(t, err)

	m.cfg = config.Config{
		Repos: map[string]config.Repo{
			"alpha": {Path: t.TempDir()},
			"beta":  {Path: t.TempDir()},
		},
		Settings: config.Settings{Concurrency: 4},
	}
	m.repoOrder = []string{"alpha", "beta"}
	m.selected = map[string]bool{"alpha": true, "beta": false}
	m.mode = modeSelect
	m.updateTableRows()

	rows := m.repoTable.Rows()
	require.Len(t, rows, 2)
	require.Equal(
		t,
		checkboxSelected,
		rows[0][0],
		"selected row checkbox must be exactly the checkmark constant",
	)
	require.Equal(
		t,
		checkboxUnsel,
		rows[1][0],
		"unselected row checkbox must be exactly the unselected constant",
	)

	// Also verify they are plain text (no ANSI escapes)
	require.NotContains(t, rows[0][0], "\x1b", "checkbox must not contain ANSI codes")
	require.NotContains(t, rows[1][0], "\x1b", "checkbox must not contain ANSI codes")
}

func TestColoredSummaryAllSuccess(t *testing.T) {
	m := &model{
		execTotal: 2,
		execResults: []execResult{
			{name: "repo-a", result: runner.Result{ExitCode: 0, Output: "ok"}},
			{name: "repo-b", result: runner.Result{ExitCode: 0, Output: "ok"}},
		},
	}
	got := m.coloredSummary()

	assert.Contains(
		t,
		got,
		"2/2 repos completed successfully",
		"coloredSummary should show success",
	)
}

func TestColoredSummaryWithFailures(t *testing.T) {
	errFoo := errors.New("foo failed")
	m := &model{
		execTotal: 3,
		execResults: []execResult{
			{name: "repo-a", result: runner.Result{ExitCode: 0, Output: "ok"}},
			{name: "repo-b", result: runner.Result{Err: errFoo}},
			{name: "repo-c", result: runner.Result{ExitCode: 1}},
		},
	}
	got := m.coloredSummary()

	assert.Contains(t, got, "failed:", "coloredSummary should show failures")
	assert.Contains(t, got, "repo-b", "coloredSummary should mention repo-b")
	assert.Contains(t, got, "repo-c", "coloredSummary should mention repo-c")
}

func TestFormatDispatchResultLineSuccess(t *testing.T) {
	res := runner.Result{ExitCode: 0, Output: "hello"}
	line := formatDispatchResultLine("myrepo", res, 40)

	assert.Contains(t, line, "myrepo", "output should contain repo name")
	assert.Contains(t, line, "hello", `output should contain 'hello'`)
}

func TestFormatDispatchResultLineError(t *testing.T) {
	res := runner.Result{
		Err:    errors.New("something broke"),
		Output: "details",
	}
	line := formatDispatchResultLine("myrepo", res, 40)

	assert.Contains(t, line, "error:", "output should contain 'error:'")
	assert.Contains(t, line, "something broke", "output should contain error msg")
}

func TestFormatDispatchResultLineNonZeroExit(t *testing.T) {
	res := runner.Result{ExitCode: 1, Output: "oops"}
	line := formatDispatchResultLine("myrepo", res, 40)

	assert.Contains(t, line, "exit 1", "output should contain 'exit 1'")
}

func TestFormatDispatchResultLineNoOutput(t *testing.T) {
	res := runner.Result{ExitCode: 0}
	line := formatDispatchResultLine("myrepo", res, 40)

	assert.Contains(t, line, "myrepo", "output should contain repo name")
}

func TestFormatExecOutput(t *testing.T) {
	results := []execResult{
		{name: "a", result: runner.Result{ExitCode: 0, Output: "ok"}},
		{name: "b", result: runner.Result{ExitCode: 1, Output: "fail"}},
	}
	got := formatExecOutput(results, 40)

	assert.Contains(t, got, "a", "output should contain 'a'")
	assert.Contains(t, got, "b", "output should contain 'b'")
}

func TestFormatExecOutputEmpty(t *testing.T) {
	got := formatExecOutput(nil, 40)
	assert.Empty(t, got, "expected empty output for nil results")
}

func TestFormatStatusLineNoStatus(t *testing.T) {
	m := &model{statuses: map[string]runner.StatusResult{}}

	line := m.formatStatusLine("nonexistent")
	assert.NotEmpty(t, line, "expected muted placeholder for missing status")
}

func TestFormatStatusLineError(t *testing.T) {
	m := &model{
		statuses: map[string]runner.StatusResult{
			"repo1": {
				Err: errors.New("something went wrong"),
			},
		},
	}
	line := m.formatStatusLine("repo1")

	assert.Contains(t, line, "something went wrong", "expected error message in output")
}

func TestFormatStatusLineWithBookmarkAndMsg(t *testing.T) {
	m := &model{
		statuses: map[string]runner.StatusResult{
			"repo1": {
				Status: backend.RepoStatus{
					Bookmarks: []backend.BookmarkStatus{
						{Name: "main", State: backend.RefStateSynced},
					},
					CommitMsg:  "fix bug",
					CommitTime: "2 hours ago",
				},
			},
		},
	}
	line := m.formatStatusLine("repo1")

	assert.Contains(t, line, "main", "expected bookmark name in output")
	assert.Contains(t, line, "fix bug", "expected commit msg in output")
}

func TestFormatStatusLineRefOnly(t *testing.T) {
	m := &model{
		statuses: map[string]runner.StatusResult{
			"repo1": {
				Status: backend.RepoStatus{
					Ref: "develop",
				},
			},
		},
	}
	line := m.formatStatusLine("repo1")

	assert.Contains(t, line, "develop", "expected ref in output")
}

func TestFormatStatusLineEmptyStatus(t *testing.T) {
	m := &model{
		statuses: map[string]runner.StatusResult{
			"repo1": {
				Status: backend.RepoStatus{},
			},
		},
	}
	line := m.formatStatusLine("repo1")

	assert.Empty(t, line, "expected empty output for empty status")
}

func TestRenderSymbols(t *testing.T) {
	status := backend.RepoStatus{
		Bookmarks: []backend.BookmarkStatus{
			{Name: "main", State: backend.RefStateSynced},
			{Name: "feature", State: backend.RefStateAhead, Ahead: 3},
			{Name: "unknown", State: backend.RefState(999)},
		},
	}
	syms := theme.FormatSymbols(status, testColorFn)

	assert.NotEmpty(t, syms, "expected non-empty symbols")
}

func TestRenderSymbolsDirty(t *testing.T) {
	status := backend.RepoStatus{
		Dirty:     true,
		Bookmarks: []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
	}
	syms := theme.FormatSymbols(status, testColorFn)

	assert.Contains(t, syms, "*", "expected dirty marker")
}

func TestRenderSymbolsConflict(t *testing.T) {
	status := backend.RepoStatus{
		Conflict: true,
		Bookmarks: []backend.BookmarkStatus{
			{Name: "main", State: backend.RefStateSynced, Conflict: true},
		},
	}
	syms := theme.FormatSymbols(status, testColorFn)

	assert.NotEmpty(t, syms, "expected non-empty symbols for conflict")
}

func testColorFn(colorName, symbol string) string {
	return fmt.Sprintf("[%s:%s]", colorName, symbol)
}

func TestFormatStatusLineCommitTimeNoMsg(t *testing.T) {
	m := &model{
		statuses: map[string]runner.StatusResult{
			"repo1": {
				Status: backend.RepoStatus{
					Bookmarks: []backend.BookmarkStatus{
						{Name: "main", State: backend.RefStateSynced},
					},
					CommitTime: "yesterday",
				},
			},
		},
	}
	line := m.formatStatusLine("repo1")

	assert.Contains(t, line, "yesterday", "expected commit time in output")
}

func TestFormatStatusLineRefAndSymsOnly(t *testing.T) {
	m := &model{
		statuses: map[string]runner.StatusResult{
			"repo1": {
				Status: backend.RepoStatus{
					Bookmarks: []backend.BookmarkStatus{
						{Name: "main", State: backend.RefStateAhead, Ahead: 1},
					},
				},
			},
		},
	}
	line := m.formatStatusLine("repo1")

	assert.NotEmpty(t, line, "expected non-empty output for ref with symbol")
}

func TestHandleKeyMsgQQuit(t *testing.T) {
	m := &model{}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	assert.NotNil(t, cmd, "expected non-nil quit cmd when q pressed on main screen")
}

func TestHandleExecResultSuccess(t *testing.T) {
	resultsCh := make(chan runner.Result)
	close(resultsCh)

	m := &model{
		execResults: []execResult{},
		output:      viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
		resultsCh:   resultsCh,
	}

	_, cmd := m.handleExecResult(execResultMsg{
		result: execResult{name: "repo1", result: runner.Result{ExitCode: 0}},
	})

	require.NotNil(t, cmd, "expected non-nil cmd after exec result")

	msg := cmd()
	require.IsType(
		t,
		execDoneMsg{},
		msg,
		"expected execDoneMsg from streamNextResult with closed channel",
	)
}

func TestHandleExecResultError(t *testing.T) {
	m := &model{
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleExecResult(execResultMsg{
		err: errors.New("command failed"),
	})

	assert.False(t, m.executing, "executing should be false after error")
	assert.Nil(t, cmd, "expected nil cmd after error")
}

func TestHandleExecDone(t *testing.T) {
	m := &model{
		executing: true,
		output:    viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleExecDone(execDoneMsg{})

	assert.False(t, m.executing, "executing should be false after exec done")
	assert.Nil(t, cmd, "expected nil cmd after exec done")
}

func TestHandleExecDoneWithSideEffect(t *testing.T) {
	m := &model{
		executing:      true,
		execSideEffect: true,
		output:         viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleExecDone(execDoneMsg{})

	assert.False(t, m.executing, "executing should be false after exec done")
	assert.False(t, m.execSideEffect, "execSideEffect should be reset after exec done")
	assert.NotNil(t, cmd, "expected non-nil cmd (refresh) when execSideEffect is true")
}
