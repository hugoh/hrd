package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
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
)

func TestMain(m *testing.M) {
	git.Register()
	jj.Register()
	os.Exit(m.Run())
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()

	for _, cmd := range []string{
		"git init",
		"git config user.email test@test.com",
		"git config user.name test",
		"echo content > file.txt",
		"git add .",
		"git commit -m 'initial commit'",
	} {
		c := exec.Command("bash", "-c", cmd)
		c.Dir = dir
		out, err := c.CombinedOutput()
		require.NoError(t, err, "git setup failed: %s", out)
	}
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
	m, err := newTestModel(t, ctx, Options{
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
	m, err := newTestModel(t, t.Context(), Options{})
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
	m, err := newTestModel(t, t.Context(), Options{})
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
	m, err := newTestModel(t, t.Context(), Options{})
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
	m, err := newTestModel(t, t.Context(), Options{})
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

func TestHandleCmdBarOpen(t *testing.T) {
	m := &model{
		cmdPrefix:   prefixNone,
		commandOpen: false,
	}
	m.initInput()

	_, _ = m.handleCmdBarOpen()
	assert.True(t, m.commandOpen, "commandOpen should be true after handleCmdBarOpen")
	assert.Equal(t, prefixNone, m.cmdPrefix, "cmdPrefix")
}

func TestHandleSelectToggle(t *testing.T) {
	m := baseModel([]string{"a"}, map[string]bool{"a": true})

	_, _ = m.handleSelectToggle()
	assert.Equal(t, modeSelect, m.mode, "mode should be modeSelect after first toggle")

	_, _ = m.handleSelectToggle()
	assert.NotEqual(t, modeSelect, m.mode, "mode should be modeNormal after second toggle")
}

func TestHandleSelectOne(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true})

	_, _ = m.handleSelectOne()

	assert.False(t, m.selected["a"], "repo 'a' should be deselected after handleSelectOne")
}

func TestHandleSelectAll(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true})

	_, _ = m.handleSelectAll()
	assert.True(
		t,
		m.selected["a"] && m.selected["b"],
		"all repos should be selected after handleSelectAll",
	)

	_, _ = m.handleSelectAll()
	assert.False(
		t,
		m.selected["a"] || m.selected["b"],
		"no repos should be selected after second handleSelectAll",
	)
}

func TestMainKeyXTogglesSelectMode(t *testing.T) {
	m := baseModel([]string{"a"}, map[string]bool{"a": true})
	m.ready = true

	_, cmd := m.handleMainKey(tea.KeyPressMsg{Code: 'x'})
	assert.Equal(
		t,
		modeSelect,
		m.mode,
		"mode should be modeSelect after pressing x (entered select mode)",
	)
	assert.NotEqual(
		t,
		modeSingle,
		m.mode,
		"mode should not be modeSingle after entering select mode",
	)
	assert.Nil(t, cmd, "expected nil cmd")

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: 'x'})
	assert.Equal(
		t,
		modeSelect,
		m.mode,
		"mode should remain modeSelect after another x (stays in select mode)",
	)
	assert.False(t, m.selected["a"], "repo 'a' should be deselected after x in select mode")
	assert.Nil(t, cmd, "expected nil cmd")

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.NotEqual(
		t,
		modeSelect,
		m.mode,
		"mode should be modeNormal after enter (exited select mode)",
	)
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestMainKeyXSelectsOne(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true, "b": false})
	m.mode = modeSelect
	m.ready = true

	_, cmd := m.handleMainKey(tea.KeyPressMsg{Code: 'x'})
	assert.False(t, m.selected["a"], "repo 'a' should be deselected after x in select mode")
	assert.Equal(t, 1, m.cursor, "cursor")
	assert.Nil(t, cmd, "expected nil cmd")

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: 'x'})
	assert.True(t, m.selected["b"], "repo 'b' should be selected after second x")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestMainKeySpaceSelectsOne(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true, "b": false})
	m.mode = modeSelect
	m.ready = true

	_, cmd := m.handleMainKey(tea.KeyPressMsg{Code: ' '})
	assert.False(t, m.selected["a"], "repo 'a' should be deselected after space in select mode")
	assert.Equal(t, 1, m.cursor, "cursor should advance after space")
	assert.Nil(t, cmd, "expected nil cmd")

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: ' '})
	assert.True(t, m.selected["b"], "repo 'b' should be selected after second space")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestMainKeySSingleToggle(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true})
	m.ready = true

	_, cmd := m.handleMainKey(tea.KeyPressMsg{Code: 's'})
	assert.Equal(t, modeSingle, m.mode, "mode should be modeSingle after pressing s")
	assert.NotEqual(
		t,
		modeSelect,
		m.mode,
		"mode should not be modeSelect after entering single mode",
	)
	assert.Nil(t, cmd, "expected nil cmd")

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: 's'})
	assert.NotEqual(t, modeSingle, m.mode, "mode should be modeNormal after pressing s again")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestMainKeyXSingleModeSelectsOne(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true, "b": false})
	m.ready = true
	m.mode = modeSingle

	names := m.selectedNames()
	require.Len(t, names, 1, "selectedNames() should have 1 element")
	assert.Equal(t, "a", names[0], "selectedNames()[0]")
}

