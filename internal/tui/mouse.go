package tui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/ui"
)

func (m *model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)
	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)
	}

	return m, nil
}

func (m *model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}

	switch m.screen {
	case screenMain:
		return m.handleMainScreenClick(msg)
	case screenGroup:
		if !m.groupNewInput {
			selectListRow(&m.groupList, msg.Y)
		}

		return m, nil
	case screenSelHistory:
		selectListRow(&m.historyList, msg.Y)

		return m, nil
	case screenOutput, screenHelp:
	}

	return m, nil
}

func (m *model) handleMainScreenClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	const tableContentStart = 2

	if msg.Y <= tableContentStart {
		return m, nil
	}

	inputLineY := m.height - layoutFooterH - layoutSepH - inputLineH

	if msg.Y == inputLineY && m.commandOpen {
		m.handleInputLineClick(msg)

		return m, nil
	}

	if m.height > layoutFooterH+layoutSepH && msg.Y >= m.height-layoutFooterH-layoutSepH {
		if !m.commandOpen {
			return m.handleCmdBarOpen()
		}

		return m, nil
	}

	names := m.tableRepos()
	if len(names) == 0 {
		return m, nil
	}

	visibleHeight := m.repoTable.Height()
	firstVisible := max(0, m.cursor-visibleHeight)

	row := firstVisible + (msg.Y - tableContentStart - 1)
	if row >= len(names) {
		return m, nil
	}

	m.cursor = row
	m.repoTable.SetCursor(m.cursor)

	if m.mode == modeSelect {
		name := names[m.cursor]
		m.selected[name] = !m.selected[name]
		m.updateTableRows()
		m.savePersState()
	}

	return m, nil
}

func (m *model) handleInputLineClick(msg tea.MouseClickMsg) {
	promptWidth := lipgloss.Width(ui.WarnStyle().Render(":")) + lipgloss.Width(m.input.Prompt)
	cursorPos := max(msg.X-promptWidth, 0)

	cursorPos = min(cursorPos, len(m.input.Value()))

	m.input.SetCursor(cursorPos)
}

func selectListRow(l *list.Model, y int) {
	const listTopY = 2

	listY := y - listTopY
	if listY < 0 {
		return
	}

	if listY >= l.Height() {
		return
	}

	row := listY / itemH
	if row >= l.Paginator.PerPage {
		return
	}

	idx := l.Paginator.Page*l.Paginator.PerPage + row
	if idx < len(l.Items()) {
		l.Select(idx)
	}
}

func (m *model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenOutput:
		var cmd tea.Cmd

		m.output, cmd = m.output.Update(msg)

		return m, cmd
	case screenHelp:
		var cmd tea.Cmd

		m.helpViewport, cmd = m.helpViewport.Update(msg)

		return m, cmd
	case screenMain:
		delta := 3
		names := m.tableRepos()

		switch msg.Button {
		case tea.MouseWheelUp:
			m.cursor = max(0, m.cursor-delta)
			m.repoTable.SetCursor(m.cursor)
		case tea.MouseWheelDown:
			m.cursor = min(len(names)-1, m.cursor+delta)
			m.repoTable.SetCursor(m.cursor)
		}

		return m, nil
	case screenGroup, screenSelHistory:
		return m, nil
	}

	return m, nil
}
