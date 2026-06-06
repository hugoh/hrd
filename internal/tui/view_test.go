package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
)

func TestMainView(t *testing.T) {
	m := testModel()

	view := m.mainView()
	if view == "" {
		t.Fatal("mainView() returned empty")
	}
}

// testModel creates a model with default test configuration ready for View().
// opts can override fields before initTable() is called.
func testModel(opts ...func(*model)) *model {
	m := &model{
		screen: screenMain,
		width:  80,
		height: 30,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	for _, o := range opts {
		o(m)
	}

	m.ready = true
	m.initTable()

	return m
}

func TestViewHelpScreen(t *testing.T) {
	m := testModel(func(m *model) {
		m.screen = screenHelp
		m.initHelpViewport()
	})

	view := m.View()
	if view.Content == "" {
		t.Fatal("View() with help screen returned empty")
	}

	if !view.AltScreen {
		t.Error("View() should have AltScreen = true")
	}

	if view.MouseMode != tea.MouseModeCellMotion {
		t.Error("View() should have MouseModeCellMotion")
	}
}

func TestViewNotReady(t *testing.T) {
	m := &model{
		screen: screenMain,
		width:  80,
		height: 30,
	}

	m.ready = false

	view := m.View()
	if view.Content != "" {
		t.Errorf("View() should return empty content when not ready, got %q", view.Content)
	}
}

func TestViewGroupScreen(t *testing.T) {
	m := testModel(func(m *model) {
		m.screen = screenGroup
		m.initGroupList()
		m.groupList.SetItems([]list.Item{groupItem{name: labelAllRepos}})
		m.groupList.SetWidth(m.width)
		m.groupList.SetHeight(m.contentHeight())
	})

	view := m.View()
	if view.Content == "" {
		t.Fatal("View() with group screen returned empty")
	}
}

func TestViewAlertInline(t *testing.T) {
	m := testModel(func(m *model) { m.modal = modalAlert })

	view := m.View()
	if view.Content == "" {
		t.Fatal("View() with alert returned empty")
	}
}

func TestViewOutputScreen(t *testing.T) {
	m := &model{
		screen: screenOutput,
		width:  80,
		height: 30,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.ready = true

	view := m.View()
	if view.Content == "" {
		t.Fatal("View() on output screen returned empty")
	}
}

func TestOutputView(t *testing.T) {
	m := &model{
		screen: screenOutput,
		width:  80,
		height: 30,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.ready = true

	view := m.outputView()
	if view == "" {
		t.Fatal("outputView() returned empty")
	}

	if !strings.Contains(view, "Output") {
		t.Errorf("outputView should contain 'Output', got %q", view)
	}
}

func TestOutputViewExecuting(t *testing.T) {
	m := &model{
		screen:    screenOutput,
		width:     80,
		height:    30,
		executing: true,
		execTotal: 5,
		execResults: []execResult{
			{name: "alpha", result: runner.Result{RepoName: "alpha"}},
			{name: "beta", result: runner.Result{RepoName: "beta"}},
		},
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.ready = true

	view := m.outputView()

	if !strings.Contains(view, "[2/5]") {
		t.Errorf("outputView should show progress [2/5], got %q", view)
	}

	if !strings.Contains(view, "Esc/q:close") {
		t.Errorf("outputView should show Esc/q:close, got %q", view)
	}
}

func TestRenderInputLine(t *testing.T) {
	m := &model{
		cmdPrefix: prefixGit,
	}
	m.initInput()

	line := m.renderInputLine()
	if !strings.Contains(line, "[git]") {
		t.Errorf("expected [git] prompt, got %q", line)
	}
}

func TestRenderInputLineShell(t *testing.T) {
	m := &model{
		cmdPrefix: prefixShell,
	}
	m.initInput()

	line := m.renderInputLine()
	if !strings.Contains(line, "[sh]") {
		t.Errorf("expected [sh] prompt, got %q", line)
	}
}

func TestEmptyTableViewNoRepos(t *testing.T) {
	m := testModel()

	view := m.mainView()

	if !strings.Contains(view, "No repos configured") {
		t.Errorf("empty mainView() should contain 'No repos configured', got %q", view)
	}

	if !strings.Contains(view, "hrd repo add") {
		t.Errorf("empty mainView() should mention 'hrd repo add', got %q", view)
	}
}

func TestEmptyTableViewNothingSelected(t *testing.T) {
	m := testModel(func(m *model) {
		m.repoOrder = []string{"a", "b"}
		m.selected = map[string]bool{}
	})

	view := m.mainView()

	if !strings.Contains(view, "No repos selected") {
		t.Errorf("empty mainView() should contain empty state message, got %q", view)
	}

	if !strings.Contains(view, "@") {
		t.Errorf("empty mainView() should mention @ key, got %q", view)
	}

	if !strings.Contains(view, "x") {
		t.Errorf("empty mainView() should mention x key, got %q", view)
	}
}

func TestRenderHeaderSingleMode(t *testing.T) {
	m := testModel(func(m *model) {
		m.mode = modeSingle
		m.repoOrder = []string{"a"}
		m.selected = map[string]bool{"a": true}
	})

	view := m.mainView()
	if !strings.Contains(view, "s:single") {
		t.Errorf("mainView() should show 's:single' indicator in single mode, got %q", view)
	}
}

func TestHelpContent(t *testing.T) {
	m := &model{}
	content := m.helpContent()

	if content == "" {
		t.Fatal("helpContent() returned empty")
	}

	if !strings.Contains(content, "Navigation") {
		t.Errorf("helpContent() missing Navigation, got %q", content)
	}
}

func TestAlertContent(t *testing.T) {
	m := &model{}

	// Default when no alertMsg
	content := m.alertContent()
	if !strings.Contains(content, "No repos selected") {
		t.Error("default alertContent should mention 'No repos selected'")
	}

	// Custom alertMsg
	m.alertMsg = "Custom warning message"

	content = m.alertContent()
	if !strings.Contains(content, "Custom warning message") {
		t.Error("alertContent with alertMsg should return the custom message")
	}
}

func TestRepoCountLabel(t *testing.T) {
	if got := repoCountLabel(1); got != "1 repo" {
		t.Errorf("repoCountLabel(1) = %q, want %q", got, "1 repo")
	}

	if got := repoCountLabel(3); got != "3 repos" {
		t.Errorf("repoCountLabel(3) = %q, want %q", got, "3 repos")
	}
}

func TestSelHistoryView(t *testing.T) {
	m := &model{
		screen: screenSelHistory,
		width:  80,
		height: 30,
		cfg: config.Config{
			Groups: map[string]config.Group{},
			Repos:  map[string]config.Repo{"a": {}, "b": {}, "c": {}, "d": {}},
		},
		persState: PersistentState{
			SelectionHistory: []SelectionEntry{
				{Repos: []string{"a", "b", "c"}},
				{Repos: []string{"d"}},
			},
		},
	}
	m.initTable()
	m.initHistoryList()

	view := m.selHistoryView()
	if view == "" {
		t.Fatal("selHistoryView() returned empty")
	}

	if !strings.Contains(view, "Selection History") {
		t.Error("selHistoryView should show 'Selection History' header")
	}
}

func TestSelHistoryViewEmpty(t *testing.T) {
	m := &model{
		persState: PersistentState{SelectionHistory: []SelectionEntry{}},
	}

	view := m.selHistoryView()
	if view != "" {
		t.Error("selHistoryView with no entries should be empty")
	}
}
