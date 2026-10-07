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
				m.setTableCursor(i)

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
	var save tea.Cmd

	names := m.tableRepos()
	if m.cursor < len(names) {
		name := names[m.cursor]
		m.selected[name] = !m.selected[name]
		m.updateTableRows()
		save = m.savePersState()
	}

	if m.cursor < len(names)-1 {
		m.cursor++
		m.setTableCursor(m.cursor)
	}

	return m, save
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

	save := m.savePersState()

	return m, save
}

// moveCursor moves the cursor by delta rows. With the cursor hidden (normal
// mode) it moves from the last row on screen instead, so every press scrolls
// the view rather than first walking an invisible cursor across it.
func (m *model) moveCursor(delta int) {
	target := m.cursor + delta

	if m.mode == modeNormal {
		if _, bottom := m.visibleRowRange(); bottom >= 0 {
			target = bottom + delta
		}
	}

	m.cursor = max(0, min(len(m.tableRepos())-1, target))
	m.setTableCursor(m.cursor)
}

func (m *model) handleCursorUp() (tea.Model, tea.Cmd) {
	m.moveCursor(-1)

	return m, nil
}

func (m *model) handleCursorDown() (tea.Model, tea.Cmd) {
	m.moveCursor(1)

	return m, nil
}

// handleCursorPage moves the cursor so the visible window shifts by one full
// page, measured from the rows on screen rather than from the cursor, which
// need not sit at the edge of the window.
func (m *model) handleCursorPage(dir int) (tea.Model, tea.Cmd) {
	last := max(0, len(m.tableRepos())-1)
	page := max(1, m.repoTable.Height())
	target := m.cursor + dir*page

	if first, bottom := m.visibleRowRange(); first >= 0 {
		if dir > 0 {
			target = bottom + page
		} else {
			target = first - 1
		}
	}

	m.cursor = max(0, min(last, target))
	m.setTableCursor(m.cursor)

	return m, nil
}
