package tui

import (
	"image/color"
	"testing"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/theme"
	"github.com/stretchr/testify/require"
)

func TestCtrlZSuspends(t *testing.T) {
	m := testModel()

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})

	require.NotNil(t, cmd)
	_, ok := cmd().(tea.SuspendMsg)
	require.True(t, ok)
}

func TestBackgroundColorMsgRestylesTable(t *testing.T) {
	m := testModel(func(m *model) { m.darkBackground = true })
	m.mode = modeSelect
	m.repoTable.SetStyles(tableStyles(true, true))
	m.repoTable.SetRows([]table.Row{{"", "alpha", "git", "ok"}})

	m.Update(tea.BackgroundColorMsg{Color: color.White})

	require.False(t, m.darkBackground)

	rendered := m.repoTable.View()
	require.Contains(t, rendered, "48;5;"+theme.SelectionBackground.Light)
	require.NotContains(t, rendered, "48;5;"+theme.SelectionBackground.Dark)
}

func TestOpenCommandBarRestoresPlaceholder(t *testing.T) {
	m := testModel()
	m.initInput()
	m.input.Placeholder = "enter group name..."

	openCommandBar(m)

	require.Equal(t, cmdPlaceholder, m.input.Placeholder)
}
