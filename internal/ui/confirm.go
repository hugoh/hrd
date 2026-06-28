package ui

import (
	"os"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

// Confirm runs an interactive y/N prompt using bubbletea and returns true only
// if the user presses "y" or "Y". Returns false when stdin is not a TTY.
func Confirm(prompt string) bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}

	m, err := tea.NewProgram(&confirmModel{prompt: prompt}).Run()
	if err != nil {
		return false
	}

	result, ok := m.(*confirmModel)
	if !ok {
		return false
	}

	return result.confirmed
}

type confirmModel struct {
	prompt    string
	confirmed bool
}

func (*confirmModel) Init() tea.Cmd { return nil }

func (m *confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "y" || key.String() == "Y" {
			m.confirmed = true
		}

		return m, tea.Quit
	}

	return m, nil
}

func (m *confirmModel) View() tea.View {
	return tea.NewView(m.prompt + " [y/N]: ")
}