func TestEscExitsSelectMode(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true})
	m.ready = true
	m.mode = modeSelect

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})
	assert.NotEqual(t, modeSelect, m.mode, "mode should be modeNormal after esc")
	assert.NotEqual(t, modeSingle, m.mode, "mode should not be modeSingle after esc")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestEscDiscardsSelectModeChanges(t *testing.T) {
	m := selectSavedModel()

	m.handleSelectOne()
	m.handleSelectOne()
	assert.True(t, m.selected["b"], "b should be toggled on during select mode")
	assert.False(t, m.selected["a"], "a should be toggled off during select mode")

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})
	assert.Equal(t, modeNormal, m.mode, "mode should be modeNormal after esc")
	assert.True(t, m.selected["a"], "a should be restored to original state")
	assert.False(t, m.selected["b"], "b should be restored to original state")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestEnterInSelectModePersistsChanges(t *testing.T) {
	m := selectSavedModel()

	m.handleSelectOne()
	assert.False(t, m.selected["a"], "a should be toggled off")

	_, cmd := m.handleMainKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, modeNormal, m.mode, "mode should be modeNormal after enter")
	assert.False(t, m.selected["a"], "a should stay toggled off after enter")
	assert.False(t, m.selected["b"], "b should stay unchanged")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestEscExitsSingleMode(t *testing.T) {
	m := baseModel([]string{"a"}, map[string]bool{"a": true})
	m.ready = true
	m.mode = modeSingle

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})
	assert.NotEqual(t, modeSingle, m.mode, "mode should be modeNormal after esc")
	assert.NotEqual(t, modeSelect, m.mode, "mode should not be modeSelect after esc")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleSingleToggle(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true})

	_, _ = m.handleSingleToggle()
	assert.Equal(t, modeSingle, m.mode, "mode should be modeSingle after first toggle")

	_, _ = m.handleSingleToggle()
	assert.NotEqual(t, modeSingle, m.mode, "mode should be modeNormal after second toggle")
}

func TestHandleCursorUpDown(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "b": true, "c": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
	}
	m.cursor = 1
	m.initTable()

	_, _ = m.handleCursorUp()

	assert.Equal(t, 0, m.cursor, "cursor after up")

	_, _ = m.handleCursorUp()

	assert.Equal(t, 0, m.cursor, "cursor should stay at 0 when at top")

	_, _ = m.handleCursorDown()

	assert.Equal(t, 1, m.cursor, "cursor after down")
}

