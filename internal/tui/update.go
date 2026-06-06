package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
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

// --- Key messages -----------------------------------------------------------

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
	if selected == labelAllRepos {
		m.groupFilter = ""
	} else {
		m.groupFilter = selected
	}

	m.cursor = 0

	m.selected = make(map[string]bool)
	for _, name := range m.filteredRepos() {
		m.selected[name] = true
	}

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

	for _, repoName := range m.selectedNames() {
		m.cfg.AddRepoToGroup(repoName, selected)
	}

	if err := config.Save(m.opts.ConfigPath, m.cfg); err != nil {
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
		name := strings.TrimSpace(m.input.Value())
		if name == "" {
			return m, nil
		}

		for _, repoName := range m.selectedNames() {
			m.cfg.AddRepoToGroup(repoName, name)
		}

		if err := config.Save(m.opts.ConfigPath, m.cfg); err != nil {
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

func (m *model) handleInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.suggestionsActive() {
		switch msg.String() {
		case "up", "down":
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

	// Phase 1: partial prefix ("g" / "gi") → suggest backend name.
	if strings.HasPrefix(prefix, input) && prefix != input {
		m.input.ShowSuggestions = true
		m.input.SetSuggestions([]string{name})

		return true
	}

	// Phase 2: full prefix typed → load and suggest subcommands.
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

func (m *model) handleSelectToggle() (tea.Model, tea.Cmd) {
	// Save cursor repo for continuity across mode switch.
	cur := m.tableRepos()

	saved := ""
	if m.cursor >= 0 && m.cursor < len(cur) {
		saved = cur[m.cursor]
	}

	if m.mode == modeSelect {
		m.mode = modeNormal
	} else {
		m.mode = modeSelect
	}

	m.repoTable.SetStyles(tableStyles(m.mode != modeNormal))
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
		m.savePersState()
	}

	if m.cursor < len(names)-1 {
		m.cursor++
		m.repoTable.SetCursor(m.cursor)
	}

	return m, nil
}

func (m *model) handleSingleToggle() (tea.Model, tea.Cmd) {
	cur := m.tableRepos()

	saved := ""
	if m.cursor >= 0 && m.cursor < len(cur) {
		saved = cur[m.cursor]
	}

	if m.mode == modeSingle {
		m.mode = modeNormal
	} else {
		m.mode = modeSingle
	}

	m.repoTable.SetStyles(tableStyles(m.mode != modeNormal))
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

func (m *model) handleCmdBarOpen() (tea.Model, tea.Cmd) {
	openCommandBar(m, prefixNone)

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

	if m.execSideEffect {
		m.execSideEffect = false
		m.loading = true

		return m, loadStatusesCmd(m)
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

func openSelHistoryPopup(m *model) {
	if len(m.persState.SelectionHistory) == 0 {
		return
	}

	allRepoSet := make(map[string]struct{}, len(m.cfg.Repos))
	for name := range m.cfg.Repos {
		allRepoSet[name] = struct{}{}
	}

	items := buildHistoryItems(m.persState.SelectionHistory, m.cfg.Groups, allRepoSet)
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
	selected := make(map[string]bool)

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

func sortedSelected(selected map[string]bool) []string {
	out := make([]string, 0, len(selected))
	for name, sel := range selected {
		if sel {
			out = append(out, name)
		}
	}

	sort.Strings(out)

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

func openCommandBar(m *model, _ cmdPrefix) {
	m.cmdPrefix = prefixNone
	m.commandOpen = true
	m.input.SetValue("")
	m.input.ShowSuggestions = false
	m.input.Focus()
	m.input.SetWidth(m.inputWidth())
	m.historyReset()
}

func openGroupPopup(m *model, mode groupMode) {
	groupNames := sortedGroupNames(m.cfg.Groups)

	var options []string

	switch mode {
	case groupFilterMode:
		options = make([]string, 0, 1+len(groupNames))
		options = append(options, labelAllRepos)
		options = append(options, groupNames...)
	case groupAddMode:
		options = make([]string, 0, len(groupNames)+1)
		options = append(options, groupNames...)
		options = append(options, labelNew)
	}

	m.groupMode = mode
	m.groupList = initList(defaultItemDelegate(0), nil, m.width)
	m.groupList.SetHeight(m.contentHeight())

	items := buildGroupItems(options, m.cfg.Groups, len(m.cfg.Repos))
	m.groupList.SetItems(items)

	cursor := 0

	if m.groupFilter != "" && mode == groupFilterMode {
		for i, opt := range options {
			if opt == m.groupFilter || opt == "@"+m.groupFilter {
				cursor = i

				break
			}
		}
	}

	m.groupList.Select(cursor)
	m.screen = screenGroup
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
