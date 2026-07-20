package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestHandlePasteMsg(t *testing.T) {
	for _, tt := range []struct {
		name      string
		setup     func(m *model)
		wantValue func(m *model) string
	}{
		{
			name: "command bar",
			setup: func(m *model) {
				m.initInput()
				m.input.Focus()
				m.commandOpen = true
			},
			wantValue: func(m *model) string { return m.input.Value() },
		},
		{
			name: "group name input",
			setup: func(m *model) {
				m.initInput()
				m.input.Focus()
				m.groupNewInput = true
			},
			wantValue: func(m *model) string { return m.input.Value() },
		},
		{
			name: "name filter",
			setup: func(m *model) {
				m.initFilterInput()
				m.filterInput.Focus()
				m.filterOpen = true
			},
			wantValue: func(m *model) string { return m.filterInput.Value() },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := readyModel(nil, nil)
			tt.setup(m)

			_, _ = m.handlePasteMsg(tea.PasteMsg{Content: "pasted"})

			require.Equal(t, "pasted", tt.wantValue(m))
		})
	}
}

func TestHandlePasteMsgNoActiveInputIsNoop(t *testing.T) {
	m := readyModel(nil, nil)
	m.initInput()
	m.initFilterInput()

	_, cmd := m.handlePasteMsg(tea.PasteMsg{Content: "pasted"})

	require.Nil(t, cmd)
	require.Empty(t, m.input.Value())
	require.Empty(t, m.filterInput.Value())
}