func TestHandleMouseClickPositionsCursor(t *testing.T) {
	m := baseModel(
		[]string{"a", "b", "c", "d"},
		map[string]bool{"a": true, "b": true, "c": true, "d": true},
	)
	m.ready = true
	m.mode = modeNormal

	_, cmd := m.Update(tea.MouseClickMsg{
		X:      0,
		Y:      5,
		Button: tea.MouseLeft,
	})
	assert.Equal(t, 2, m.cursor, "cursor should move to row 2 (Y=5 minus 3)")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickSelectModeTogglesSelection(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true, "b": false})
	m.ready = true
	m.mode = modeSelect

	m.Update(tea.MouseClickMsg{
		X:      0,
		Y:      3,
		Button: tea.MouseLeft,
	})
	assert.False(t, m.selected["a"], "repo 'a' should be deselected after click on first data row")
	assert.False(
		t,
		m.selected["b"],
		"repo 'b' should remain deselected (cursor moved to a, b not toggled)",
	)
	assert.Equal(t, 0, m.cursor, "cursor should be on repo a (row 0)")

	m.Update(tea.MouseClickMsg{
		X:      0,
		Y:      4,
		Button: tea.MouseLeft,
	})
	assert.False(t, m.selected["a"], "repo 'a' should remain deselected")
	assert.True(t, m.selected["b"], "repo 'b' should be toggled on after click")
	assert.Equal(t, 1, m.cursor, "cursor should be on repo b (row 1)")
}

func TestHandleMouseClickAboveTableReturnsEarly(t *testing.T) {
	m := baseModel([]string{"a"}, map[string]bool{"a": true})
	m.ready = true

	t.Run("app header", func(t *testing.T) {
		_, cmd := m.Update(tea.MouseClickMsg{
			X:      0,
			Y:      0,
			Button: tea.MouseLeft,
		})
		assert.Equal(t, 0, m.cursor, "cursor should stay at 0")
		assert.Nil(t, cmd, "expected nil cmd")
	})

	t.Run("separator", func(t *testing.T) {
		_, cmd := m.Update(tea.MouseClickMsg{
			X:      0,
			Y:      1,
			Button: tea.MouseLeft,
		})
		assert.Equal(t, 0, m.cursor, "cursor should stay at 0")
		assert.Nil(t, cmd, "expected nil cmd")
	})

	t.Run("table header", func(t *testing.T) {
		_, cmd := m.Update(tea.MouseClickMsg{
			X:      0,
			Y:      2,
			Button: tea.MouseLeft,
		})
		assert.Equal(t, 0, m.cursor, "cursor should stay at 0")
		assert.Nil(t, cmd, "expected nil cmd")
	})
}

func TestHandleMouseClickNonMainScreenDoesNothing(t *testing.T) {
	m := &model{
		repoOrder: []string{"a"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
		ready:     true,
		screen:    screenHelp,
	}
	m.cursor = 0
	m.initTable()

	m.Update(tea.MouseClickMsg{
		X:      0,
		Y:      1,
		Button: tea.MouseLeft,
	})
	assert.Equal(t, 0, m.cursor, "cursor should stay at 0 on non-main screen")
}

func TestHandleMouseWheelScrolls(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c", "d", "e", "f", "g"},
		selected: map[string]bool{
			"a": true,
			"b": true,
			"c": true,
			"d": true,
			"e": true,
			"f": true,
			"g": true,
		},
		cfg:   config.Config{Settings: config.Settings{Concurrency: 1}},
		ready: true,
	}
	m.cursor = 3
	m.initTable()

	m.Update(tea.MouseWheelMsg{
		Y:      0,
		Button: tea.MouseWheelUp,
	})
	assert.Equal(t, 0, m.cursor, "cursor should scroll up by delta")

	m.Update(tea.MouseWheelMsg{
		Y:      0,
		Button: tea.MouseWheelDown,
	})
	assert.Equal(t, 3, m.cursor, "cursor should scroll back down")
}

func TestHandleMouseWheelStaysAtBounds(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true, "b": true})
	m.ready = true

	m.Update(tea.MouseWheelMsg{
		Y:      0,
		Button: tea.MouseWheelUp,
	})
	assert.Equal(t, 0, m.cursor, "cursor should stay at 0 when at top")
}

