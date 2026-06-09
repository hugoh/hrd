package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainView(t *testing.T) {
	m := testModel()

	require.NotEmpty(t, m.mainView())
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
	require.NotEmpty(t, view.Content)
	assert.True(t, view.AltScreen)
	assert.Equal(t, tea.MouseModeCellMotion, view.MouseMode)
}

func TestViewNotReady(t *testing.T) {
	m := &model{
		screen: screenMain,
		width:  80,
		height: 30,
	}
	m.ready = false

	assert.Empty(t, m.View().Content)
}

func TestViewGroupScreen(t *testing.T) {
	m := testModel(func(m *model) {
		m.screen = screenGroup
		m.initGroupList()
		m.groupList.SetItems([]list.Item{groupItem{name: labelAllRepos}})
		m.groupList.SetWidth(m.width)
		m.groupList.SetHeight(m.contentHeight())
	})

	require.NotEmpty(t, m.View().Content)
}

func TestViewAlertInline(t *testing.T) {
	m := testModel(func(m *model) { m.modal = modalAlert })

	require.NotEmpty(t, m.View().Content)
}

func TestViewOutputScreen(t *testing.T) {
	m := &model{
		screen: screenOutput,
		width:  80,
		height: 30,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.ready = true

	require.NotEmpty(t, m.View().Content)
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
	require.NotEmpty(t, view)
	assert.Contains(t, view, "Output")
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
	assert.Contains(t, view, "[2/5]")
	assert.Contains(t, view, "Esc/q:close")
}

func TestRenderInputLine(t *testing.T) {
	m := &model{}
	m.initInput()

	assert.Contains(t, m.renderInputLine(), ":")
}

func TestEmptyTableViewNoRepos(t *testing.T) {
	m := testModel()

	view := m.mainView()
	assert.Contains(t, view, "No repos configured")
	assert.Contains(t, view, "hrd repo add")
}

func TestEmptyTableViewNothingSelected(t *testing.T) {
	m := testModel(func(m *model) {
		m.repoOrder = []string{"a", "b"}
		m.selected = map[string]bool{}
	})

	view := m.mainView()
	assert.Contains(t, view, "No repos selected")
	assert.Contains(t, view, "@")
	assert.Contains(t, view, "x")
}

func TestRenderHeaderSingleMode(t *testing.T) {
	m := testModel(func(m *model) {
		m.mode = modeSingle
		m.repoOrder = []string{"a"}
		m.selected = map[string]bool{"a": true}
	})

	assert.Contains(t, m.mainView(), "s:single")
}

func TestHelpContent(t *testing.T) {
	m := &model{}
	content := m.helpContent()

	require.NotEmpty(t, content)
	assert.Contains(t, content, "Navigation")
}

func TestAlertContent(t *testing.T) {
	m := &model{}

	content := m.alertContent()
	assert.Contains(t, content, "No repos selected")

	m.alertMsg = "Custom warning message"
	content = m.alertContent()
	assert.Contains(t, content, "Custom warning message")
}

func TestRepoCountLabel(t *testing.T) {
	assert.Equal(t, "1 repo", repoCountLabel(1))
	assert.Equal(t, "3 repos", repoCountLabel(3))
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
	require.NotEmpty(t, view)
	assert.Contains(t, view, "Selection History")
}

func TestSelHistoryViewEmpty(t *testing.T) {
	m := &model{
		persState: PersistentState{SelectionHistory: []SelectionEntry{}},
	}

	assert.Empty(t, m.selHistoryView())
}
