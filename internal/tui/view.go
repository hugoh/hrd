package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/theme"
	"github.com/hugoh/hrd/internal/ui"
)

const (
	progressBarW = 10
)

//nolint:gochecknoglobals // effectively constant, progress bar model
var progressModel = progress.New(
	progress.WithWidth(progressBarW),
	progress.WithoutPercentage(),
)

func (m *model) View() tea.View {
	if !m.ready {
		return tea.NewView("")
	}

	var content string

	switch m.screen {
	case screenMain:
		content = m.mainView()
	case screenOutput:
		content = m.outputView()
	case screenHelp:
		content = m.helpView()
	case screenGroup:
		content = m.groupView()
	case screenSelHistory:
		content = m.selHistoryView()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	return v
}

func (m *model) mainView() string {
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))

	m.repoTable.SetHeight(m.contentHeight())
	m.repoTable.SetWidth(m.width)

	var tableContent string

	switch {
	case m.modal == modalAlert:
		tableContent = lipgloss.NewStyle().
			Width(m.width).
			Height(m.contentHeight()).
			Align(lipgloss.Center).
			AlignVertical(lipgloss.Center).
			Render(m.alertContent())
	case len(m.repoTable.Rows()) == 0 && m.mode != modeSelect:
		tableContent = m.emptyTableView()
	default:
		tableContent = m.repoTable.View()
	}

	parts := []string{m.renderHeader(), sep, tableContent}

	switch {
	case m.commandOpen:
		parts = append(parts, m.renderInputLine())
	case m.filterOpen:
		parts = append(parts, m.renderFilterLine())
	}

	parts = append(parts, sep, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

func (m *model) emptyTableView() string {
	msg := "No repos selected\nSelect a group with @ or specific repos with x"
	if len(m.repoOrder) == 0 {
		msg = "No repos configured\nUse `hrd repo add <path>` to add one"
	}

	return ui.MutedStyle().
		Width(m.width).
		Height(m.contentHeight()).
		Align(lipgloss.Center).
		Render(msg)
}

func (m *model) alertContent() string {
	if m.alertMsg != "" {
		return ui.WarnStyle().Render(m.alertMsg)
	}

	return ui.WarnStyle().Render("No repos selected") + "\n" +
		ui.Muted("Select a group with @ or specific repos with x")
}

func (m *model) renderHeader() string {
	left := styleHeader.Render(" hrd")

	if gl := m.groupLabel(); gl != "" && gl != labelAll {
		left += ui.WarnStyle().Render(" " + gl)
	}

	if m.nameFilter != "" {
		left += ui.WarnStyle().Render(" /" + m.nameFilter)
	}

	switch m.mode {
	case modeSelect:
		left += ui.WarnStyle().Render(" x:select")
	case modeSingle:
		left += ui.WarnStyle().Render(" s:single")
	case modeNormal:
	}

	if cnt := m.selectedCount(); cnt > 0 {
		var repoCount string
		if total := m.totalCount(); cnt == total {
			repoCount = fmt.Sprintf("%d repos", cnt)
		} else {
			repoCount = fmt.Sprintf("%d/%d repos", cnt, total)
		}

		left += ui.WarnStyle().Render(" " + repoCount)
	}

	right := m.renderHeaderRight()

	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	pad = max(pad, 1)

	return left + strings.Repeat(" ", pad) + right
}

func (m *model) renderHeaderRight() string {
	var right string

	if m.loading {
		right = ui.Muted(" " + m.spinner.View())
	}

	var parts []string

	for _, b := range mainBindings {
		if !b.hrd || b.label == "" {
			continue
		}

		if b.key == "a" && m.mode != modeSelect {
			continue
		}

		dk := b.displayKey
		if dk == "" {
			dk = b.key
		}

		parts = append(parts, dk+":"+b.label)
	}

	if m.screen == screenMain {
		parts = append(parts, "q:quit")
	}

	return right + ui.Muted(" "+strings.Join(parts, " "))
}

func (m *model) renderInputLine() string {
	return ui.WarnStyle().Render(":") + m.input.View()
}

func (m *model) renderFilterLine() string {
	return ui.WarnStyle().Render("/") + m.filterInput.View()
}

func (*model) renderFooter() string {
	var parts []string

	for _, b := range mainBindings {
		if b.hrd || b.label == "" {
			continue
		}

		dk := b.displayKey
		if dk == "" {
			dk = b.key
		}

		parts = append(parts, dk+":"+b.label)
	}

	return ui.MutedStyle().Render(strings.Join(parts, " "))
}

func (m *model) outputView() string {
	var header string
	if m.executing {
		header = styleHeader.Render(" Output " + m.spinner.View() + " ")
	} else {
		header = styleHeader.Render(" Output ")
	}

	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	m.output.SetWidth(m.width)
	m.output.SetHeight(m.contentHeight())

	var left, right string

	right = ui.MutedStyle().Render(" Esc/q:close")

	if m.executing && m.execTotal > 0 {
		done := len(m.execResults)
		pct := float64(done) / float64(m.execTotal)
		bar := progressModel.ViewAs(pct)
		left = ui.MutedStyle().Render(fmt.Sprintf(" %s [%d/%d]", bar, done, m.execTotal))
	} else if len(m.execResults) > 0 {
		left = m.coloredSummary()
	}

	pad := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))

	footer := left + strings.Repeat(" ", pad) + right

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, m.output.View(), sep, footer)
}