func TestHandleMouseWheelOutputScreenScrollsOutput(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true, "b": true})
	m.ready = true
	m.screen = screenOutput
	m.initOutput()
	m.output.SetContent(
		"line0\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\nline13\nline14",
	)

	prevOffset := m.output.YOffset()
	m.Update(tea.MouseWheelMsg{
		Y:      0,
		Button: tea.MouseWheelDown,
	})
	assert.Equal(t, 0, m.cursor, "main table cursor should not move on output screen")
	assert.Greater(t, m.output.YOffset(), prevOffset, "output viewport should scroll down")
}

func TestHandleMouseWheelHelpScreenScrollsHelp(t *testing.T) {
	m := &model{
		cfg:    config.Config{Settings: config.Settings{Concurrency: 1}},
		ready:  true,
		screen: screenHelp,
	}
	m.initTable()
	m.initHelpViewport()
	m.helpViewport.SetContent(
		"line0\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\nline13\nline14",
	)

	prevOffset := m.helpViewport.YOffset()
	m.Update(tea.MouseWheelMsg{
		Y:      0,
		Button: tea.MouseWheelDown,
	})
	assert.Greater(t, m.helpViewport.YOffset(), prevOffset, "help viewport should scroll down")
}

func TestHandleModalKeyAlert(t *testing.T) {
	m := &model{
		modal: modalAlert,
	}
	m.initTable()

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: ' '})

	assert.Nil(t, cmd, "expected nil cmd after closing alert")
	assert.Equal(t, modalNone, m.modal, "alert should be cleared after any key")
}

func TestHandleModalKeyAlertAt(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Groups: map[string]config.Group{"work": {}},
		},
		modal: modalAlert,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: '@'})

	assert.Equal(t, screenGroup, m.screen, "screen should be screenGroup after @ in alert")
	assert.Nil(t, cmd, "expected nil cmd after closing alert")
}

func TestHandleGroupKeyEnterNonAll(t *testing.T) {
	m := &model{
		ctx: t.Context(),
		cfg: config.Config{
			Repos:  map[string]config.Repo{"r1": {}},
			Groups: map[string]config.Group{"work": {Repos: []string{"r1"}}},
		},
		repoOrder: []string{"r1"},
		selected:  map[string]bool{},
	}
	m.initTable()
	m.initGroupList()
	openGroupPopup(m, groupFilterMode)
	m.groupList.Select(1)

	_, _ = m.handleGroupKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, "work", m.groupFilter, "groupFilter")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.True(t, m.loading, "loading should be true after group selection")
}

func TestHandleKeyMsgRefresh(t *testing.T) {
	m := &model{
		loading:  true,
		statuses: map[string]runner.StatusResult{},
	}
	m.repoOrder = []string{}
	m.selected = map[string]bool{}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'r'})

	assert.NotNil(t, cmd, "expected non-nil cmd for refresh")
}

func TestHandleKeyMsgHelp(t *testing.T) {
	m := &model{}
	m.initHelpViewport()

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: '?'})

	assert.Equal(t, screenHelp, m.screen, "screen")
	assert.Nil(t, cmd, "expected nil cmd for help")
}

func TestHandleKeyMsgCtrlCExecuting(t *testing.T) {
	m := &model{
		executing: true,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	assert.Nil(t, cmd, "expected nil cmd when Ctrl+C during execution")
}

func TestHandleKeyMsgQCommandOpen(t *testing.T) {
	m := &model{
		commandOpen: true,
	}
	m.initInput()
	m.input.Focus()

	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: 'q', Text: "q"})

	assert.Equal(t, "q", m.input.Value(), "expected input value to be 'q'")
}

