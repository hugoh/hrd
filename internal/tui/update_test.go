package tui

import (
	"context"
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
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/theme"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	git.Register()
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
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git setup failed: %v\n%s", err, out)
		}
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

	ctx := context.Background()
	m, err := newModel(ctx, Options{
		ConfigPath: cfgPath,
		StatePath:  filepath.Join(tmp, "state.json"),
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
	if cmd != nil {
		t.Log("handleStatusUpdate returned a continuation cmd")
	}

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
	if cmd != nil {
		t.Error("handleStatusUpdate with no filtered repos should return nil cmd")
	}
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
	if cmd != nil {
		t.Error("handleStatusDone should return nil cmd")
	}

	if m.loading {
		t.Error("loading should be false after handleStatusDone")
	}

	if m.statusCh != nil {
		t.Error("statusCh should be nil after handleStatusDone")
	}
}

func TestRefColumnWidthAfterWindowSize(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	m, err := newModel(context.Background(), Options{StatePath: statePath})
	require.NoError(t, err)

	m.cfg = config.Config{
		Repos: map[string]config.Repo{
			"test": {Path: t.TempDir(), Backends: []string{"git"}},
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
	m, err := newModel(context.Background(), Options{})
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
		m.selectMode = false
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
		m.selectMode = false
		m.selected = map[string]bool{}
		m.updateTableRows()

		rows := m.repoTable.Rows()
		require.Empty(t, rows, "table should be empty when no repos selected")
	})

	t.Run("select mode shows checkbox", func(t *testing.T) {
		m.selectMode = true
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

	if lipgloss.Width(cursor) != lipgloss.Width(nonCursor) {
		t.Errorf("cursor row width (%d) differs from non-cursor (%d)",
			lipgloss.Width(cursor), lipgloss.Width(nonCursor))
	}
}

func TestNameNotTruncatedWhenSelected(t *testing.T) {
	// A name that fits within maxNameWidth (24) must not be passed
	// with embedded ANSI codes, because runewidth.Truncate treats ANSI
	// escapes as printable chars, causing premature truncation → mangled output.
	m, err := newModel(context.Background(), Options{})
	require.NoError(t, err)

	m.cfg = config.Config{
		Repos: map[string]config.Repo{
			"AppBadgeWatcher.spoon": {Path: t.TempDir(), Backends: []string{"git"}},
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
	m, err := newModel(context.Background(), Options{})
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
	m.selectMode = true
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

	if !strings.Contains(got, "2/2 repos completed successfully") {
		t.Errorf("coloredSummary() = %q, want success message", got)
	}
}

func TestColoredSummaryWithFailures(t *testing.T) {
	errFoo := fmt.Errorf("foo failed") //nolint:perfsprint // test readability
	m := &model{
		execTotal: 3,
		execResults: []execResult{
			{name: "repo-a", result: runner.Result{ExitCode: 0, Output: "ok"}},
			{name: "repo-b", result: runner.Result{Err: errFoo}},
			{name: "repo-c", result: runner.Result{ExitCode: 1}},
		},
	}
	got := m.coloredSummary()

	if !strings.Contains(got, "failed:") ||
		!strings.Contains(got, "repo-b") ||
		!strings.Contains(got, "repo-c") {
		t.Errorf("coloredSummary() = %q, want summary with failures", got)
	}
}

func TestFormatDispatchResultLineSuccess(t *testing.T) {
	res := runner.Result{ExitCode: 0, Output: "hello"}
	line := formatDispatchResultLine("myrepo", res, 40)

	if !strings.Contains(line, "myrepo") {
		t.Errorf("output should contain repo name, got %q", line)
	}

	if !strings.Contains(line, "hello") {
		t.Errorf("output should contain 'hello', got %q", line)
	}
}

func TestFormatDispatchResultLineError(t *testing.T) {
	res := runner.Result{
		Err:    fmt.Errorf("something broke"), //nolint:perfsprint // test readability
		Output: "details",
	}
	line := formatDispatchResultLine("myrepo", res, 40)

	if !strings.Contains(line, "error:") {
		t.Errorf("output should contain 'error:', got %q", line)
	}

	if !strings.Contains(line, "something broke") {
		t.Errorf("output should contain error msg, got %q", line)
	}
}

func TestFormatDispatchResultLineNonZeroExit(t *testing.T) {
	res := runner.Result{ExitCode: 1, Output: "oops"}
	line := formatDispatchResultLine("myrepo", res, 40)

	if !strings.Contains(line, "exit 1") {
		t.Errorf("output should contain 'exit 1', got %q", line)
	}
}

func TestFormatDispatchResultLineNoOutput(t *testing.T) {
	res := runner.Result{ExitCode: 0}
	line := formatDispatchResultLine("myrepo", res, 40)

	if !strings.Contains(line, "myrepo") {
		t.Errorf("output should contain repo name, got %q", line)
	}
}

func TestFormatExecOutput(t *testing.T) {
	results := []execResult{
		{name: "a", result: runner.Result{ExitCode: 0, Output: "ok"}},
		{name: "b", result: runner.Result{ExitCode: 1, Output: "fail"}},
	}
	got := formatExecOutput(results, 40)

	if !strings.Contains(got, "a") {
		t.Errorf("output should contain 'a', got %q", got)
	}

	if !strings.Contains(got, "b") {
		t.Errorf("output should contain 'b', got %q", got)
	}
}

func TestFormatExecOutputEmpty(t *testing.T) {
	got := formatExecOutput(nil, 40)
	if got != "" {
		t.Errorf("expected empty output for nil results, got %q", got)
	}
}

func TestFormatStatusLineNoStatus(t *testing.T) {
	m := &model{statuses: map[string]runner.StatusResult{}}

	line := m.formatStatusLine("nonexistent")
	if line == "" {
		t.Error("expected muted placeholder for missing status")
	}
}

func TestFormatStatusLineError(t *testing.T) {
	m := &model{
		statuses: map[string]runner.StatusResult{
			"repo1": {
				Err: fmt.Errorf("something went wrong"), //nolint:perfsprint // test readability
			},
		},
	}
	line := m.formatStatusLine("repo1")

	if !strings.Contains(line, "something went wrong") {
		t.Errorf("expected error message in output, got %q", line)
	}
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

	if !strings.Contains(line, "main") {
		t.Errorf("expected bookmark name in output, got %q", line)
	}

	if !strings.Contains(line, "fix bug") {
		t.Errorf("expected commit msg in output, got %q", line)
	}
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

	if !strings.Contains(line, "develop") {
		t.Errorf("expected ref in output, got %q", line)
	}
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

	if line != "" {
		t.Errorf("expected empty output for empty status, got %q", line)
	}
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

	if syms == "" {
		t.Error("expected non-empty symbols")
	}
}

func TestRenderSymbolsDirty(t *testing.T) {
	status := backend.RepoStatus{
		Dirty:     true,
		Bookmarks: []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
	}
	syms := theme.FormatSymbols(status, testColorFn)

	if !strings.Contains(syms, "*") {
		t.Errorf("expected dirty marker, got %q", syms)
	}
}

func TestRenderSymbolsConflict(t *testing.T) {
	status := backend.RepoStatus{
		Conflict: true,
		Bookmarks: []backend.BookmarkStatus{
			{Name: "main", State: backend.RefStateSynced, Conflict: true},
		},
	}
	syms := theme.FormatSymbols(status, testColorFn)

	if syms == "" {
		t.Error("expected non-empty symbols for conflict")
	}
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

	if !strings.Contains(line, "yesterday") {
		t.Errorf("expected commit time in output, got %q", line)
	}
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

	if line == "" {
		t.Error("expected non-empty output for ref with symbol")
	}
}

func TestHandleCmdBarOpen(t *testing.T) {
	m := &model{
		cmdPrefix:   prefixNone,
		commandOpen: false,
	}
	m.initInput()

	_, _ = m.handleCmdBarOpen(prefixGit)
	if !m.commandOpen {
		t.Error("commandOpen should be true after handleCmdBarOpen")
	}

	if m.cmdPrefix != prefixGit {
		t.Errorf("cmdPrefix = %d, want %d", m.cmdPrefix, prefixGit)
	}
}

func TestHandleSelectToggle(t *testing.T) {
	m := &model{
		repoOrder: []string{"a"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
	}
	m.cursor = 0
	m.initTable()

	_, _ = m.handleSelectToggle()
	if !m.selectMode {
		t.Error("selectMode should be true after first toggle")
	}

	_, _ = m.handleSelectToggle()
	if m.selectMode {
		t.Error("selectMode should be false after second toggle")
	}
}

func TestHandleSelectOne(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
	}
	m.cursor = 0
	m.initTable()

	_, _ = m.handleSelectOne()

	if m.selected["a"] {
		t.Error("repo 'a' should be deselected after handleSelectOne")
	}
}

func TestHandleSelectAll(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
	}
	m.initTable()

	_, _ = m.handleSelectAll()
	if !m.selected["a"] || !m.selected["b"] {
		t.Error("all repos should be selected after handleSelectAll")
	}

	_, _ = m.handleSelectAll()
	if m.selected["a"] || m.selected["b"] {
		t.Error("no repos should be selected after second handleSelectAll")
	}
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

	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after up", m.cursor)
	}

	_, _ = m.handleCursorUp()

	if m.cursor != 0 {
		t.Errorf("cursor should stay at 0 when at top, got %d", m.cursor)
	}

	_, _ = m.handleCursorDown()

	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 after down", m.cursor)
	}
}

func TestHandleModalKeyAlert(t *testing.T) {
	m := &model{
		modal: modalAlert,
	}
	m.initTable()

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: ' '})

	if cmd != nil {
		t.Error("expected nil cmd after closing alert")
	}
}

