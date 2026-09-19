package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/cmdspec"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/theme"
	"github.com/hugoh/hrd/internal/ui"
)

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	m.syncLayout()

	return next, cmd
}

func (m *model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	case tea.PasteMsg:
		return m.handlePasteMsg(msg)
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
		return m.handleStatusDone(msg)
	case spinner.TickMsg:
		return m.handleSpinnerTick(msg)
	case progress.FrameMsg:
		return m.handleProgressFrame(msg)
	case execResultMsg:
		return m.handleExecResult(msg)
	case execDoneMsg:
		return m.handleExecDone(msg)
	case vcsCompletionsMsg:
		return m.handleVCSCompletions(msg)
	case tea.BackgroundColorMsg:
		m.applyBackground(msg.IsDark())

		return m, nil
	}

	return m.handleDiskMsg(msg)
}

// handleDiskMsg dispatches the results of config-file Cmds.
func (m *model) handleDiskMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case configLoadedMsg:
		return m.handleConfigLoaded(msg)
	case groupSavedMsg:
		return m.handleGroupSaved(msg)
	}

	return m, nil
}

// applyBackground re-derives every light/dark-dependent style built at
// startup, since the terminal's real background only arrives after Init.
func (m *model) applyBackground(dark bool) {
	m.darkBackground = dark
	m.repoTable.SetStyles(tableStyles(m.mode != modeNormal, dark))
	m.historyList.SetDelegate(defaultItemDelegate(dark))
	m.groupList.SetDelegate(defaultItemDelegate(dark))
}

func (m *model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.ready = true

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

// handleSpinnerTick advances the spinners. Each spinner re-arms its own tick
// chain through the Cmd its Update returns, so dropping that Cmd once nothing
// is animating stops the idle redraws; spinnerTicks restarts them.
func (m *model) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var headerCmd, rowCmd tea.Cmd

	m.spinner, headerCmd = m.spinner.Update(msg)
	m.rowSpinner, rowCmd = m.rowSpinner.Update(msg)

	if !m.animating() {
		return m, nil
	}

	if len(m.pending) > 0 {
		m.updateTableRows()
	}

	return m, tea.Batch(headerCmd, rowCmd)
}

func (m *model) animating() bool {
	return m.loading || m.executing || len(m.pending) > 0
}

// spinnerTicks (re)starts both spinner tick chains. Bubbles drops ticks
// carrying a stale tag, so starting one while a chain is still alive is
// harmless.
func (m *model) spinnerTicks() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.rowSpinner.Tick)
}

// handleProgressFrame advances the exec-progress bar's spring animation one
// step. The progress model self-perpetuates its own tick chain (the returned
// Cmd re-arms the next frame) until it settles at its target percent.
func (m *model) handleProgressFrame(msg progress.FrameMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m.progress, cmd = m.progress.Update(msg)

	return m, cmd
}

//nolint:cyclop // key dispatch with multiple screens
func (m *model) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m.handleCtrlC()
	case "ctrl+z":
		return m, tea.Suspend
	}

	if m.modal == modalAlert {
		key := msg.String()

		m.dismissAlert()

		// Esc, and q outside a text input, only dismiss; any other key also
		// acts normally so an alert never costs the user a keypress.
		if key == keyEsc || (key == "q" && !m.textInputFocused()) {
			return m, nil
		}
	}

	if m.commandOpen {
		return m.handleInputKey(msg)
	}

	if m.filterOpen {
		return m.handleFilterKey(msg)
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

// handlePasteMsg routes a bracketed paste to whichever text input is
// currently active. bubbletea delivers pasted text as tea.PasteMsg rather
// than tea.KeyPressMsg, so it needs its own dispatch alongside handleKeyMsg.
func (m *model) handlePasteMsg(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch {
	case m.commandOpen:
		m.input, cmd = m.input.Update(msg)
		loadCmd := m.updateCompletions()

		return m, tea.Batch(cmd, loadCmd)
	case m.groupNewInput:
		m.input, cmd = m.input.Update(msg)

		return m, cmd
	case m.filterOpen:
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.nameFilter = strings.TrimSpace(m.filterInput.Value())
		m.updateTableRows()

		return m, cmd
	}

	return m, nil
}

// handleFilterKey edits the "/" name filter: every keystroke re-filters
// live, enter confirms (filter stays active), esc clears and closes.
func (m *model) handleFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEnter:
		m.filterOpen = false
		m.filterInput.Blur()

		return m, nil
	case keyEsc:
		m.clearNameFilter()

		return m, nil
	}

	var cmd tea.Cmd

	m.filterInput, cmd = m.filterInput.Update(msg)
	m.nameFilter = strings.TrimSpace(m.filterInput.Value())
	m.updateTableRows()

	return m, cmd
}