func TestHandleKeyMsgQOutputScreen(t *testing.T) {
	m := &model{
		screen: screenOutput,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	assert.Nil(t, cmd, "expected nil cmd when q pressed in output mode")
}

func TestHandleKeyMsgQWithHelpScreen(t *testing.T) {
	m := &model{
		screen: screenHelp,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	assert.Nil(t, cmd, "expected nil cmd when q pressed on help screen")
	assert.Equal(t, screenMain, m.screen, "should return to main screen after q on help")
}

func TestHandleKeyMsgQWithAlert(t *testing.T) {
	m := &model{
		modal: modalAlert,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	assert.Nil(t, cmd, "expected nil cmd when q pressed with alert")
	assert.Equal(t, modalNone, m.modal, "alert should be dismissed on q")
}

func TestHandleKeyMsgQQuit(t *testing.T) {
	m := &model{}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	assert.NotNil(t, cmd, "expected non-nil quit cmd when q pressed on main screen")
}

func TestShortcutCmdSuccess(t *testing.T) {
	m := &model{
		repoOrder: []string{"a"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
		persState: PersistentState{History: []HistoryEntry{}},
		output:    viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	cmd := shortcutCmd(m, "status", false)

	assert.NotNil(t, cmd, "expected non-nil cmd when repos selected")
	assert.Equal(t, screenOutput, m.screen, "screen")
}

func TestShortcutCmdNoSelected(t *testing.T) {
	m := &model{
		repoOrder: []string{},
		selected:  map[string]bool{},
	}

	cmd := shortcutCmd(m, "status", false)
	assert.Nil(t, cmd, "expected nil cmd when no repos selected")
	assert.Equal(t, modalAlert, m.modal, "modal")
}

func TestOpenGroupPopup(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Groups: map[string]config.Group{
				"work":     {},
				"personal": {},
			},
		},
		groupFilter: "work",
	}
	m.initGroupList()

	openGroupPopup(m, groupFilterMode)

	assert.Equal(t, screenGroup, m.screen, "screen")
	assert.Len(t, m.groupList.Items(), 3, "expected 3 popup items")
	assert.Equal(t, 2, m.groupList.Index(), "cursor should point to work")
}

func TestOpenGroupPopupNoGroupFilter(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Groups: map[string]config.Group{
				"work": {},
			},
		},
	}
	m.initGroupList()

	openGroupPopup(m, groupFilterMode)

	assert.Equal(t, screenGroup, m.screen, "screen")
	assert.Len(t, m.groupList.Items(), 2, "expected 2 popup items")
	assert.Equal(t, 0, m.groupList.Index(), "cursor should be at [all]")
}

func TestHandleOutputKeyEsc(t *testing.T) {
	m := &model{
		screen: screenOutput,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleOutputKey(tea.KeyPressMsg{Code: tea.KeyEsc})

	assert.Equal(t, screenMain, m.screen, "screen")
	assert.Nil(t, cmd, "expected nil cmd when closing output")
}

func TestHandleOutputKeyQ(t *testing.T) {
	m := &model{
		screen: screenOutput,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleOutputKey(tea.KeyPressMsg{Code: 'q'})

	assert.Equal(t, screenMain, m.screen, "screen")
	assert.Nil(t, cmd, "expected nil cmd when closing output with q")
}

func TestSortedSelected(t *testing.T) {
	selected := map[string]bool{"c": true, "a": true, "b": false}
	got := sortedSelected(selected)

	assert.Equal(t, []string{"a", "c"}, got)
}

func TestEqualStringSlices(t *testing.T) {
	assert.True(t, equalStringSlices([]string{"a", "b"}, []string{"a", "b"}),
		"equalStringSlices should return true for equal slices")
	assert.False(t, equalStringSlices([]string{"a"}, []string{"a", "b"}),
		"equalStringSlices should return false for different lengths")
	assert.False(t, equalStringSlices([]string{"a", "b"}, []string{"a", "c"}),
		"equalStringSlices should return false for different elements")
}

func TestPushSelectionHistory(t *testing.T) {
	m := &model{
		selected:  map[string]bool{"a": true, "b": true},
		persState: PersistentState{SelectionHistory: []SelectionEntry{}},
	}

	m.pushSelectionHistory()

	require.Len(t, m.persState.SelectionHistory, 1)

	entry := m.persState.SelectionHistory[0]
	assert.Equal(t, []string{"a", "b"}, entry.Repos)
	assert.False(t, entry.Timestamp.IsZero())

	// Push identical state — should be deduped
	m.pushSelectionHistory()

	assert.Len(t, m.persState.SelectionHistory, 1, "SelectionHistory should be deduped")

	// Push different state
	m.selected = map[string]bool{"a": true}
	m.pushSelectionHistory()

	assert.Len(t, m.persState.SelectionHistory, 2, "SelectionHistory length")
}

func TestOpenSelHistoryPopup(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Repos: map[string]config.Repo{"a": {}, "b": {}},
		},
		persState: PersistentState{
			SelectionHistory: []SelectionEntry{
				{Repos: []string{"a", "b"}},
			},
		},
	}
	m.initHistoryList()
	openSelHistoryPopup(m)

	assert.Equal(t, screenSelHistory, m.screen, "screen")
	assert.Equal(t, 0, m.historyList.Index(), "cursor")
}

func TestHandleSelHistoryKeyNavigation(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Repos: map[string]config.Repo{"a": {}, "b": {}, "c": {}, "d": {}},
		},
		persState: PersistentState{
			SelectionHistory: []SelectionEntry{
				{Repos: []string{"a", "b"}},
				{Repos: []string{"c"}},
				{Repos: []string{"d"}},
			},
		},
	}
	m.initTable()
	m.initHistoryList()

	openSelHistoryPopup(m)

	require.Equal(t, 0, m.historyList.Index(), "initial cursor")

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, m.historyList.Index(), "cursor after down")

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, m.historyList.Index(), "cursor after up")

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: 'j'})
	assert.Equal(t, 1, m.historyList.Index(), "cursor after j")

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: 'k'})
	assert.Equal(t, 0, m.historyList.Index(), "cursor after k")

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, m.historyList.Index(), "cursor at top")
}