func TestHandleModalKeyAlertAt(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Groups: map[string]config.Group{"work": {}},
		},
		modal: modalAlert,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: '@'})

	if m.modal != modalGroup {
		t.Errorf("modal should be modalGroup after @ in alert, got %v", m.modal)
	}

	if cmd != nil {
		t.Error("expected nil cmd after closing alert")
	}
}

func TestHandleGroupKeyEnterNonAll(t *testing.T) {
	m := &model{
		ctx: context.Background(),
		cfg: config.Config{
			Repos:  map[string]config.Repo{"r1": {}},
			Groups: map[string]config.Group{"work": {Repos: []string{"r1"}}},
		},
		repoOrder:         []string{"r1"},
		selected:          map[string]bool{},
		groupPopupOptions: []string{labelAllCap, "work"},
		groupPopupCursor:  1,
	}

	_, _ = m.handleGroupKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.groupFilter != "work" {
		t.Errorf("groupFilter = %q, want %q", m.groupFilter, "work")
	}

	if m.modal != modalNone {
		t.Errorf("modal = %d, want %d", m.modal, modalNone)
	}

	if !m.loading {
		t.Error("loading should be true after group selection")
	}
}

func TestHandleKeyMsgRefresh(t *testing.T) {
	m := &model{
		loading:  true,
		statuses: map[string]runner.StatusResult{},
	}
	m.repoOrder = []string{}
	m.selected = map[string]bool{}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'r'})

	if cmd == nil {
		t.Error("expected non-nil cmd for refresh")
	}
}

