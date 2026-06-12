package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/backend"
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
	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	default:
		return m.handleAsyncMsg(msg)
	}
}

// handleAsyncMsg dispatches messages produced by Cmd goroutines
// (status/exec streaming, spinner ticks, completion loads).
func (m *model) handleAsyncMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
	case vcsCompletionsMsg:
		return m.handleVCSCompletions(msg)
	}

	return m, nil
}

func (m *model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.ready = true

	m.repoTable.SetHeight(m.contentHeight())
	m.repoTable.SetWidth(m.width)
	m.output.SetWidth(msg.Width)
	m.output.SetHeight(m.contentHeight())
	m.helpViewport.SetWidth(m.width)
	m.helpViewport.SetHeight(m.contentHeight())
	m.historyList.SetWidth(m.width)
	m.historyList.SetHeight(m.contentHeight())
	m.groupList.SetWidth(m.width)
	m.groupList.SetHeight(m.contentHeight())
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

//nolint:cyclop // key dispatch with multiple screens
func (m *model) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m.handleCtrlC()
	}

	if m.commandOpen {
		return m.handleInputKey(msg)
	}

	if m.groupNewInput {
		return m.handleGroupNewInput(msg)
	}

	if msg.String() == "q" {
		return m.handleQKey()
	}

	if msg.String() == keyEsc {
		return m.handleEscKey()
	}

	switch m.screen {
	case screenMain:
		return m.handleMainKey(msg)
	case screenOutput:
		return m.handleOutputKey(msg)
	case screenHelp:
		return m.handleHelpKey(msg)
	case screenGroup:
		return m.handleGroupKey(msg)
	case screenSelHistory:
		return m.handleSelHistoryKey(msg)
	}

	return m, nil
}

func (m *model) handleEscKey() (tea.Model, tea.Cmd) {
	if m.groupNewInput {
		m.groupNewInput = false

		return m, nil
	}

	if m.mode != modeNormal {
		if m.mode == modeSelect && m.selectSaved != nil {
			m.selected = m.selectSaved
			m.selectSaved = nil
		}

		m.mode = modeNormal
		m.repoTable.SetStyles(tableStyles(false))
		m.updateTableRows()

		return m, nil
	}

	switch m.screen { //nolint:exhaustive
	case screenOutput:
		m.screen = screenMain
		m.output.SetContent("")

		return m, nil
	case screenHelp, screenGroup, screenSelHistory:
		m.screen = screenMain

		return m, nil
	}

	return m, nil
}

func (m *model) handleCtrlC() (tea.Model, tea.Cmd) {
	if m.executing {
		m.execCancelAll()

		return m, nil
	}

	m.quit()

	return m, tea.Quit
}

func (m *model) handleQKey() (tea.Model, tea.Cmd) {
	if m.commandOpen {
		return m, nil
	}

	if m.modal == modalAlert {
		m.modal = modalNone
		m.alertMsg = ""

		return m, nil
	}

	switch m.screen {
	case screenOutput:
		m.screen = screenMain
		m.output.SetContent("")

		return m, nil
	case screenHelp, screenGroup, screenSelHistory:
		m.groupNewInput = false
		m.screen = screenMain

		return m, nil
	case screenMain:
		m.quit()

		return m, tea.Quit
	}

	return m, nil
}

func (m *model) handleMainKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Clear alert on any key.
	if m.modal == modalAlert {
		m.modal = modalNone
		m.alertMsg = ""
	}

	key := msg.String()

	handler, ok := getKeyHandlers()[key]
	if ok {
		return handler(m)
	}

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

	m.execResults = append(m.execResults, msg.result)
	m.output.SetContent(formatExecOutput(m.execResults, m.output.Width()))

	return m, streamNextResult(m)
}

func (m *model) handleExecDone(_ execDoneMsg) (tea.Model, tea.Cmd) {
	m.executing = false
	m.resultsCh = nil
	m.output.SetContent(formatExecOutput(m.execResults, m.output.Width()))

	if m.execSideEffect {
		m.execSideEffect = false
		m.loading = true

		return m, loadStatusesCmd(m)
	}

	return m, nil
}