func (m *model) coloredSummary() string {
	var failed []string

	for _, er := range m.execResults {
		if er.result.Err != nil || er.result.ExitCode != 0 {
			failed = append(failed, er.name)
		}
	}

	text := ui.FormatSummary(m.execTotal, failed)

	if len(failed) > 0 {
		return ui.ApplyColor("red", "✗ "+text)
	}

	return ui.ApplyColor("green", "✓ "+text)
}

// --- Full-screen views ------------------------------------------------------

func (m *model) helpView() string {
	m.helpViewport.SetWidth(m.width)
	m.helpViewport.SetHeight(m.contentHeight())

	header := styleHeader.Render(" Help ")
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	footer := ui.MutedStyle().Render(" ↑/↓:scroll  j/k:scroll  Esc/q:close")

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, m.helpViewport.View(), sep, footer)
}

func (m *model) groupView() string {
	if m.groupNewInput {
		return m.groupNewInputView()
	}

	m.groupList.SetWidth(m.width)
	m.groupList.SetHeight(m.contentHeight())

	headerTxt := " Select group "
	if m.groupMode == groupAddMode {
		headerTxt = " Add to group "
	}

	header := styleHeader.Render(headerTxt)
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	footer := ui.MutedStyle().Render(" ↑/↓:navigate  Enter:select  Esc/q:close")

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, m.groupList.View(), sep, footer)
}

func (m *model) groupNewInputView() string {
	header := styleHeader.Render(" New group name ")
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	footer := ui.MutedStyle().Render(" Enter:confirm  Esc:back")

	prompt := ui.WarnStyle().Render("name: ")
	inputLine := prompt + m.input.View()

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, inputLine, sep, footer)
}

func (m *model) selHistoryView() string {
	if len(m.persState.SelectionHistory) == 0 {
		return ""
	}

	m.historyList.SetWidth(m.width)
	m.historyList.SetHeight(m.contentHeight())

	header := styleHeader.Render(" Selection History ")
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	footer := ui.MutedStyle().Render(" ↑/↓:navigate  Enter:restore  Esc/q:close")

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, m.historyList.View(), sep, footer)
}

func buildHelp(bindings []binding) string {
	type secEntry struct {
		key  string
		desc string
	}

	grouped := make(map[string][]secEntry, len(bindings))

	var sections []string

	for _, b := range bindings {
		if b.desc == "" {
			continue
		}

		if _, ok := grouped[b.section]; !ok {
			sections = append(sections, b.section)
		}

		dk := b.displayKey
		if dk == "" {
			dk = b.key
		}

		grouped[b.section] = append(grouped[b.section], secEntry{dk, b.desc})
	}

	var bld strings.Builder

	for _, sec := range sections {
		bld.WriteString(sec)
		bld.WriteString(":\n")

		for _, e := range grouped[sec] {
			fmt.Fprintf(&bld, "  %-6s %s\n", e.key, e.desc)
		}

		bld.WriteString("\n")
	}

	bld.WriteString("Status symbols:\n")

	for _, d := range theme.StatusSymbolDocs {
		fmt.Fprintf(&bld, "  %-6s %s\n", d.Symbol, d.Description)
	}

	bld.WriteString("\n")
	bld.WriteString("General:\n")
	bld.WriteString("  q      Quit (or go back from screens)\n")
	bld.WriteString("  Ctrl+C Cancel execution / Quit")

	return bld.String()
}

//nolint:gochecknoglobals // cached help text, built once at init
var helpStr string

func init() { //nolint:gochecknoinits
	helpStr = buildHelp(mainBindings)
}

func (*model) helpContent() string {
	return helpStr
}