func TestHandleSelHistoryKeyEmpty(t *testing.T) {
	m := &model{
		screen:    screenSelHistory,
		persState: PersistentState{SelectionHistory: []SelectionEntry{}},
	}
	m.initTable()
	m.initHistoryList()

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, screenSelHistory, m.screen, "empty list, no-op")
}

func TestHandleSelHistoryRestore(t *testing.T) {
	m := &model{
		ctx:       t.Context(),
		repoOrder: []string{"a", "b", "c"},
		cfg: config.Config{
			Repos: map[string]config.Repo{
				"a": {},
				"b": {},
				"c": {},
			},
			Settings: config.Settings{Concurrency: 1},
		},
		persState: PersistentState{SelectionHistory: []SelectionEntry{}},
		mode:      modeSingle,
	}
	m.initTable()

	repos := []string{"a", "c"}
	_, cmd := m.handleSelHistoryRestore(repos)

	assert.Equal(t, modeNormal, m.mode, "mode should be modeNormal after restore")
	assert.True(t, m.selected["a"] && m.selected["c"] && !m.selected["b"],
		"restore should set correct selected repos")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.NotNil(t, cmd, "expected non-nil cmd (refresh) after restore")
	assert.Equal(t, modalNone, m.modal, "modal should not be set when all repos exist")
}

func TestHandleSelHistoryRestoreMissingRepo(t *testing.T) {
	m := &model{
		ctx:       t.Context(),
		repoOrder: []string{"a"},
		cfg: config.Config{
			Repos: map[string]config.Repo{
				"a": {},
			},
			Settings: config.Settings{Concurrency: 1},
		},
		persState: PersistentState{SelectionHistory: []SelectionEntry{}},
	}
	m.initTable()

	repos := []string{"a", "stale_repo"}
	_, _ = m.handleSelHistoryRestore(repos)

	assert.Equal(t, modalAlert, m.modal, "modal should be modalAlert when repos are missing")
	assert.NotEmpty(t, m.alertMsg, "alertMsg should be set when repos are missing")
	assert.True(t, m.selected["a"], "existing repos should still be selected")
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
		err:  errors.New("command failed"),
		done: true,
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

func TestHandleInputKeyEnterEmptyString(t *testing.T) {
	m := &model{
		commandOpen: true,
	}
	m.initInput()
	m.input.SetValue("   ")

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd, "expected nil cmd for whitespace-only command")
}

