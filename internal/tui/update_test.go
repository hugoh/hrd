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
	if m.mode != modeSelect {
		t.Error("mode should be modeSelect after first toggle")
	}

	_, _ = m.handleSelectToggle()
	if m.mode == modeSelect {
		t.Error("mode should be modeNormal after second toggle")
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

func TestMainKeySpaceTogglesSelectMode(t *testing.T) {
	m := &model{
		repoOrder: []string{"a"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
		ready:     true,
	}
	m.cursor = 0
	m.initTable()

	_, cmd := m.handleMainKey(tea.KeyPressMsg{Code: ' '})
	if m.mode != modeSelect {
		t.Error("mode should be modeSelect after space (entered select mode)")
	}

	if m.mode == modeSingle {
		t.Error("mode should not be modeSingle after entering select mode")
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: ' '})
	if m.mode != modeSelect {
		t.Error("mode should remain modeSelect after another space (stays in select mode)")
	}

	if m.selected["a"] {
		t.Error("repo 'a' should be deselected after space in select mode")
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode == modeSelect {
		t.Error("mode should be modeNormal after enter (exited select mode)")
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestMainKeySpaceSelectsOne(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{"a": true, "b": false},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
		mode:      modeSelect,
		ready:     true,
	}
	m.cursor = 0
	m.initTable()

	_, cmd := m.handleMainKey(tea.KeyPressMsg{Code: ' '})
	if m.selected["a"] {
		t.Error("repo 'a' should be deselected after space in select mode")
	}

	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: ' '})
	if !m.selected["b"] {
		t.Error("repo 'b' should be selected after second space")
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestMainKeyXSingleToggle(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
		ready:     true,
	}
	m.cursor = 0
	m.initTable()

	_, cmd := m.handleMainKey(tea.KeyPressMsg{Code: 'x'})
	if m.mode != modeSingle {
		t.Error("mode should be modeSingle after pressing x")
	}

	if m.mode == modeSelect {
		t.Error("mode should not be modeSelect after entering single mode")
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: 'x'})
	if m.mode == modeSingle {
		t.Error("mode should be modeNormal after pressing x again")
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestMainKeyXSingleModeSelectsOne(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{"a": true, "b": false},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
		ready:     true,
	}
	m.cursor = 0
	m.initTable()
	m.mode = modeSingle

	names := m.selectedNames()
	if len(names) != 1 {
		t.Fatalf("selectedNames() = %v, want [a]", names)
	}

	if names[0] != "a" {
		t.Errorf("selectedNames()[0] = %q, want %q", names[0], "a")
	}
}

func TestEscExitsSelectMode(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
		ready:     true,
	}
	m.cursor = 0
	m.initTable()
	m.mode = modeSelect

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.mode == modeSelect {
		t.Error("mode should be modeNormal after esc")
	}

	if m.mode == modeSingle {
		t.Error("mode should not be modeSingle after esc")
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestEscExitsSingleMode(t *testing.T) {
	m := &model{
		repoOrder: []string{"a"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
		ready:     true,
	}
	m.cursor = 0
	m.initTable()
	m.mode = modeSingle

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.mode == modeSingle {
		t.Error("mode should be modeNormal after esc")
	}

	if m.mode == modeSelect {
		t.Error("mode should not be modeSelect after esc")
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestHandleSingleToggle(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{"a": true},
		cfg:       config.Config{Settings: config.Settings{Concurrency: 1}},
	}
	m.cursor = 0
	m.initTable()

	_, _ = m.handleSingleToggle()
	if m.mode != modeSingle {
		t.Error("mode should be modeSingle after first toggle")
	}

	_, _ = m.handleSingleToggle()
	if m.mode == modeSingle {
		t.Error("mode should be modeNormal after second toggle")
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

	if m.modal != modalNone {
		t.Error("alert should be cleared after any key")
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

	if m.screen != screenGroup {
		t.Errorf("screen should be screenGroup after @ in alert, got %v", m.screen)
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

	if m.screen != screenMain {
		t.Errorf("screen = %d, want %d", m.screen, screenMain)
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
	m.initHelpViewport()

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: '?'})

	if m.screen != screenHelp {
		t.Errorf("screen = %d, want %d", m.screen, screenHelp)
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

func TestHandleKeyMsgQCommandOpen(t *testing.T) {
	m := &model{
		commandOpen: true,
	}
	m.initInput()
	m.input.Focus()

	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: 'q', Text: "q"})

	if got := m.input.Value(); got != "q" {
		t.Errorf("expected input value to be 'q', got %q", got)
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

func TestHandleKeyMsgQWithHelpScreen(t *testing.T) {
	m := &model{
		screen: screenHelp,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	if cmd != nil {
		t.Error("expected nil cmd when q pressed on help screen")
	}

	if m.screen != screenMain {
		t.Error("should return to main screen after q on help")
	}
}

func TestHandleKeyMsgQWithAlert(t *testing.T) {
	m := &model{
		modal: modalAlert,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	if cmd != nil {
		t.Error("expected nil cmd when q pressed with alert")
	}

	if m.modal != modalNone {
		t.Error("alert should be dismissed on q")
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
		persState: PersistentState{History: []HistoryEntry{}},
		output:    viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	cmd := shortcutCmd(m, "status", false)

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

	cmd := shortcutCmd(m, "status", false)
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

	openGroupPopup(m, groupFilterMode)

	if m.screen != screenGroup {
		t.Errorf("screen = %d, want %d", m.screen, screenGroup)
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

	openGroupPopup(m, groupFilterMode)

	if m.screen != screenGroup {
		t.Errorf("screen = %d, want %d", m.screen, screenGroup)
	}

	if len(m.groupPopupOptions) != 2 {
		t.Errorf("expected 2 popup options, got %d", len(m.groupPopupOptions))
	}

	if m.groupPopupCursor != 0 {
		t.Errorf("cursor = %d, want 0 (index of [all])", m.groupPopupCursor)
	}
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

func TestHandleExecDoneWithSideEffect(t *testing.T) {
	m := &model{
		executing:      true,
		execSideEffect: true,
		output:         viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleExecDone(execDoneMsg{})

	if m.executing {
		t.Error("executing should be false after exec done")
	}

	if m.execSideEffect {
		t.Error("execSideEffect should be reset after exec done")
	}

	if cmd == nil {
		t.Error("expected non-nil cmd (refresh) when execSideEffect is true")
	}
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

	openGroupPopup(m, groupAddMode)

	if m.screen != screenGroup {
		t.Errorf("screen = %d, want %d", m.screen, screenGroup)
	}

	if len(m.groupPopupOptions) != 3 {
		t.Errorf("expected 3 popup options (2 groups + new), got %d", len(m.groupPopupOptions))
	}

	if m.groupPopupOptions[2] != labelNew {
		t.Errorf("last option = %q, want %q", m.groupPopupOptions[2], labelNew)
	}

	if m.groupMode != groupAddMode {
		t.Errorf("groupMode = %d, want %d", m.groupMode, groupAddMode)
	}
}

func TestHandleGroupEnterFilterModeAll(t *testing.T) {
	m := &model{
		ctx: context.Background(),
		cfg: config.Config{
			Repos:  map[string]config.Repo{"r1": {Groups: []string{"work"}}},
			Groups: map[string]config.Group{"work": {Repos: []string{"r1"}}},
		},
		repoOrder:         []string{"r1"},
		selected:          map[string]bool{},
		groupFilter:       "work",
		groupPopupOptions: []string{labelAllCap, "work"},
	}

	_, cmd := m.handleGroupEnter()

	if m.groupFilter != "" {
		t.Errorf("groupFilter = %q, want empty", m.groupFilter)
	}

	if m.screen != screenMain {
		t.Errorf("screen = %d, want %d", m.screen, screenMain)
	}

	if cmd == nil {
		t.Error("expected non-nil cmd (refresh) after group selection")
	}
}

func TestHandleGroupEnterAddModeExistingGroup(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")

	m := &model{
		ctx: context.Background(),
		cfg: config.Config{
			Repos: map[string]config.Repo{
				"repo1": {Groups: []string{"work"}},
				"repo2": {},
			},
			Groups: map[string]config.Group{"work": {Repos: []string{"repo1"}}},
		},
		opts:              Options{ConfigPath: cfgPath},
		repoOrder:         []string{"repo1", "repo2"},
		selected:          map[string]bool{"repo2": true},
		groupPopupOptions: []string{"work", labelNew},
		groupPopupCursor:  0,
		groupMode:         groupAddMode,
	}

	_, cmd := m.handleGroupEnter()

	if cmd != nil {
		t.Error("expected nil cmd after adding to existing group")
	}

	if m.screen != screenMain {
		t.Errorf("screen = %d, want %d", m.screen, screenMain)
	}

	if len(m.cfg.Groups["work"].Repos) != 2 {
		t.Errorf("expected 2 repos in work group, got %v", m.cfg.Groups["work"].Repos)
	}
}

func TestHandleGroupEnterAddModeNew(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Repos: map[string]config.Repo{"repo1": {}},
		},
		repoOrder:         []string{"repo1"},
		selected:          map[string]bool{"repo1": true},
		groupPopupOptions: []string{"work", labelNew},
		groupPopupCursor:  1,
		groupMode:         groupAddMode,
	}
	m.initInput()

	_, cmd := m.handleGroupEnter()

	if cmd != nil {
		t.Error("expected nil cmd when entering new group name")
	}

	if !m.groupNewInput {
		t.Error("groupNewInput should be true after selecting [new...]")
	}
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

	if cmd != nil {
		t.Error("expected nil cmd after creating new group")
	}

	if m.groupNewInput {
		t.Error("groupNewInput should be false after Enter")
	}

	if m.screen != screenMain {
		t.Errorf("screen = %d, want %d", m.screen, screenMain)
	}

	if _, ok := m.cfg.Groups["my-group"]; !ok {
		t.Error("my-group should exist in config after creation")
	}
}

func TestHandleGroupNewInputEmpty(t *testing.T) {
	m := &model{
		groupNewInput: true,
	}
	m.initInput()

	_, cmd := m.handleGroupNewInput(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Error("expected nil cmd for empty group name")
	}

	if !m.groupNewInput {
		t.Error("groupNewInput should remain true when name is empty")
	}
}

func TestHandleGroupNewInputEsc(t *testing.T) {
	m := &model{
		groupNewInput: true,
	}
	m.initInput()

	_, cmd := m.handleGroupNewInput(tea.KeyPressMsg{Code: tea.KeyEsc})

	if cmd != nil {
		t.Error("expected nil cmd after Esc")
	}

	if m.groupNewInput {
		t.Error("groupNewInput should be false after Esc")
	}
}

func TestHandleGroupKeyNavigation(t *testing.T) {
	m := &model{
		groupPopupOptions: []string{"a", "b", "c"},
		groupPopupCursor:  1,
	}

	// Press up
	_, cmd := m.handleGroupKey(tea.KeyPressMsg{Code: 'k'})

	if cmd != nil {
		t.Error("expected nil cmd after up")
	}

	if m.groupPopupCursor != 0 {
		t.Errorf("cursor = %d, want 0 after up", m.groupPopupCursor)
	}

	// Press down
	_, cmd = m.handleGroupKey(tea.KeyPressMsg{Code: 'j'})

	if cmd != nil {
		t.Error("expected nil cmd after down")
	}

	if m.groupPopupCursor != 1 {
		t.Errorf("cursor = %d, want 1 after down", m.groupPopupCursor)
	}

	// Up at top clamps to 0
	m.groupPopupCursor = 0
	m.handleGroupKey(tea.KeyPressMsg{Code: 'k'})

	if m.groupPopupCursor != 0 {
		t.Errorf("cursor = %d, want 0 at top", m.groupPopupCursor)
	}

	// Down at bottom clamps to len-1
	m.groupPopupCursor = 2
	m.handleGroupKey(tea.KeyPressMsg{Code: 'j'})

	if m.groupPopupCursor != 2 {
		t.Errorf("cursor = %d, want 2 at bottom", m.groupPopupCursor)
	}
}

func TestHandleGroupKeyEnterOnAllFilterMode(t *testing.T) {
	m := &model{
		ctx: context.Background(),
		cfg: config.Config{
			Repos:  map[string]config.Repo{"r1": {}},
			Groups: map[string]config.Group{"work": {Repos: []string{"r1"}}},
		},
		repoOrder:         []string{"r1"},
		selected:          map[string]bool{},
		groupPopupOptions: []string{labelAllCap, "work"},
	}

	_, cmd := m.handleGroupKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.groupFilter != "" {
		t.Errorf("groupFilter = %q, want empty", m.groupFilter)
	}

	if m.screen != screenMain {
		t.Errorf("screen = %d, want %d", m.screen, screenMain)
	}

	if cmd == nil {
		t.Error("expected non-nil cmd (refresh) after group selection")
	}
}
