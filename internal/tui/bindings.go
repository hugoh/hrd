package tui

import (
	"sync"

	tea "charm.land/bubbletea/v2"
)

//nolint:gochecknoglobals,goconst,mnd // binding definitions
var mainBindings = []binding{
	// Navigation (help only)
	{
		key:     "up",
		handler: func(m *model) (tea.Model, tea.Cmd) { return m.handleCursorUp() },
		section: secNavigation,
		desc:    "Move cursor up",
		order:   10,
	},
	{
		key:     "k",
		handler: func(m *model) (tea.Model, tea.Cmd) { return m.handleCursorUp() },
		section: secNavigation,
		order:   10,
	},
	{
		key:     "down",
		handler: func(m *model) (tea.Model, tea.Cmd) { return m.handleCursorDown() },
		section: secNavigation,
		desc:    "Move cursor down",
		order:   20,
	},
	{
		key:     "j",
		handler: func(m *model) (tea.Model, tea.Cmd) { return m.handleCursorDown() },
		section: secNavigation,
		order:   20,
	},

	// Selection
	{
		key: "x", displayKey: "x", handler: func(m *model) (tea.Model, tea.Cmd) {
			if m.mode == modeSelect {
				return m.handleSelectOne()
			}

			return m.handleSelectToggle()
		}, label: "select", desc: "Toggle selection mode / toggle repo", hrd: true,
		section: secSelection, order: 10,
	},
	{key: "enter", handler: func(m *model) (tea.Model, tea.Cmd) {
		if m.mode == modeSelect {
			return m.handleSelectToggle()
		}

		return m, nil
	}, desc: "Open repo / toggle selection", section: secSelection, order: 15},
	{
		key:     "a",
		handler: (*model).handleSelectAll,
		label:   labelAll,
		desc:    "Select / deselect all",
		hrd:     true,
		section: secSelection,
		order:   20,
	},
	{
		key:     "s",
		handler: func(m *model) (tea.Model, tea.Cmd) { return m.handleSingleToggle() },
		label:   "single",
		desc:    "Focus commands on one repo",
		hrd:     true,
		section: secSelection,
		order:   25,
	},

	// Groups
	{key: "@", handler: func(m *model) (tea.Model, tea.Cmd) {
		openGroupPopup(m, groupFilterMode)

		return m, nil
	}, label: "filter", desc: "Filter by group", hrd: true, section: secGroups, order: 10},
	{key: "g", handler: func(m *model) (tea.Model, tea.Cmd) {
		selected := m.selectedNames()
		if len(selected) == 0 {
			m.modal = modalAlert

			return m, nil
		}

		openGroupPopup(m, groupAddMode)

		return m, nil
	}, label: "group", desc: "Add selected repos to group", hrd: true, section: secGroups, order: 20},

	// Commands
	{
		key:     "S",
		handler: func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "status", false) },
		label:   "status",
		desc:    "Run status on selected",
		section: secCommands,
		order:   10,
	},
	{
		key:     "l",
		handler: func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "log", false) },
		label:   "log",
		desc:    "Run log on selected",
		section: secCommands,
		order:   20,
	},
	{
		key:     "d",
		handler: func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "diff", false) },
		label:   "diff",
		desc:    "Run diff on selected",
		section: secCommands,
		order:   30,
	},
	{
		key:        "f",
		handler:    func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "fetch", true) },
		label:      "fetch",
		desc:       "Run fetch on selected",
		section:    secCommands,
		sideEffect: true,
		order:      40,
	},
	{
		key:        "p",
		handler:    func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "pull", true) },
		label:      "pull",
		desc:       "Run pull on selected",
		section:    secCommands,
		sideEffect: true,
		order:      50,
	},
	{
		key:        "P",
		handler:    func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "push", true) },
		label:      "push",
		desc:       "Run push on selected",
		section:    secCommands,
		sideEffect: true,
		order:      60,
	},

	// General
	{key: "r", handler: func(m *model) (tea.Model, tea.Cmd) {
		m.loading = true

		return m, loadStatusesCmd(m)
	}, label: "refresh", desc: "Refresh repo statuses", hrd: true, section: secGeneral, order: 10},
	{key: "?", handler: func(m *model) (tea.Model, tea.Cmd) {
		m.helpViewport.SetContent(m.helpContent())
		m.helpViewport.GotoTop()
		m.screen = screenHelp

		return m, nil
	}, label: "help", desc: "Toggle this help", hrd: true, section: secGeneral, order: 20},
	{key: "H", handler: func(m *model) (tea.Model, tea.Cmd) {
		openSelHistoryPopup(m)

		return m, nil
	}, label: "history", desc: "Restore a previous repo selection", hrd: true, section: secGeneral, order: 25},

	// Cmd bar
	{
		key:        ":",
		handler:    func(m *model) (tea.Model, tea.Cmd) { return m.handleCmdBarOpen() },
		label:      "cmd",
		desc:       "Open command bar",
		hrd:        true,
		section:    secCmdBar,
		sideEffect: true,
		order:      10,
	},
}

//nolint:gochecknoglobals // cache for key dispatch table, built lazily
var (
	mainKeyHandlers   map[string]func(*model) (tea.Model, tea.Cmd)
	buildHandlersOnce sync.Once
)

func getKeyHandlers() map[string]func(*model) (tea.Model, tea.Cmd) {
	buildHandlersOnce.Do(func() {
		mainKeyHandlers = make(map[string]func(*model) (tea.Model, tea.Cmd), len(mainBindings))
		for _, b := range mainBindings {
			mainKeyHandlers[b.key] = b.handler
		}
	})

	return mainKeyHandlers
}