func TestHandleInputKeyEnterEmpty(t *testing.T) {
	m := &model{
		commandOpen: true,
	}
	m.initInput()

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd, "expected nil cmd for empty command")
}

func TestHandleInputKeyEnterExecuting(t *testing.T) {
	m := &model{
		commandOpen: true,
		executing:   true,
	}
	m.initInput()
	m.input.SetValue("status")

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd, "expected nil cmd when already executing")
}

func TestHandleInputKeyEsc(t *testing.T) {
	m := &model{
		commandOpen: true,
	}
	m.initInput()

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyEsc})

	assert.False(t, m.commandOpen, "commandOpen should be false after Esc")
	assert.Nil(t, cmd, "expected nil cmd after Esc in input mode")
}

func TestContentHeightWithCommandOpen(t *testing.T) {
	m := &model{
		height:      10,
		commandOpen: true,
	}
	h := m.contentHeight()

	assert.GreaterOrEqual(t, h, 3, "contentHeight()")
}

func TestContentHeightSmall(t *testing.T) {
	m := &model{
		height: 2,
	}
	h := m.contentHeight()

	assert.GreaterOrEqual(t, h, 3, "contentHeight()")
}

func TestInputWidthNarrow(t *testing.T) {
	m := &model{width: 5}
	w := m.inputWidth()

	assert.GreaterOrEqual(t, w, minInputWidth, "inputWidth()")
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

func TestOpenGroupPopupAddMode(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Repos: map[string]config.Repo{
				"r1": {Groups: []string{"work", "personal"}},
			},
			Groups: map[string]config.Group{
				"personal": {Repos: []string{"r1"}},
				"work":     {Repos: []string{"r1"}},
			},
		},
	}
	m.initGroupList()

	openGroupPopup(m, groupAddMode)

	assert.Equal(t, screenGroup, m.screen, "screen")
	assert.Len(t, m.groupList.Items(), 3, "expected 3 popup items")
	assert.Equal(t, groupAddMode, m.groupMode, "groupMode")
}

func TestHandleGroupEnterFilterModeAll(t *testing.T) {
	m := &model{
		ctx: t.Context(),
		cfg: config.Config{
			Repos:  map[string]config.Repo{"r1": {Groups: []string{"work"}}},
			Groups: map[string]config.Group{"work": {Repos: []string{"r1"}}},
		},
		repoOrder:   []string{"r1"},
		selected:    map[string]bool{},
		groupFilter: "work",
	}
	m.initTable()
	m.initGroupList()
	openGroupPopup(m, groupFilterMode)
	m.groupList.Select(0)

	_, cmd := m.handleGroupEnter()

	assert.Empty(t, m.groupFilter, "groupFilter")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.NotNil(t, cmd, "expected non-nil cmd (refresh) after group selection")
}

func TestHandleGroupEnterAddModeExistingGroup(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")

	m := &model{
		ctx: t.Context(),
		cfg: config.Config{
			Repos: map[string]config.Repo{
				"repo1": {Groups: []string{"work"}},
				"repo2": {},
			},
			Groups: map[string]config.Group{"work": {Repos: []string{"repo1"}}},
		},
		opts:      Options{ConfigPath: cfgPath},
		repoOrder: []string{"repo1", "repo2"},
		selected:  map[string]bool{"repo2": true},
		groupMode: groupAddMode,
	}
	m.initTable()
	m.initGroupList()
	openGroupPopup(m, groupAddMode)
	m.groupList.Select(0)

	_, cmd := m.handleGroupEnter()

	assert.Nil(t, cmd, "expected nil cmd after adding to existing group")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.Len(t, m.cfg.Groups["work"].Repos, 2, "expected 2 repos in work group")
}