func TestHandleKeyMsgHelp(t *testing.T) {
	m := &model{}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: '?'})

	if m.modal != modalHelp {
		t.Errorf("modal = %d, want %d", m.modal, modalHelp)
	}

	if cmd != nil {
		t.Error("expected nil cmd for help")
	}
}

func TestHandleKeyMsgCtrlCExecuting(t *testing.T) {
	m := &model{
		executing: true,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd != nil {
		t.Error("expected nil cmd when Ctrl+C during execution")
	}
}

func TestHandleKeyMsgQQCommandOpen(t *testing.T) {
	m := &model{
		commandOpen: true,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	if cmd != nil {
		t.Error("expected nil cmd when q pressed in command mode")
	}
}

func TestHandleKeyMsgQOutputScreen(t *testing.T) {
	m := &model{
		screen: screenOutput,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	if cmd != nil {
		t.Error("expected nil cmd when q pressed in output mode")
	}
}

func TestHandleKeyMsgQWithModal(t *testing.T) {
	m := &model{
		modal: modalHelp,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	if cmd != nil {
		t.Error("expected nil cmd when q pressed with modal")
	}
}

func TestHandleKeyMsgQWithNonHelpModal(t *testing.T) {
	m := &model{
		modal: modalAlert,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	if cmd != nil {
		t.Error("expected nil cmd when q pressed with non-help modal")
	}
}

func TestHandleKeyMsgQQuit(t *testing.T) {
	m := &model{}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	if cmd == nil {
		t.Error("expected non-nil quit cmd when q pressed on main screen")
	}
}

func TestShortcutCmdSuccess(t *testing.T) {
	m := &model{
		repoOrder: []string{"a"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
		persState: PersistentState{History: []string{}},
		output:    viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	cmd := shortcutCmd(m, "status")

	if cmd == nil {
		t.Error("expected non-nil cmd when repos selected")
	}

	if m.screen != screenOutput {
		t.Errorf("screen = %d, want %d", m.screen, screenOutput)
	}
}

func TestShortcutCmdNoSelected(t *testing.T) {
	m := &model{
		repoOrder: []string{},
		selected:  map[string]bool{},
	}

	cmd := shortcutCmd(m, "status")
	if cmd != nil {
		t.Error("expected nil cmd when no repos selected")
	}

	if m.modal != modalAlert {
		t.Errorf("modal = %d, want %d", m.modal, modalAlert)
	}
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

	openGroupPopup(m)

	if m.modal != modalGroup {
		t.Errorf("modal = %d, want %d", m.modal, modalGroup)
	}

	if len(m.groupPopupOptions) != 3 {
		t.Errorf("expected 3 popup options, got %d", len(m.groupPopupOptions))
	}

	if m.groupPopupCursor != 2 {
		t.Errorf("cursor = %d, want 2 (index of work)", m.groupPopupCursor)
	}
}

func TestOpenGroupPopupNoGroupFilter(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Groups: map[string]config.Group{
				"work": {},
			},
		},
	}

	openGroupPopup(m)

	if m.modal != modalGroup {
		t.Errorf("modal = %d, want %d", m.modal, modalGroup)
	}

	if m.groupPopupCursor != 0 {
		t.Errorf("cursor = %d, want 0 (index of [all])", m.groupPopupCursor)
	}
}

func TestTuiGroupPopupNavigation(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(cfgPath, []byte(
		`[repos.repo1]
path = "`+t.TempDir()+`"
backends = ["git"]

[groups.work]
repos = ["repo1"]

[groups.personal]
repos = ["repo1"]
`), 0o644)
	require.NoError(t, err)

	ctx := context.Background()
	m, err := newModel(ctx, Options{
		ConfigPath: cfgPath,
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
	})
	require.NoError(t, err)

	m.width = 80
	m.height = 24
	m.ready = true

	// Open group popup
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: '@'})
	require.Equal(t, modalGroup, m.modal, "modal should be group after pressing @")

	// Move down
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: 'j'})

	// Move up
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: 'k'})

	// Close with Esc
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.Equal(t, modalNone, m.modal, "modal should be none after pressing Esc")
}