func (m *model) handleEscKey() (tea.Model, tea.Cmd) {
	if m.groupNewInput {
		m.groupNewInput = false

		return m, nil
	}

	// A confirmed name filter is cleared by esc on the main screen.
	if m.screen == screenMain && m.nameFilter != "" && m.mode == modeNormal {
		m.clearNameFilter()

		return m, nil
	}

	if m.mode != modeNormal {
		if m.mode == modeSelect && m.selectSaved != nil {
			m.selected = m.selectSaved
			m.selectSaved = nil
		}

		m.mode = modeNormal
		m.repoTable.SetStyles(tableStyles(false, m.darkBackground))
		m.updateTableRows()

		return m, nil
	}

	switch m.screen { //nolint:exhaustive
	case screenOutput:
		m.screen = screenMain

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

func (m *model) dismissAlert() {
	m.modal = modalNone
	m.alertMsg = ""
}

func (m *model) textInputFocused() bool {
	return m.commandOpen || m.filterOpen || m.groupNewInput
}

func (m *model) handleQKey() (tea.Model, tea.Cmd) {
	if m.commandOpen {
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
	handler, ok := getKeyHandlers()[msg.String()]
	if ok {
		return handler(m)
	}

	return m, nil
}

// --- Custom messages --------------------------------------------------------

func (m *model) handleStatusUpdate(msg statusUpdateMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.statusGen {
		return m, nil
	}

	m.statuses[msg.result.RepoName] = msg.result
	delete(m.pending, msg.result.RepoName)
	m.updateTableRows()

	m.statusAnyErr = m.statusAnyErr || msg.result.Err != nil

	return m, streamNextStatusCmd(m)
}

func (m *model) handleStatusDone(msg statusDoneMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.statusGen {
		return m, nil
	}

	m.loading = false
	m.statusCh = nil
	clear(m.pending)

	total := m.totalCount()
	if m.cursor >= total && total > 0 {
		m.cursor = total - 1
	}

	m.updateTableRows()

	return m, nil
}

func (m *model) handleExecResult(msg execResultMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.execGen {
		return m, nil
	}

	if msg.err != nil {
		m.executing = false
		m.screen = screenOutput
		m.output.SetContent("error: " + msg.err.Error())

		return m, nil
	}

	m.execResults = append(m.execResults, msg.result)
	m.execResultOffsets = append(m.execResultOffsets, strings.Count(m.execOutputStr, "\n"))
	m.execOutputStr += formatDispatchResultLine(
		msg.result.name,
		msg.result.result,
		m.output.Width(),
		m.darkBackground,
	) + "\n\n"
	m.output.SetContent(m.execOutputStr)

	var progressCmd tea.Cmd

	if m.execTotal > 0 {
		pct := float64(len(m.execResults)) / float64(m.execTotal)
		progressCmd = m.progress.SetPercent(pct)
	}

	return m, tea.Batch(streamNextResult(m), progressCmd)
}

func (m *model) handleExecDone(msg execDoneMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.execGen {
		return m, nil
	}

	m.executing = false
	m.resultsCh = nil

	if m.execSideEffect {
		m.execSideEffect = false
		m.loading = true

		names := execResultNames(m.execResults)
		if len(names) == 0 {
			names = m.filteredRepos()
		}

		cmd := loadStatusesForCmd(m, names)
		m.updateTableRows()

		return m, cmd
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
		cmd := m.updateCompletions()

		return m, cmd
	}

	return m, nil
}

// --- Internal helpers -------------------------------------------------------

func shortcutCmd(m *model, subcmd string, sideEffect bool) tea.Cmd {
	if m.executing {
		return nil
	}

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
	return cmdspec.Parse(input)
}

func (m *model) pushSelectionHistory() {
	current := sortedSelected(m.selected)
	if len(current) == 0 {
		return
	}

	if len(m.persState.SelectionHistory) > 0 {
		last := m.persState.SelectionHistory[0].Repos
		if slices.Equal(current, last) {
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

			if m.pending[name] {
				chk += m.rowSpinner.View()
			}
		} else if m.pending[name] {
			chk = m.rowSpinner.View()
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

func formatExecOutput(results []execResult, width int, dark bool) string {
	var b strings.Builder

	for _, er := range results {
		b.WriteString(formatDispatchResultLine(er.name, er.result, width, dark))
		b.WriteString("\n\n")
	}

	return b.String()
}

func formatDispatchResultLine(name string, res runner.Result, width int, dark bool) string {
	res.RepoName = name

	return ui.RenderDispatchResultBar(res, width, dark)
}
