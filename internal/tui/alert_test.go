package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/require"
)

func alertModel(setup func(*model)) *model {
	return testModel(func(m *model) {
		m.cfg = config.Config{Groups: map[string]config.Group{"work": {}}}
		m.modal = modalAlert
		m.alertMsg = "save failed: nope"
		m.initGroupList()
		m.initInput()

		if setup != nil {
			setup(m)
		}
	})
}

func TestKeyDismissesAlert(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*model)
		key   tea.KeyPressMsg
	}{
		{name: "esc on main", key: tea.KeyPressMsg{Code: tea.KeyEscape}},
		{name: "any key on main", key: tea.KeyPressMsg{Code: 'j', Text: "j"}},
		{
			name:  "key on group screen",
			setup: func(m *model) { m.screen = screenGroup },
			key:   tea.KeyPressMsg{Code: tea.KeyDown},
		},
		{
			name: "key in command bar",
			setup: func(m *model) {
				openCommandBar(m)
			},
			key: tea.KeyPressMsg{Code: 'a', Text: "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := alertModel(tt.setup)
			screen := m.screen

			m.handleKeyMsg(tt.key)

			require.Equal(t, modalNone, m.modal)
			require.Empty(t, m.alertMsg)
			require.Equal(t, screen, m.screen, "dismissing must not leave the screen")
		})
	}
}

func TestEscOnlyDismissesAlert(t *testing.T) {
	m := alertModel(func(m *model) { openCommandBar(m) })

	m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})

	require.True(t, m.commandOpen, "esc dismisses the alert without also closing the command bar")
}

func TestKeyDismissingAlertStillReachesInput(t *testing.T) {
	m := alertModel(func(m *model) { openCommandBar(m) })

	m.handleKeyMsg(tea.KeyPressMsg{Code: 'a', Text: "a"})

	require.Equal(t, "a", m.input.Value())
}

func TestGroupViewsRenderAlert(t *testing.T) {
	m := alertModel(nil)

	require.Contains(t, m.groupView(), "save failed: nope")

	m.groupNewInput = true

	require.Contains(t, m.groupView(), "save failed: nope")
}