func TestTuiCommandBarOpenClose(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(cfgPath, []byte(
		`[repos.repo1]
path = "`+t.TempDir()+`"
backends = ["git"]
`), 0o644)
	require.NoError(t, err)

	ctx := context.Background()
	m, err := newModel(ctx, Options{
		ConfigPath: cfgPath,
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
	})
	require.NoError(t, err)

	m.width = 80
	m.height = 24
	m.ready = true

	// Open git command bar
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: 'G'})
	require.True(t, m.commandOpen, "command bar should be open after pressing G")

	// Close with Esc
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.False(t, m.commandOpen, "command bar should be closed after pressing Esc")
}

func TestHandleOutputKeyEsc(t *testing.T) {
	m := &model{
		screen: screenOutput,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleOutputKey(tea.KeyPressMsg{Code: tea.KeyEsc})

	if m.screen != screenMain {
		t.Errorf("screen = %d, want %d", m.screen, screenMain)
	}

	if cmd != nil {
		t.Error("expected nil cmd when closing output")
	}
}

func TestHandleOutputKeyQ(t *testing.T) {
	m := &model{
		screen: screenOutput,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleOutputKey(tea.KeyPressMsg{Code: 'q'})

	if m.screen != screenMain {
		t.Errorf("screen = %d, want %d", m.screen, screenMain)
	}

	if cmd != nil {
		t.Error("expected nil cmd when closing output with q")
	}
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

	if cmd == nil {
		t.Fatal("expected non-nil cmd after exec result")
	}

	msg := cmd()
	if _, ok := msg.(execDoneMsg); !ok {
		t.Fatalf("expected execDoneMsg from streamNextResult with closed channel, got %T", msg)
	}
}

func TestHandleExecResultError(t *testing.T) {
	m := &model{
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleExecResult(execResultMsg{
		err:  fmt.Errorf("command failed"), //nolint:perfsprint // test readability
		done: true,
	})

	if m.executing {
		t.Error("executing should be false after error")
	}

	if cmd != nil {
		t.Error("expected nil cmd after error")
	}
}

func TestHandleExecDone(t *testing.T) {
	m := &model{
		executing: true,
		output:    viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleExecDone(execDoneMsg{})

	if m.executing {
		t.Error("executing should be false after exec done")
	}

	if cmd != nil {
		t.Error("expected nil cmd after exec done")
	}
}

func TestHandleInputKeyEnterEmptyString(t *testing.T) {
	m := &model{
		commandOpen: true,
	}
	m.initInput()
	m.input.SetValue("   ")

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Error("expected nil cmd for whitespace-only command")
	}
}

func TestHandleInputKeyEnterEmpty(t *testing.T) {
	m := &model{
		commandOpen: true,
	}
	m.initInput()

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Error("expected nil cmd for empty command")
	}
}

func TestHandleInputKeyEnterExecuting(t *testing.T) {
	m := &model{
		commandOpen: true,
		executing:   true,
	}
	m.initInput()
	m.input.SetValue("status")

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Error("expected nil cmd when already executing")
	}
}

func TestHandleInputKeyEsc(t *testing.T) {
	m := &model{
		commandOpen: true,
	}
	m.initInput()

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyEsc})

	if m.commandOpen {
		t.Error("commandOpen should be false after Esc")
	}

	if cmd != nil {
		t.Error("expected nil cmd after Esc in input mode")
	}
}

