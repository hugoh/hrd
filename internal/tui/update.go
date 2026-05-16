package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/theme"
	"github.com/hugoh/hrd/internal/ui"
)

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	case statusUpdateMsg:
		return m.handleStatusUpdate(msg)

	case statusDoneMsg:
		return m.handleStatusDone()
	case spinner.TickMsg:
		return m.handleSpinnerTick(msg)
	case execResultMsg:
		return m.handleExecResult(msg)
	case execDoneMsg:
		return m.handleExecDone(msg)
	case errMsg:
		return m, nil
	}

	return m, nil
}

// --- Window size ------------------------------------------------------------

func (m *model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.ready = true

	m.repoTable.SetHeight(m.contentHeight())
	m.repoTable.SetWidth(m.width)
	m.output.SetWidth(msg.Width)
	m.output.SetHeight(m.contentHeight())
	m.input.SetWidth(m.inputWidth())

	const (
		statusWPad = 6
		minStatusW = 10
	)

	statusW := m.width - checkboxColW - maxNameWidth - listVCSWidth - statusWPad
	statusW = max(statusW, minStatusW)
	m.repoTable.SetColumns([]table.Column{
		{Title: "", Width: checkboxColW},
		{Title: colName, Width: maxNameWidth},
		{Title: colVCS, Width: listVCSWidth},
		{Title: colStatus, Width: statusW},
	})

	return m, nil
}

func (m *model) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m.spinner, cmd = m.spinner.Update(msg)

	return m, cmd
}

// --- Key messages -----------------------------------------------------------

func (m *model) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Global keys
	if msg.String() == "ctrl+c" {
		if m.executing {
			m.execCancelAll()

			return m, nil
		}

		m.quit()

		return m, tea.Quit
	}

	if msg.String() == "q" {
		return m.handleQKey()
	}

	// Modal mode
	if m.modal != modalNone {
		return m.handleModalKey(msg)
	}

	// Command input mode
	if m.commandOpen {
		return m.handleInputKey(msg)
	}

	// Screen-specific
	if m.screen == screenOutput {
		return m.handleOutputKey(msg)
	}

	return m.handleMainKey(msg)
}

func (m *model) handleQKey() (tea.Model, tea.Cmd) {
	if m.commandOpen {
		return m, nil
	}

	if m.screen == screenOutput {
		m.screen = screenMain
		m.output.SetContent("")

		return m, nil
	}

	if m.modal == modalHelp {
		m.modal = modalNone

		return m, nil
	}

	if m.modal != modalNone {
		return m, nil
	}

	m.quit()

	return m, tea.Quit
}

func (m *model) handleModalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.modal { //nolint:exhaustive // modalNone handled before calling this
	case modalHelp:
		m.modal = modalNone

		return m, nil
	case modalAlert:
		switch msg.String() {
		case "space":
			m.modal = modalNone

			return m.handleSelectToggle()
		case "@":
			m.modal = modalNone
			openGroupPopup(m)

			return m, nil
		default:
			m.modal = modalNone

			return m, nil
		}
	case modalGroup:
		return m.handleGroupKey(msg)
	}

	return m, nil
}

func (m *model) handleGroupKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc, "q":
		m.modal = modalNone

		return m, nil
	case "up", "k":
		if m.groupPopupCursor > 0 {
			m.groupPopupCursor--
		}

		return m, nil
	case "down", "j":
		if m.groupPopupCursor < len(m.groupPopupOptions)-1 {
			m.groupPopupCursor++
		}

		return m, nil
	case keyEnter:
		selected := m.groupPopupOptions[m.groupPopupCursor]
		if selected == labelAllCap {
			m.groupFilter = ""
		} else {
			m.groupFilter = selected
		}

		m.cursor = 0

		m.selected = make(map[string]bool)
		for _, name := range m.filteredRepos() {
			m.selected[name] = true
		}

		m.modal = modalNone
		m.loading = true

		return m, loadStatusesCmd(m)
	}

	return m, nil
}

func (m *model) handleInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)

	switch msg.String() {
	case keyEnter:
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

		return m, execCmd(m, selected, m.selectedPrefix(), cmdStr)
	case keyEsc:
		m.commandOpen = false

		return m, nil
	}

	return m, cmd
}

func (m *model) handleOutputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc, "q":
		m.screen = screenMain

		return m, nil
	}

	var cmd tea.Cmd

	m.output, cmd = m.output.Update(msg)

	return m, cmd
}

