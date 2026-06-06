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
	progressBarW    = 10
	historyIndent   = 4
	historyMinWrapW = 10
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

// --- Main screen ------------------------------------------------------------

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
	if m.commandOpen {
		parts = append(parts, m.renderInputLine())
	}

	parts = append(parts, sep, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

func (m *model) emptyTableView() string {
	msg := "No repos selected\nSelect a group with @ or specific repos with x"
	if len(m.repoOrder) == 0 {
		msg = "No repos configured\nUse `hrd repo add <path>` to add one"
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.contentHeight()).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color("8")).
		Render(msg)
}

func (m *model) alertContent() string {
	if m.alertMsg != "" {
		return styleWarn.Render(m.alertMsg)
	}

	return styleWarn.Render("No repos selected") + "\n" +
		ui.Muted("Select a group with @ or specific repos with x")
}

func (m *model) renderHeader() string {
	left := styleHeader.Render(" hrd")

	if gl := m.groupLabel(); gl != "" && gl != labelAll {
		left += styleGroupLabel.Render(" " + gl)
	}

	switch m.mode {
	case modeSelect:
		left += styleSelectMarker.Render(" x:select")
	case modeSingle:
		left += styleSelectMarker.Render(" s:single")
	case modeNormal:
	}

	if cnt := m.selectedCount(); cnt > 0 {
		var repoCount string
		if total := m.totalCount(); cnt == total {
			repoCount = fmt.Sprintf("%d repos", cnt)
		} else {
			repoCount = fmt.Sprintf("%d/%d repos", cnt, total)
		}

		left += styleSelectedCount.Render(" " + repoCount)
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
	prompt := styleWarn.Render(fmt.Sprintf("[%s] $ ", prefixLabels[m.cmdPrefix]))

	return prompt + m.input.View()
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

	return styleFooter.Render(strings.Join(parts, " "))
}

// --- Output screen ----------------------------------------------------------

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

	right = styleFooter.Render(" Esc/q:close")

	if m.executing && m.execTotal > 0 {
		done := len(m.execResults)
		pct := float64(done) / float64(m.execTotal)
		bar := progressModel.ViewAs(pct)
		left = styleFooter.Render(fmt.Sprintf(" %s [%d/%d]", bar, done, m.execTotal))
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
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorCode("red"))).
			Render("✗ " + text)
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.ColorCode("green"))).
		Render("✓ " + text)
}

// --- Full-screen views ------------------------------------------------------

func (m *model) helpView() string {
	m.helpViewport.SetWidth(m.width)
	m.helpViewport.SetHeight(m.contentHeight())

	header := styleHeader.Render(" Help ")
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	footer := styleFooter.Render(" ↑/↓:scroll  j/k:scroll  Esc/q:close")

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, m.helpViewport.View(), sep, footer)
}

func (m *model) groupView() string {
	if m.groupNewInput {
		return m.groupNewInputView()
	}

	items := m.renderGroupItems()

	ch := m.contentHeight()

	var visible []string

	if len(items) <= ch {
		visible = items
	} else {
		scrollOff := m.groupPopupCursor

		if scrollOff+ch > len(items) {
			scrollOff = len(items) - ch
		}

		visible = items[scrollOff : scrollOff+ch]
	}

	content := strings.Join(visible, "\n")

	headerTxt := " Select group "
	if m.groupMode == groupAddMode {
		headerTxt = " Add to group "
	}

	header := styleHeader.Render(headerTxt)
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	footer := styleFooter.Render(" ↑/↓:navigate  Enter:select  Esc/q:close")

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, content, sep, footer)
}

func (m *model) groupNewInputView() string {
	header := styleHeader.Render(" New group name ")
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	footer := styleFooter.Render(" Enter:confirm  Esc:back")

	prompt := styleWarn.Render("name: ")
	inputLine := prompt + m.input.View()

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, inputLine, sep, footer)
}

func (m *model) renderGroupItems() []string {
	items := make([]string, 0, len(m.groupPopupOptions))
	for i, opt := range m.groupPopupOptions {
		marker := "  "
		if i == m.groupPopupCursor {
			marker = "▸ "
		}

		name := opt
		if m.groupMode == groupFilterMode && (opt == m.groupFilter || opt == "@"+m.groupFilter) {
			name = styleBold.Render(opt)
		}

		items = append(items, marker+name)
	}

	return items
}

//nolint:funlen // detailed entry rendering with repo list wrapping
func (m *model) selHistoryView() string {
	entries := m.persState.SelectionHistory
	if len(entries) == 0 {
		return ""
	}

	allRepoSet := make(map[string]struct{}, len(m.cfg.Repos))
	for name := range m.cfg.Repos {
		allRepoSet[name] = struct{}{}
	}

	ch := m.contentHeight()

	wrapW := max(m.width-historyIndent, historyMinWrapW)

	type entryMeta struct{ start, count int }

	var (
		lines []string
		meta  []entryMeta
	)

	for i, e := range entries {
		start := len(lines)

		marker := "  "
		if i == m.selHistoryCursor {
			marker = "▸ "
		}

		ts := e.Timestamp.Format("Jan 02 15:04")
		header := fmt.Sprintf("%s%s  %s", marker, ui.Muted(ts), m.repoCountLabel(len(e.Repos)))
		lines = append(lines, header)

		var displayStr string

		if groups, uncovered := matchingGroups(e.Repos, allRepoSet, m.cfg.Groups); len(groups) > 0 {
			parts := make([]string, 0, len(groups)+len(uncovered))
			parts = append(parts, groups...)
			parts = append(parts, uncovered...)
			displayStr = strings.Join(parts, ", ")
		} else {
			displayStr = strings.Join(e.Repos, ", ")
		}

		for _, w := range wrapString(displayStr, wrapW) {
			lines = append(lines, "  "+w)
		}

		if i < len(entries)-1 {
			lines = append(lines, "")
		}

		meta = append(meta, entryMeta{start, len(lines) - start})
	}

	var visible []string

	if len(lines) <= ch {
		visible = lines
	} else {
		cursorStart := meta[m.selHistoryCursor].start

		off := cursorStart
		if off+ch > len(lines) {
			off = len(lines) - ch
		}

		visible = lines[off : off+ch]
	}

	content := strings.Join(visible, "\n")
	header := styleHeader.Render(" Selection History ")
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	footer := styleFooter.Render(" ↑/↓:navigate  Enter:restore  Esc/q:close")

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, content, sep, footer)
}

func wrapString(s string, maxW int) []string {
	if len(s) <= maxW {
		return []string{s}
	}

	var out []string

	for len(s) > 0 {
		if len(s) <= maxW {
			out = append(out, s)

			break
		}

		idx := strings.LastIndex(s[:maxW], ", ")
		if idx < 0 {
			idx = strings.LastIndex(s[:maxW], " ")
		}

		if idx < 0 {
			idx = maxW
		} else {
			idx += 2 // include separator
		}

		out = append(out, s[:idx])
		s = s[idx:]
		s = strings.TrimLeft(s, " ")
	}

	return out
}

func (*model) repoCountLabel(n int) string {
	if n == 1 {
		return "1 repo"
	}

	return fmt.Sprintf("%d repos", n)
}

func buildHelp(bindings []binding) string {
	type secEntry struct {
		key  string
		desc string
	}

	grouped := make(map[string][]secEntry)

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

//nolint:gochecknoglobals // pre-built help string from bindings
var helpStr string

func init() { //nolint:gochecknoinits
	helpStr = buildHelp(mainBindings)
}

func (*model) helpContent() string {
	return helpStr
}
