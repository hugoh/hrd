package tui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/backend"
)

func (m *model) handleInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.suggestionsActive() {
		switch msg.String() {
		case "up", "down": //nolint:goconst
			m.input, cmd = m.input.Update(msg)

			return m, cmd
		}
	}

	switch msg.String() {
	case "up":
		m.updateHistoryFilter()
		m.historyPrev()

		return m, nil
	case "down":
		m.updateHistoryFilter()
		m.historyNext()

		return m, nil
	}

	m.input, cmd = m.input.Update(msg)
	m.updateCompletions()

	switch msg.String() {
	case keyEnter:
		return m.handleInputEnter()
	case keyEsc:
		m.commandOpen = false

		return m, nil
	}

	return m, cmd
}

func (m *model) handleInputEnter() (tea.Model, tea.Cmd) {
	cmdStr := strings.TrimSpace(m.input.Value())
	if cmdStr == "" {
		return m, nil
	}

	if m.executing {
		return m, nil
	}

	selected := m.selectedNames()
	if len(selected) == 0 {
		m.modal = modalAlert
		m.commandOpen = false

		return m, nil
	}

	m.commandOpen = false
	m.screen = screenOutput
	m.output.SetContent("running...")

	m.execSideEffect = true

	prefix := prefixLabels[m.cmdPrefix]
	cmd := cmdStr

	if m.cmdPrefix == prefixNone {
		prefix, cmd = parseUnifiedCmd(cmdStr)
	}

	m.pushHistory(prefix, cmd)

	return m, execCmd(m, selected, prefix, cmd)
}

func (m *model) suggestionsActive() bool {
	return m.input.ShowSuggestions && len(m.input.MatchedSuggestions()) > 0
}

func (m *model) updateCompletions() {
	input := m.input.Value()
	m.input.ShowSuggestions = false

	if input == "" {
		return
	}

	if strings.HasPrefix(input, "!") {
		return
	}

	if m.updateVCSCompletions(input) {
		return
	}

	if m.cmdPrefix == prefixNone && len(vcsSubcommands) > 0 {
		m.input.ShowSuggestions = true
		m.input.SetSuggestions(vcsSubcommands)
	}
}

func (m *model) updateVCSCompletions(input string) bool {
	for _, name := range backend.Names() {
		if m.matchVCSCompletions(input, name) {
			return true
		}
	}

	return false
}

func (m *model) matchVCSCompletions(input, name string) bool {
	prefix := name + " "

	if !strings.HasPrefix(input, prefix) && !strings.HasPrefix(prefix, input) {
		return false
	}

	if strings.HasPrefix(prefix, input) && prefix != input {
		m.input.ShowSuggestions = true
		m.input.SetSuggestions([]string{name})

		return true
	}

	m.loadVCSCompletions(name)

	if len(m.vcsCompletions[name]) > 0 {
		m.input.ShowSuggestions = true
		m.input.SetSuggestions(m.vcsCompletions[name])
	}

	return true
}

func (m *model) loadVCSCompletions(name string) {
	if m.vcsCompletions == nil {
		m.vcsCompletions = make(map[string][]string)
	}

	if _, ok := m.vcsCompletions[name]; ok {
		return
	}

	b, err := backend.ByName(name)
	if err != nil {
		m.vcsCompletions[name] = []string{}

		return
	}

	cmds, err := b.Subcommands(context.Background())
	if err != nil || cmds == nil {
		m.vcsCompletions[name] = []string{}

		return
	}

	prefixed := make([]string, len(cmds))
	for i, c := range cmds {
		prefixed[i] = name + " " + c
	}

	m.vcsCompletions[name] = prefixed
}

func (m *model) handleCmdBarOpen() (tea.Model, tea.Cmd) {
	openCommandBar(m, prefixNone)

	return m, nil
}

func openCommandBar(m *model, _ cmdPrefix) {
	m.cmdPrefix = prefixNone
	m.commandOpen = true
	m.input.SetValue("")
	m.input.ShowSuggestions = false
	m.input.Focus()
	m.input.SetWidth(m.inputWidth())
	m.historyReset()
}
