package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/config"
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
