package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/config"
)

func (m *model) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.helpViewport.ScrollUp(1)

		return m, nil
	case "down", "j": //nolint:goconst
		m.helpViewport.ScrollDown(1)

		return m, nil
	}

	var cmd tea.Cmd

	m.helpViewport, cmd = m.helpViewport.Update(msg)

	return m, cmd
}

func (m *model) handleGroupKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == keyEnter {
		return m.handleGroupEnter()
	}

	var cmd tea.Cmd

	m.groupList, cmd = m.groupList.Update(msg)

	return m, cmd
}

func (m *model) handleGroupEnter() (tea.Model, tea.Cmd) {
	item, ok := m.groupList.SelectedItem().(groupItem)
	if !ok {
		return m, nil
	}

	selected := item.name

	switch m.groupMode {
	case groupFilterMode:
		return m.handleGroupFilterSelect(selected)
	case groupAddMode:
		return m.handleGroupAddSelect(selected)
	}

	return m, nil
}

func (m *model) handleGroupFilterSelect(selected string) (tea.Model, tea.Cmd) {
	switch selected {
	case labelAllRepos:
		m.groupFilter = ""
	case labelUngrouped:
		m.groupFilter = config.ReservedNone
	default:
		m.groupFilter = selected
	}

	m.cursor = 0
	m.selected = makeSelectedMap(m.filteredRepos())

	m.screen = screenMain
	m.loading = true
	m.pushSelectionHistory()
	m.savePersState()

	return m, loadStatusesCmd(m)
}

func (m *model) handleGroupAddSelect(selected string) (tea.Model, tea.Cmd) {
	if selected == labelNew {
		m.input.SetValue("")
		m.input.Placeholder = "enter group name..."
		m.input.Focus()
		m.input.SetWidth(m.inputWidth())
		m.groupNewInput = true

		return m, nil
	}

	if err := m.addSelectedToGroup(selected); err != nil {
		m.modal = modalAlert
		m.alertMsg = "save failed: " + err.Error()

		return m, nil
	}

	m.screen = screenMain

	return m, nil
}

func (m *model) handleGroupNewInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)

	switch msg.String() {
	case keyEnter:
		name := stripGroupPrefix(strings.TrimSpace(m.input.Value()))
		if name == "" {
			return m, nil
		}

		if err := config.ValidGroupName(name); err != nil {
			m.modal = modalAlert
			m.alertMsg = err.Error()

			return m, nil
		}

		if err := m.addSelectedToGroup(name); err != nil {
			m.modal = modalAlert
			m.alertMsg = "save failed: " + err.Error()

			return m, nil
		}

		m.groupNewInput = false
		m.screen = screenMain

		return m, nil
	case keyEsc:
		m.groupNewInput = false

		return m, nil
	}

	return m, cmd
}

func (m *model) handleOutputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc, "q", "o":
		m.screen = screenMain

		return m, nil
	case "enter":
		if !m.executing {
			m.screen = screenMain

			return m, nil
		}
	}

	var cmd tea.Cmd

	m.output, cmd = m.output.Update(msg)

	return m, cmd
}

func openSelHistoryPopup(m *model) {
	if len(m.persState.SelectionHistory) == 0 {
		return
	}

	items := buildHistoryItems(m.persState.SelectionHistory, m.cfg.Groups, m.allRepoSet())
	m.historyList.SetItems(items)
	m.historyList.Select(0)

	m.screen = screenSelHistory
}

func (m *model) handleSelHistoryKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == keyEnter {
		item, ok := m.historyList.SelectedItem().(historyItem)
		if !ok {
			return m, nil
		}

		return m.handleSelHistoryRestore(item.repos)
	}

	var cmd tea.Cmd

	m.historyList, cmd = m.historyList.Update(msg)

	return m, cmd
}

func (m *model) handleSelHistoryRestore(repos []string) (tea.Model, tea.Cmd) {
	selected := make(map[string]bool, len(repos))

	var missing []string

	for _, name := range repos {
		if _, ok := m.cfg.Repos[name]; ok {
			selected[name] = true
		} else {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		m.modal = modalAlert
		m.alertMsg = fmt.Sprintf("Warning: %d repo(s) no longer exist:\n%s",
			len(missing), strings.Join(missing, "\n"))
	}

	m.selected = selected
	m.mode = modeNormal
	m.repoTable.SetStyles(tableStyles(false))
	m.updateTableRows()
	m.pushSelectionHistory()
	m.savePersState()
	m.screen = screenMain

	return m, loadStatusesCmd(m)
}

func openGroupPopup(m *model, mode groupMode) {
	groupNames := sortedGroupNames(m.cfg.Groups)

	var options []string

	const builtinFilterOptions = 2 // [all], [ungrouped]

	switch mode {
	case groupFilterMode:
		options = make([]string, 0, builtinFilterOptions+len(groupNames))
		options = append(options, labelAllRepos, labelUngrouped)
		options = append(options, groupNames...)
	case groupAddMode:
		options = make([]string, 0, len(groupNames)+1)
		options = append(options, groupNames...)
		options = append(options, labelNew)
	}

	m.groupMode = mode
	m.groupList = initList(defaultItemDelegate(0), nil, m.width)
	m.groupList.SetHeight(m.contentHeight())

	items := buildGroupItems(options, m.cfg.Groups, len(m.cfg.Repos), len(m.cfg.UngroupedRepos()))
	m.groupList.SetItems(items)

	cursor := 0

	if m.groupFilter != "" && mode == groupFilterMode {
		for i, opt := range options {
			if opt == m.groupFilter || opt == "@"+m.groupFilter ||
				(opt == labelUngrouped && m.groupFilter == config.ReservedNone) {
				cursor = i

				break
			}
		}
	}

	m.groupList.Select(cursor)
	m.screen = screenGroup
}
