package tui

import (
	"testing"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
)

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