func (m *model) handleVCSCompletions(msg vcsCompletionsMsg) (tea.Model, tea.Cmd) {
	if m.vcsCompletions == nil {
		m.vcsCompletions = make(map[string][]string)
	}

	m.vcsCompletions[msg.name] = msg.cmds

	// Refresh suggestions if the user is still typing a matching command.
	if m.commandOpen {
		return m, m.updateCompletions()
	}

	return m, nil
}

// --- Internal helpers -------------------------------------------------------

func shortcutCmd(m *model, subcmd string, sideEffect bool) tea.Cmd {
	selected := m.selectedNames()
	if len(selected) == 0 {
		m.modal = modalAlert

		return nil
	}

	m.screen = screenOutput
	m.output.SetContent("running...")

	m.execSideEffect = sideEffect

	// VCS shortcuts always use empty prefix so they route through
	// runner.VCSSubcmd, not the current command-bar prefix.
	// might be "sh" and would route through runner.Shell instead).
	return execCmd(m, selected, "", subcmd)
}

func parseUnifiedCmd(input string) (string, string) {
	if strings.HasPrefix(input, "!") {
		return "sh", strings.TrimSpace(input[1:])
	}

	for _, name := range backend.Names() {
		prefix := name + " "
		if strings.HasPrefix(input, prefix) {
			return name, strings.TrimSpace(input[len(prefix):])
		}
	}

	return "", input
}

func (m *model) pushSelectionHistory() {
	current := sortedSelected(m.selected)
	if len(current) == 0 {
		return
	}

	if len(m.persState.SelectionHistory) > 0 {
		last := m.persState.SelectionHistory[0].Repos
		if equalStringSlices(current, last) {
			return
		}
	}

	entry := SelectionEntry{
		Timestamp: time.Now(),
		Repos:     current,
	}

	m.persState.SelectionHistory = append([]SelectionEntry{entry}, m.persState.SelectionHistory...)
	if len(m.persState.SelectionHistory) > selectionHistoryCap {
		m.persState.SelectionHistory = m.persState.SelectionHistory[:selectionHistoryCap]
	}
}

func sortedSelected(selected map[string]bool) []string {
	out := make([]string, 0, len(selected))
	for name, sel := range selected {
		if sel {
			out = append(out, name)
		}
	}

	slices.Sort(out)

	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func (m *model) updateTableRows() {
	names := m.tableRepos()

	rows := make([]table.Row, 0, len(names))
	for _, name := range names {
		chk := ""

		if m.mode == modeSelect {
			if m.selected[name] {
				chk = checkboxSelected
			} else {
				chk = checkboxUnsel
			}
		}

		statusStr := m.formatStatusLine(name)
		rows = append(rows, table.Row{chk, name, m.repoVCS(name), statusStr})
	}

	m.cursor = max(0, min(m.cursor, len(rows)-1))
	m.repoTable.SetRows(rows)
	m.repoTable.SetCursor(m.cursor)
}

// repoVCS returns the backend name for a repo without re-running
// filesystem detection on every table redraw. Fresh status results win;
// otherwise the detection result is cached for the session.
func (m *model) repoVCS(name string) string {
	if sr, ok := m.statuses[name]; ok && sr.VCS != "" {
		return sr.VCS
	}

	if v, ok := m.vcsCache[name]; ok {
		return v
	}

	if m.vcsCache == nil {
		m.vcsCache = make(map[string]string)
	}

	v := m.cfg.Repos[name].ActiveBackend()
	m.vcsCache[name] = v

	return v
}

func (m *model) formatStatusLine(name string) string {
	sr, ok := m.statuses[name]
	if !ok {
		return ui.Muted("...")
	}

	if sr.Err != nil {
		return ui.ApplyColor("red", "✗ "+sr.Err.Error())
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
