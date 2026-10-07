package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	assert.NotNil(t, cmd, "selecting persists state via a save cmd")

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
	assert.NotNil(t, cmd, "selecting persists state via a save cmd")

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: 'x'})
	assert.True(t, m.selected["b"], "repo 'b' should be selected after second x")
	assert.NotNil(t, cmd, "selecting persists state via a save cmd")
}

func TestMainKeySpaceSelectsOne(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true, "b": false})
	m.mode = modeSelect
	m.ready = true

	_, cmd := m.handleMainKey(tea.KeyPressMsg{Code: ' '})
	assert.False(t, m.selected["a"], "repo 'a' should be deselected after space in select mode")
	assert.Equal(t, 1, m.cursor, "cursor should advance after space")
	assert.NotNil(t, cmd, "selecting persists state via a save cmd")

	_, cmd = m.handleMainKey(tea.KeyPressMsg{Code: ' '})
	assert.True(t, m.selected["b"], "repo 'b' should be selected after second space")
	assert.NotNil(t, cmd, "selecting persists state via a save cmd")
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

func TestCursorStaysVisibleAtBottom(t *testing.T) {
	names := make([]string, 30)
	sel := map[string]bool{}

	for i := range names {
		names[i] = fmt.Sprintf("repo%02d", i)
		sel[names[i]] = true
	}

	m := baseModel(names, sel)
	m.ready = true
	m.repoTable.SetHeight(8)
	m.toggleMode(modeSingle, false)

	for range names {
		m.handleCursorDown()
		require.Contains(t, m.repoTable.View(), names[m.cursor])
	}

	for range names {
		m.handleCursorUp()
		require.Contains(t, m.repoTable.View(), names[m.cursor])
	}
}

func TestEscOnOutputScreenKeepsMode(t *testing.T) {
	m := baseModel([]string{"a", "b"}, map[string]bool{"a": true, "b": true})
	m.ready = true
	m.mode = modeSingle
	m.screen = screenOutput

	m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})

	assert.Equal(t, screenMain, m.screen)
	assert.Equal(t, modeSingle, m.mode)
}

func TestSelectedRowKeepsBackgroundAfterInnerReset(t *testing.T) {
	m := baseModel([]string{"a"}, map[string]bool{"a": true})
	m.mode = modeSingle
	m.repoTable.SetStyles(tableStyles(true, true))
	m.repoTable.SetColumns([]table.Column{{Width: 2}, {Width: 4}, {Width: 3}, {Width: 20}})
	m.repoTable.SetRows([]table.Row{{"", "a", "git", "\x1b[31mred\x1b[0m tail"}})

	bg := lipgloss.NewStyle().
		Background(lipgloss.Color(theme.SelectionBackground.Resolve(true))).
		Render("x")
	bg, _, _ = strings.Cut(bg, "x")

	require.Contains(t, m.repoTable.View(), "\x1b[0m"+bg)
}

func TestPageUpDownMovesByTableHeight(t *testing.T) {
	names := make([]string, 30)
	sel := map[string]bool{}

	for i := range names {
		names[i] = fmt.Sprintf("repo%02d", i)
		sel[names[i]] = true
	}

	m := baseModel(names, sel)
	m.repoTable.SetHeight(8)
	m.updateTableRows()
	h := m.repoTable.Height()

	m.handleMainKey(tea.KeyPressMsg{Code: tea.KeyPgDown})
	assert.Equal(t, 2*h-1, m.cursor, "cursor lands on the last row of the next page")
	require.Contains(t, m.repoTable.View(), names[m.cursor])

	for range 10 {
		m.handleMainKey(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}

	assert.Equal(t, 29, m.cursor)
	require.Contains(t, m.repoTable.View(), names[29])

	for range 10 {
		m.handleMainKey(tea.KeyPressMsg{Code: tea.KeyPgUp})
	}

	assert.Equal(t, 0, m.cursor)
	require.Contains(t, m.repoTable.View(), names[0])
}
