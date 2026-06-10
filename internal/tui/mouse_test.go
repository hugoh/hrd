package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestHandleMouseClickPositionsCursor(t *testing.T) {
	m := readyModel(
		[]string{"a", "b", "c", "d"},
		map[string]bool{"a": true, "b": true, "c": true, "d": true},
	)

	_, cmd := m.Update(mouseClick(5))
	assert.Equal(t, 2, m.cursor, "cursor should move to row 2 (Y=5 minus 3)")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickSelectModeTogglesSelection(t *testing.T) {
	m := readyModel([]string{"a", "b"}, map[string]bool{"a": true, "b": false})
	m.mode = modeSelect

	m.Update(mouseClick(3))
	assert.False(t, m.selected["a"], "repo 'a' should be deselected after click on first data row")
	assert.False(
		t,
		m.selected["b"],
		"repo 'b' should remain deselected (cursor moved to a, b not toggled)",
	)
	assert.Equal(t, 0, m.cursor, "cursor should be on repo a (row 0)")

	m.Update(mouseClick(4))
	assert.False(t, m.selected["a"], "repo 'a' should remain deselected")
	assert.True(t, m.selected["b"], "repo 'b' should be toggled on after click")
	assert.Equal(t, 1, m.cursor, "cursor should be on repo b (row 1)")
}

func TestHandleMouseClickAboveTableReturnsEarly(t *testing.T) {
	m := readyModel([]string{"a"}, map[string]bool{"a": true})

	t.Run("app header", func(t *testing.T) {
		_, cmd := m.Update(mouseClick(0))
		assert.Equal(t, 0, m.cursor, "cursor should stay at 0")
		assert.Nil(t, cmd, "expected nil cmd")
	})

	t.Run("separator", func(t *testing.T) {
		_, cmd := m.Update(mouseClick(1))
		assert.Equal(t, 0, m.cursor, "cursor should stay at 0")
		assert.Nil(t, cmd, "expected nil cmd")
	})

	t.Run("table header", func(t *testing.T) {
		_, cmd := m.Update(mouseClick(2))
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
	m := readyModel([]string{"a", "b"}, map[string]bool{"a": true, "b": true})

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

func TestHandleMouseClickNonLeftButtonReturnsEarly(t *testing.T) {
	m := readyModel([]string{"a"}, map[string]bool{"a": true})

	_, cmd := m.Update(tea.MouseClickMsg{X: 0, Y: 3, Button: tea.MouseRight})
	assert.Equal(t, 0, m.cursor, "cursor should not move on right-click")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickEmptyTableReturnsEarly(t *testing.T) {
	m := readyModel(nil, nil)

	_, cmd := m.Update(mouseClick(5))
	assert.Equal(t, 0, m.cursor, "cursor should stay at 0 on empty table")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickBelowRowsReturnsEarly(t *testing.T) {
	m := readyModel([]string{"a"}, map[string]bool{"a": true})

	_, cmd := m.Update(mouseClick(99))
	assert.Equal(t, 0, m.cursor, "cursor should not move when clicking below rows")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickGroupScreen(t *testing.T) {
	m := readyModel([]string{"a"}, map[string]bool{"a": true})
	m.screen = screenGroup
	initGroupTestList(m, []list.Item{
		groupItem{name: "all"},
		groupItem{name: "work"},
	})

	_, cmd := m.Update(mouseClick(5))
	assert.Equal(t, 1, m.groupList.Index(), "should select second item on group screen")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickGroupScreenNewInputDoesNothing(t *testing.T) {
	m := readyModel([]string{"a"}, map[string]bool{"a": true})
	m.screen = screenGroup
	m.groupNewInput = true
	initGroupTestList(m, []list.Item{groupItem{name: "all"}})
	m.groupList.Select(1)

	_, cmd := m.Update(mouseClick(3))
	assert.Equal(t, 1, m.groupList.Index(), "index should not change when groupNewInput")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickHistoryScreen(t *testing.T) {
	m := readyModel([]string{"a", "b"}, map[string]bool{"a": true})
	m.screen = screenSelHistory
	m.initHistoryList()
	m.historyList.SetItems([]list.Item{
		historyItem{repos: []string{"a", "b"}},
		historyItem{repos: []string{"a"}},
	})
	m.historyList.Paginator.PerPage = 10

	_, cmd := m.Update(mouseClick(5))
	assert.Equal(t, 1, m.historyList.Index(), "should select second history item")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickInputLinePositionsCursor(t *testing.T) {
	m := readyModel([]string{"a"}, map[string]bool{"a": true})
	m.commandOpen = true
	m.initInput()
	m.input.SetValue("hello world")
	m.input.SetCursor(0)
	m.height = 10

	_, cmd := m.Update(mouseClickAt(1, 7))
	assert.Equal(t, 0, m.input.Position(), "cursor at col 1 → position 0")
	assert.Nil(t, cmd, "expected nil cmd")

	_, cmd = m.Update(mouseClickAt(6, 7))
	assert.Equal(
		t,
		3,
		m.input.Position(),
		"cursor at col 6 → position 3 (col 6 minus prompt ':> ')",
	)
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickFooterOpensCommandBar(t *testing.T) {
	m := readyModel([]string{"a"}, map[string]bool{"a": true})
	m.height = 10
	m.initInput()

	_, cmd := m.Update(mouseClick(m.height - 1))
	assert.True(t, m.commandOpen, "command bar should open after footer click")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickSeparatorOpensCommandBar(t *testing.T) {
	m := readyModel([]string{"a"}, map[string]bool{"a": true})
	m.height = 10
	m.initInput()

	_, cmd := m.Update(mouseClick(m.height - 2))
	assert.True(t, m.commandOpen, "command bar should open after separator click")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleMouseClickFooterNoOpWhenCommandOpen(t *testing.T) {
	m := readyModel([]string{"a"}, map[string]bool{"a": true})
	m.commandOpen = true
	m.initInput()
	m.height = 10

	_, cmd := m.Update(mouseClick(m.height - 2))
	assert.True(t, m.commandOpen, "command bar should stay open")
	assert.Nil(t, cmd, "expected nil cmd when already open")
}

func TestSelectListRowBounds(t *testing.T) {
	l := list.New([]list.Item{
		groupItem{name: "a"},
		groupItem{name: "b"},
	}, list.NewDefaultDelegate(), 0, 0)
	l.Paginator.PerPage = 10

	selectListRow(&l, 0)
	assert.Equal(t, 0, l.Index(), "click at Y=0 (above list) should not change index")

	selectListRow(&l, 99)
	assert.Equal(t, 0, l.Index(), "click at Y=99 (below list) should not change index")
}
