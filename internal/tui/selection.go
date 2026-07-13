package tui

import (
	"maps"

	tea "charm.land/bubbletea/v2"
)

func (m *model) toggleMode(target mode, saveSelection bool) {
	cur := m.tableRepos()

	saved := ""
	if m.cursor >= 0 && m.cursor < len(cur) {
		saved = cur[m.cursor]
	}

	if m.mode == target {
		m.mode = modeNormal
	} else {
		if saveSelection {
			m.selectSaved = maps.Clone(m.selected)
		}

		m.mode = target
	}

	m.repoTable.SetStyles(tableStyles(m.mode != modeNormal, m.darkBackground))
	m.updateTableRows()

	if saved != "" {
		for i, name := range m.tableRepos() {
			if name == saved {
				m.cursor = i
				m.repoTable.SetCursor(i)

				break
			}
		}
	}
}

func (m *model) handleSelectToggle() (tea.Model, tea.Cmd) {
	m.toggleMode(modeSelect, true)

	return m, nil
}

func (m *model) handleSelectOne() (tea.Model, tea.Cmd) {
	names := m.tableRepos()
	if m.cursor < len(names) {
		name := names[m.cursor]
		m.selected[name] = !m.selected[name]
		m.updateTableRows()
		m.savePersState()
	}

	if m.cursor < len(names)-1 {
		m.cursor++
		m.repoTable.SetCursor(m.cursor)
	}

	return m, nil
}

func (m *model) handleSingleToggle() (tea.Model, tea.Cmd) {
	m.toggleMode(modeSingle, false)

	return m, nil
}

func (m *model) handleSelectAll() (tea.Model, tea.Cmd) {
	if m.allSelected() {
		m.selected = make(map[string]bool)
	} else {
		m.selected = make(map[string]bool)
		for _, name := range m.filteredRepos() {
			m.selected[name] = true
		}
	}

	m.updateTableRows()
	m.pushSelectionHistory()
	m.savePersState()

	return m, nil
}

func (m *model) handleCursorUp() (tea.Model, tea.Cmd) {
	if m.cursor > 0 {
		m.cursor--
		m.repoTable.SetCursor(m.cursor)
	}

	return m, nil
}

func (m *model) handleCursorDown() (tea.Model, tea.Cmd) {
	if m.cursor < len(m.tableRepos())-1 {
		m.cursor++
		m.repoTable.SetCursor(m.cursor)
	}

	return m, nil
}