func TestContentHeightWithCommandOpen(t *testing.T) {
	m := &model{
		height:      10,
		commandOpen: true,
	}
	h := m.contentHeight()

	if h < 3 {
		t.Errorf("contentHeight() = %d, want >= 3", h)
	}
}

func TestContentHeightSmall(t *testing.T) {
	m := &model{
		height: 2,
	}
	h := m.contentHeight()

	if h < 3 {
		t.Errorf("contentHeight() = %d, want >= 3", h)
	}
}

func TestInputWidthNarrow(t *testing.T) {
	m := &model{width: 5}
	w := m.inputWidth()

	if w < minInputWidth {
		t.Errorf("inputWidth() = %d, want >= %d", w, minInputWidth)
	}
}

func TestHandleWindowSize(t *testing.T) {
	m, err := newModel(context.Background(), Options{})
	require.NoError(t, err)

	m.initTable()

	_, cmd := m.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 30})

	if !m.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}

	if m.width != 100 {
		t.Errorf("width = %d, want 100", m.width)
	}

	if m.height != 30 {
		t.Errorf("height = %d, want 30", m.height)
	}

	cols := m.repoTable.Columns()
	if len(cols) != 4 {
		t.Errorf("expected 4 columns, got %d", len(cols))
	}

	if cols[3].Width <= 0 {
		t.Errorf("STATUS column width should be > 0, got %d", cols[3].Width)
	}

	if cmd != nil {
		t.Error("expected nil cmd after WindowSizeMsg")
	}
}
