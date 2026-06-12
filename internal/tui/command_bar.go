package tui

import (
	"context"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/cmdspec"
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
	loadCmd := m.updateCompletions()

	switch msg.String() {
	case keyEnter:
		return m.handleInputEnter()
	case keyEsc:
		m.commandOpen = false

		return m, nil
	}

	return m, tea.Batch(cmd, loadCmd)
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
		prefix, cmd = parseUnifiedCmd(cmdspec.ExpandAlias(m.cfg.Aliases, cmdStr))
	}

	m.pushHistory(prefix, cmd)

	return m, execCmd(m, selected, prefix, cmd)
}

func (m *model) suggestionsActive() bool {
	return m.input.ShowSuggestions && len(m.input.MatchedSuggestions()) > 0
}

// updateCompletions refreshes the suggestion list for the current input.
// The returned Cmd (possibly nil) asynchronously loads a backend's
// subcommand list the first time it is needed.
func (m *model) updateCompletions() tea.Cmd {
	input := m.input.Value()
	m.input.ShowSuggestions = false

	if input == "" {
		return nil
	}

	if strings.HasPrefix(input, "!") {
		return nil
	}

	if cmd, handled := m.updateVCSCompletions(input); handled {
		return cmd
	}

	if m.cmdPrefix == prefixNone && len(vcsSubcommands) > 0 {
		m.input.ShowSuggestions = true
		m.input.SetSuggestions(m.unifiedSuggestions())
	}

	return nil
}

// unifiedSuggestions returns the VCS subcommands plus the user's config
// aliases for the unified command bar.
func (m *model) unifiedSuggestions() []string {
	suggestions := slices.Clone(vcsSubcommands)
	for name := range m.cfg.Aliases {
		suggestions = append(suggestions, name)
	}

	slices.Sort(suggestions[len(vcsSubcommands):])

	return suggestions
}

func (m *model) updateVCSCompletions(input string) (tea.Cmd, bool) {
	for _, name := range backend.Names() {
		if cmd, ok := m.matchVCSCompletions(input, name); ok {
			return cmd, true
		}
	}

	return nil, false
}

func (m *model) matchVCSCompletions(input, name string) (tea.Cmd, bool) {
	prefix := name + " "

	if !strings.HasPrefix(input, prefix) && !strings.HasPrefix(prefix, input) {
		return nil, false
	}

	if strings.HasPrefix(prefix, input) && prefix != input {
		m.input.ShowSuggestions = true
		m.input.SetSuggestions([]string{name})

		return nil, true
	}

	cmd := m.ensureVCSCompletions(name)

	if len(m.vcsCompletions[name]) > 0 {
		m.input.ShowSuggestions = true
		m.input.SetSuggestions(m.vcsCompletions[name])
	}

	return cmd, true
}

// ensureVCSCompletions kicks off an async load of the backend's
// subcommand list the first time it is needed. Shelling out (git help,
// jj util completion) must not run on the Update goroutine — it would
// freeze the UI on the first matching keystroke. Returns nil when the
// list is already loaded or loading.
func (m *model) ensureVCSCompletions(name string) tea.Cmd {
	if m.vcsCompletions == nil {
		m.vcsCompletions = make(map[string][]string)
	}

	if _, ok := m.vcsCompletions[name]; ok {
		return nil
	}

	// Mark as loading so further keystrokes don't spawn more loads.
	m.vcsCompletions[name] = []string{}

	b, err := backend.ByName(name)
	if err != nil {
		return nil
	}

	return func() tea.Msg {
		cmds, err := b.Subcommands(context.Background())
		if err != nil {
			cmds = nil
		}

		prefixed := make([]string, len(cmds))
		for i, c := range cmds {
			prefixed[i] = name + " " + c
		}

		return vcsCompletionsMsg{name: name, cmds: prefixed}
	}
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
