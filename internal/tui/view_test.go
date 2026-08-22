package tui

import (
	"testing"
	"time"

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

// outputScreenModel returns a ready model on screenOutput, sized like
// testModel's default, for tests that exercise the output screen directly.
func outputScreenModel() *model {
	m := &model{
		screen: screenOutput,
		width:  80,
		height: 30,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.ready = true

	return m
}

func TestViewOutputScreen(t *testing.T) {
	m := outputScreenModel()

	require.NotEmpty(t, m.View().Content)
}

func TestOutputView(t *testing.T) {
	m := outputScreenModel()

	view := m.outputView()
	require.NotEmpty(t, view)
	assert.Contains(t, view, "Output")
}

func TestOutputViewWithLabel(t *testing.T) {
	tests := []struct {
		name      string
		execLabel string
		want      string
	}{
		{"vcs shortcut", "status", "status"},
		{"git command", "git diff HEAD", "git diff HEAD"},
		{"shell command", "sh ls -la", "sh ls -la"},
		{"no label falls back", "", "Output"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				screen:    screenOutput,
				width:     80,
				height:    30,
				execLabel: tt.execLabel,
				output:    viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
			}
			m.ready = true

			assert.Contains(t, m.outputView(), tt.want)
		})
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
	assert.Contains(t, view, "[2/5]")
	assert.Contains(t, view, "Esc/q:close")
}

func TestOutputViewExecuting_LiveCounts(t *testing.T) {
	m := &model{
		screen:    screenOutput,
		width:     80,
		height:    30,
		executing: true,
		execTotal: 4,
		execResults: []execResult{
			{name: "alpha", result: runner.Result{RepoName: "alpha"}},
			{name: "beta", result: runner.Result{RepoName: "beta", ExitCode: 1}},
		},
		execStartTime: time.Now().Add(-2 * time.Second),
		output:        viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.ready = true

	view := m.outputView()
	assert.Contains(t, view, "✓1")
	assert.Contains(t, view, "✗1")
	assert.Contains(t, view, "ETA")
}

func TestOutputViewExecuting_NoETABeforeFirstResult(t *testing.T) {
	m := &model{
		screen:        screenOutput,
		width:         80,
		height:        30,
		executing:     true,
		execTotal:     5,
		execStartTime: time.Now(),
		output:        viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.ready = true

	assert.NotContains(t, m.outputView(), "ETA")
}

func TestExecCounts(t *testing.T) {
	m := &model{
		execResults: []execResult{
			{name: "a", result: runner.Result{ExitCode: 0}},
			{name: "b", result: runner.Result{ExitCode: 1}},
			{name: "c", result: runner.Result{Err: assert.AnError}},
		},
	}

	succeeded, failed := m.execCounts()
	assert.Equal(t, 1, succeeded)
	assert.Equal(t, 2, failed)
}

func TestExecETA(t *testing.T) {
	t.Run("no estimate before any result", func(t *testing.T) {
		m := &model{execTotal: 5, execStartTime: time.Now()}
		_, ok := m.execETA(0)
		assert.False(t, ok)
	})

	t.Run("no estimate once complete", func(t *testing.T) {
		m := &model{execTotal: 5, execStartTime: time.Now()}
		_, ok := m.execETA(5)
		assert.False(t, ok)
	})

	t.Run("no estimate with zero start time", func(t *testing.T) {
		m := &model{execTotal: 5}
		_, ok := m.execETA(2)
		assert.False(t, ok)
	})

	t.Run("extrapolates from elapsed time", func(t *testing.T) {
		m := &model{execTotal: 4, execStartTime: time.Now().Add(-4 * time.Second)}
		eta, ok := m.execETA(2)
		require.True(t, ok)
		assert.InDelta(t, 4*time.Second, eta, float64(time.Second))
	})
}

func TestFormatETA(t *testing.T) {
	for _, tt := range []struct {
		name string
		d    time.Duration
		want string
	}{
		{"seconds only", 8 * time.Second, "0:08"},
		{"minutes and seconds", 83 * time.Second, "1:23"},
		{"rounds to nearest second", 8*time.Second + 600*time.Millisecond, "0:09"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatETA(tt.d))
		})
	}
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