func (m *model) handleMainKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	handler, ok := mainKeyHandlers[key]
	if ok {
		return handler(m)
	}

	return m, nil
}

//nolint:gochecknoglobals // effectively constant, key dispatch table
var mainKeyHandlers = map[string]func(*model) (tea.Model, tea.Cmd){
	"s": func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "status") },
	"l": func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "log") },
	"d": func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "diff") },
	"f": func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "fetch") },
	"p": func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "pull") },
	"P": func(m *model) (tea.Model, tea.Cmd) { return m, shortcutCmd(m, "push") },
	"r": func(m *model) (tea.Model, tea.Cmd) {
		m.loading = true

		return m, loadStatusesCmd(m)
	},
	"?": func(m *model) (tea.Model, tea.Cmd) { //nolint:unparam
		m.modal = modalHelp

		return m, nil
	},
	keyEnter: func(m *model) (tea.Model, tea.Cmd) {
		if m.selectMode {
			return m.handleSelectToggle()
		}

		return m, nil
	},
	"space": func(m *model) (tea.Model, tea.Cmd) {
		if m.selectMode {
			return m.handleSelectOne()
		}

		return m.handleSelectToggle()
	},
	"a": (*model).handleSelectAll,
	"up": func(m *model) (tea.Model, tea.Cmd) {
		return m.handleCursorUp()
	},
	"k": func(m *model) (tea.Model, tea.Cmd) {
		return m.handleCursorUp()
	},
	"down": func(m *model) (tea.Model, tea.Cmd) {
		return m.handleCursorDown()
	},
	"j": func(m *model) (tea.Model, tea.Cmd) {
		return m.handleCursorDown()
	},
	"G": func(m *model) (tea.Model, tea.Cmd) {
		return m.handleCmdBarOpen(prefixGit)
	},
	"J": func(m *model) (tea.Model, tea.Cmd) {
		return m.handleCmdBarOpen(prefixJj)
	},
	"S": func(m *model) (tea.Model, tea.Cmd) {
		return m.handleCmdBarOpen(prefixShell)
	},
	":": func(m *model) (tea.Model, tea.Cmd) {
		return m.handleCmdBarOpen(m.cmdPrefix)
	},
	"@": func(m *model) (tea.Model, tea.Cmd) { //nolint:unparam
		openGroupPopup(m)

		return m, nil
	},
}

func (m *model) handleSelectToggle() (tea.Model, tea.Cmd) {
	// Save cursor repo for continuity across mode switch.
	cur := m.tableRepos()

	saved := ""
	if m.cursor >= 0 && m.cursor < len(cur) {
		saved = cur[m.cursor]
	}

	m.selectMode = !m.selectMode
	m.repoTable.SetStyles(tableStyles(m.selectMode))
	m.updateTableRows()

	// Restore cursor to the same repo by name.
	if saved != "" {
		for i, name := range m.tableRepos() {
			if name == saved {
				m.cursor = i
				m.repoTable.SetCursor(i)

				break
			}
		}
	}

	return m, nil
}

func (m *model) handleSelectOne() (tea.Model, tea.Cmd) {
	names := m.tableRepos()
	if m.cursor < len(names) {
		name := names[m.cursor]
		m.selected[name] = !m.selected[name]
		m.updateTableRows()
	}

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

func (m *model) handleCmdBarOpen(p cmdPrefix) (tea.Model, tea.Cmd) {
	openCommandBar(m, p)

	return m, nil
}

// --- Custom messages --------------------------------------------------------

func (m *model) handleStatusUpdate(msg statusUpdateMsg) (tea.Model, tea.Cmd) {
	m.statuses[msg.result.RepoName] = msg.result
	m.updateTableRows()

	return m, streamNextStatusCmd(m)
}

func (m *model) handleStatusDone() (tea.Model, tea.Cmd) {
	m.loading = false
	m.statusCh = nil

	total := m.totalCount()
	if m.cursor >= total && total > 0 {
		m.cursor = total - 1
	}

	m.updateTableRows()

	return m, nil
}

func (m *model) handleExecResult(msg execResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.executing = false
		m.output.SetContent("error: " + msg.err.Error())

		return m, nil
	}

	m.output.SetContent(formatExecOutput(m.execResults, m.output.Width()))

	return m, streamNextResult(m)
}

func (m *model) handleExecDone(_ execDoneMsg) (tea.Model, tea.Cmd) {
	m.executing = false
	m.resultsCh = nil
	m.output.SetContent(formatExecOutput(m.execResults, m.output.Width()))

	return m, nil
}