func TestHandleGroupEnterAddModeNew(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Repos: map[string]config.Repo{"repo1": {}},
		},
		repoOrder: []string{"repo1"},
		selected:  map[string]bool{"repo1": true},
		groupMode: groupAddMode,
	}
	m.initGroupList()
	openGroupPopup(m, groupAddMode)
	m.groupList.Select(0)
	m.initInput()

	_, cmd := m.handleGroupEnter()

	assert.Nil(t, cmd, "expected nil cmd when entering new group name")
	assert.True(t, m.groupNewInput, "groupNewInput should be true after selecting [new...]")
}

func TestHandleGroupNewInputEnter(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")

	m := &model{
		cfg: config.Config{
			Repos: map[string]config.Repo{
				"repo1": {},
				"repo2": {},
			},
			Groups: map[string]config.Group{},
		},
		opts:          Options{ConfigPath: cfgPath},
		repoOrder:     []string{"repo1", "repo2"},
		selected:      map[string]bool{"repo2": true},
		groupNewInput: true,
		screen:        screenGroup,
	}
	m.initInput()
	m.input.SetValue("my-group")

	_, cmd := m.handleGroupNewInput(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd, "expected nil cmd after creating new group")
	assert.False(t, m.groupNewInput, "groupNewInput should be false after Enter")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.Contains(t, m.cfg.Groups, "my-group", "my-group should exist in config after creation")
}

func TestHandleGroupNewInputEmpty(t *testing.T) {
	m := &model{
		groupNewInput: true,
	}
	m.initInput()

	_, cmd := m.handleGroupNewInput(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd, "expected nil cmd for empty group name")
	assert.True(t, m.groupNewInput, "groupNewInput should remain true when name is empty")
}

func TestHandleGroupNewInputEsc(t *testing.T) {
	m := &model{
		groupNewInput: true,
	}
	m.initInput()

	_, cmd := m.handleGroupNewInput(tea.KeyPressMsg{Code: tea.KeyEsc})

	assert.Nil(t, cmd, "expected nil cmd after Esc")
	assert.False(t, m.groupNewInput, "groupNewInput should be false after Esc")
}

func TestHandleGroupKeyNavigation(t *testing.T) {
	m := &model{
		groupMode: groupFilterMode,
	}
	m.initGroupList()
	m.groupList.SetItems([]list.Item{
		groupItem{name: "a"},
		groupItem{name: "b"},
		groupItem{name: "c"},
	})
	m.groupList.Select(1)

	_, cmd := m.handleGroupKey(tea.KeyPressMsg{Code: 'k'})
	assert.Nil(t, cmd, "expected nil cmd after up")
	assert.Equal(t, 0, m.groupList.Index(), "cursor after up")

	_, cmd = m.handleGroupKey(tea.KeyPressMsg{Code: 'j'})
	assert.Nil(t, cmd, "expected nil cmd after down")
	assert.Equal(t, 1, m.groupList.Index(), "cursor after down")

	m.groupList.Select(0)
	m.handleGroupKey(tea.KeyPressMsg{Code: 'k'})
	assert.Equal(t, 0, m.groupList.Index(), "cursor at top")

	m.groupList.Select(2)
	m.handleGroupKey(tea.KeyPressMsg{Code: 'j'})
	assert.Equal(t, 2, m.groupList.Index(), "cursor at bottom")
}

func TestHandleGroupKeyEnterOnAllFilterMode(t *testing.T) {
	m := &model{
		ctx: t.Context(),
		cfg: config.Config{
			Repos:  map[string]config.Repo{"r1": {}},
			Groups: map[string]config.Group{"work": {Repos: []string{"r1"}}},
		},
		repoOrder: []string{"r1"},
		selected:  map[string]bool{},
	}
	m.initTable()
	m.initGroupList()
	openGroupPopup(m, groupFilterMode)
	m.groupList.Select(0)

	_, cmd := m.handleGroupKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Empty(t, m.groupFilter, "groupFilter")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.NotNil(t, cmd, "expected non-nil cmd (refresh) after group selection")
}