// --- Internal helpers -------------------------------------------------------

func shortcutCmd(m *model, subcmd string) tea.Cmd {
	selected := m.selectedNames()
	if len(selected) == 0 {
		m.modal = modalAlert

		return nil
	}

	m.screen = screenOutput
	m.output.SetContent("running...")

	// VCS shortcuts always use empty prefix so they route through
	// runner.VCSSubcmd, not the current command-bar prefix.
	// might be "sh" and would route through runner.Shell instead).
	return execCmd(m, selected, "", subcmd)
}

func openCommandBar(m *model, p cmdPrefix) {
	m.cmdPrefix = p
	m.commandOpen = true
	m.input.SetValue("")
	m.input.Focus()
	m.input.SetWidth(m.inputWidth())
}

func openGroupPopup(m *model) {
	groupNames := sortedGroupNames(m.cfg.Groups)
	options := make([]string, 0, 1+len(groupNames))
	options = append(options, labelAllCap)
	options = append(options, groupNames...)
	m.groupPopupOptions = options

	m.groupPopupCursor = 0
	if m.groupFilter != "" {
		for i, opt := range options {
			if opt == m.groupFilter || opt == "@"+m.groupFilter {
				m.groupPopupCursor = i

				break
			}
		}
	}

	m.modal = modalGroup
}

func (m *model) updateTableRows() {
	names := m.tableRepos()

	rows := make([]table.Row, 0, len(names))
	for _, name := range names {
		chk := ""

		if m.selectMode {
			if m.selected[name] {
				chk = checkboxSelected
			} else {
				chk = checkboxUnsel
			}
		}

		vcs := m.cfg.Repos[name].ActiveBackend()
		statusStr := m.formatStatusLine(name)
		rows = append(rows, table.Row{chk, name, vcs, statusStr})
	}

	m.cursor = max(0, min(m.cursor, len(rows)-1))
	m.repoTable.SetRows(rows)
	m.repoTable.SetCursor(m.cursor)
}

func (m *model) formatStatusLine(name string) string {
	sr, ok := m.statuses[name]
	if !ok {
		return ui.Muted("...")
	}

	if sr.Err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorCode("red"))).
			Render("✗ " + sr.Err.Error())
	}

	st := sr.Status
	parts := ui.FormatStatusLine(st, "", "")
	refStr := parts.Ref
	refStyle := styleBold

	if len(st.Bookmarks) > 0 {
		refStyle = ui.StateStyle(st.Bookmarks[0].State)
	}

	symStr := theme.FormatSymbols(st, ui.ApplyColor)
	msg := ui.FormatDetail(st.CommitMsg, st.CommitTime)

	if msg != "" {
		return fmt.Sprintf("%s %s  %s", refStyle.Render(refStr), symStr, ui.Muted(msg))
	}

	if refStr != "" || symStr != "" {
		return fmt.Sprintf("%s %s", refStyle.Render(refStr), symStr)
	}

	return ""
}

func formatExecOutput(results []execResult, width int) string {
	var b strings.Builder

	for _, er := range results {
		b.WriteString(formatDispatchResultLine(er.name, er.result, width))
		b.WriteString("\n")
	}

	return b.String()
}

func formatDispatchResultLine(name string, res runner.Result, width int) string {
	headerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("15"))

	statusSym := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.ColorCode("green"))).
		Render("✓")

	switch {
	case res.Err != nil:
		statusSym = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorCode("red"))).
			Render("✗")
	case res.ExitCode != 0:
		statusSym = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorCode("yellow"))).
			Render("✗")
	}

	header := headerStyle.Width(width).
		Render(fmt.Sprintf("%s %s ", ui.FormatDispatchHeader(name, res.VCS), statusSym))

	switch {
	case res.Err != nil:
		body := "error: " + res.Err.Error()
		if res.Output != "" {
			body += "\n  " + strings.ReplaceAll(
				strings.TrimRight(res.Output, "\n"), "\n", "\n  ")
		}

		return header + "\n" + body
	case res.ExitCode != 0:
		body := fmt.Sprintf("exit %d", res.ExitCode)
		if res.Output != "" {
			body += "\n  " + strings.ReplaceAll(
				strings.TrimRight(res.Output, "\n"), "\n", "\n  ")
		}

		return header + "\n" + body
	default:
		if res.Output != "" {
			return header + "\n  " + strings.ReplaceAll(
				strings.TrimRight(res.Output, "\n"), "\n", "\n  ")
		}

		return header
	}
}
