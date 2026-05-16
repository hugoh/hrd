package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/theme"
	"github.com/hugoh/hrd/internal/ui"
)

const progressBarW = 10

//nolint:gochecknoglobals // effectively constant, progress bar model
var progressModel = progress.New(
	progress.WithWidth(progressBarW),
	progress.WithoutPercentage(),
)

func (m *model) View() tea.View {
	if !m.ready {
		return tea.NewView("")
	}

	var base string
	if m.screen == screenOutput {
		base = m.outputView()
	} else {
		base = m.mainView()
	}

	var content string

	switch m.modal { //nolint:exhaustive // modalNone falls through to return base
	case modalHelp:
		content = m.overlayView(base, m.helpContent(), helpOverlayW, helpOverlayH)
	case modalGroup:
		content = m.overlayView(base, m.groupPopupContent(), groupOverlayW, groupOverlayH)
	case modalAlert:
		content = m.overlayView(base, m.alertContent(), alertOverlayW, alertOverlayH)
	default:
		content = base
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
	if len(m.repoTable.Rows()) == 0 && !m.selectMode {
		tableContent = m.emptyTableView()
	} else {
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
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.contentHeight()).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color("8")).
		Render("No repos selected\nSelect a group with @ or specific repos with Space")
}

func (m *model) renderHeader() string {
	left := styleHeader.Render(" hrd")

	if gl := m.groupLabel(); gl != "" && gl != labelAll {
		left += styleGroupLabel.Render(" " + gl)
	}

	if m.selectMode {
		left += styleSelectMarker.Render(" x:select")
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

	var right string
	if m.loading {
		right = ui.Muted(" " + m.spinner.View())
	}

	right += ui.Muted(
		" @:group x:select r:refresh ?:Help q:Quit",
	)

	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	pad = max(pad, 1)

	return left + strings.Repeat(" ", pad) + right
}

func (m *model) renderInputLine() string {
	prompt := styleWarn.Render(fmt.Sprintf("[%s] $ ", prefixLabels[m.cmdPrefix]))

	return prompt + m.input.View()
}

func (m *model) renderFooter() string {
	return styleFooter.Render("G:git J:jj S:shell f:fetch d:diff l:log p:pull P:push s:status")
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

// --- Overlays ---------------------------------------------------------------

func (m *model) overlayView(base, content string, w, h int) string {
	lines := strings.Split(base, "\n")
	if len(lines) == 0 {
		return base
	}

	x := (m.width - w) / 2    //nolint:mnd // centering
	y := (len(lines) - h) / 2 //nolint:mnd // centering

	if x < 0 {
		x = 0
	}

	switch {
	case y < 0:
		y = 0
	case y+h > len(lines):
		y = max(0, len(lines)-h)
	}

	boxLines := strings.Split(content, "\n")
	composeOverlay(lines, boxLines, x, y, w, h, m.width)

	return strings.Join(lines, "\n")
}

func composeOverlay(lines, boxLines []string, x, y, w, h, totalW int) {
	for i := 0; i < h && y+i < len(lines); i++ {
		line := lines[y+i]

		var overlayLine string
		if i < len(boxLines) {
			overlayLine = boxLines[i]
		}

		olVis := lipgloss.Width(overlayLine)
		if olVis > w {
			overlayLine = truncateVis(overlayLine, w)
			olVis = w
		}

		prefix := ansiPrefix(line, x)
		rest := ansiSuffix(line, x+olVis)

		// Pad with spaces if the base line is shorter than the overlay position.
		prefVis := lipgloss.Width(prefix)
		if prefVis < x {
			prefix += strings.Repeat(" ", x-prefVis)
		}

		combined := prefix + overlayLine + rest

		cVis := lipgloss.Width(combined)
		if cVis > totalW {
			combined = truncateVis(combined, totalW)
		} else if cVis < totalW {
			combined += strings.Repeat(" ", totalW-cVis)
		}

		lines[y+i] = combined
	}
}

// ansiPrefix returns the prefix of s containing up to visCol visual columns,
// preserving ANSI escape codes and multi-byte runes intact.
func ansiPrefix(s string, visCol int) string {
	return s[:ansiByteOff(s, visCol)]
}

// ansiSuffix returns the suffix of s starting after visCol visual columns.
func ansiSuffix(s string, visCol int) string {
	return s[ansiByteOff(s, visCol):]
}

// ansiByteOff returns the byte offset in s that corresponds to the given
// visual column position, skipping ANSI escape sequences.
func ansiByteOff(s string, visCol int) int {
	off := 0

	col := 0
	for col < visCol && off < len(s) {
		r, size := utf8.DecodeRuneInString(s[off:])
		if r != '\x1b' {
			off += size
			col++

			continue
		}

		off++ // skip ESC
		if off >= len(s) {
			continue
		}

		b := s[off]

		switch {
		case b == '[':
			off = skipCSIParams(s, off+1)
		case b >= 0x40 && b <= 0x5F:
			off++ // two-character escape
		default:
			off = skipUntilFinal(s, off)
		}
	}

	return off
}

// skipCSIParams advances off past a CSI parameter sequence.
func skipCSIParams(s string, off int) int {
	for off < len(s) && s[off] >= 0x20 && s[off] <= 0x3E {
		off++
	}

	if off < len(s) {
		off++ // final byte (0x40-0x7E)
	}

	return off
}

// skipUntilFinal advances off past an escape sequence ending at 0x30-0x7E.
func skipUntilFinal(s string, off int) int {
	for off < len(s) && s[off] >= 0x20 && s[off] <= 0x2F {
		off++
	}

	if off < len(s) && s[off] >= 0x30 && s[off] <= 0x7E {
		off++
	}

	return off
}

// truncateVis truncates s to at most maxVis visual columns,
// preserving ANSI escape codes and multi-byte runes intact.
func truncateVis(s string, maxVis int) string {
	return s[:ansiByteOff(s, maxVis)]
}

func (m *model) helpContent() string {
	content := " Help\n\n"
	content += "Navigation:\n"
	content += "  ↑/k  Move cursor up (select mode)\n"
	content += "  ↓/j  Move cursor down (select mode)\n\n"
	content += "Selection:\n"
	content += "  Space  Enter/exit selection mode\n"
	content += "  a    Select / deselect all\n"
	content += "  a    Select / deselect all\n\n"
	content += "Filtering:\n"
	content += "  @    Select group filter\n\n"
	content += "Commands:\n"
	content += "  s    Run status on selected\n"
	content += "  l    Run log on selected\n"
	content += "  d    Run diff on selected\n"
	content += "  f    Run fetch on selected\n"
	content += "  p    Run pull on selected\n"
	content += "  P    Run push on selected\n"
	content += "  G    Open command bar (git)\n"
	content += "  J    Open command bar (jj)\n"
	content += "  S    Open command bar (shell)\n\n"
	content += "General:\n"
	content += "  r    Refresh repo status\n"
	content += "  ?    Toggle this help\n"
	content += "  q / Ctrl+C  Quit"

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(1, 2).           //nolint:mnd // style padding
		Width(helpOverlayW - 4). //nolint:mnd // width minus padding
		Render(content)
}

func (m *model) groupPopupContent() string {
	items := make([]string, 0, len(m.groupPopupOptions))
	for i, opt := range m.groupPopupOptions {
		marker := "  "
		if i == m.groupPopupCursor {
			marker = "▸ "
		}

		name := opt
		if opt == m.groupFilter || opt == "@"+m.groupFilter {
			name = styleBold.Render(opt)
		}

		items = append(items, marker+name)
	}

	content := styleHeader.Render(" Select group ") + "\n" + strings.Join(items, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2).            //nolint:mnd // style padding
		Width(groupOverlayW - 4). //nolint:mnd // width minus padding
		Render(content)
}

func (m *model) alertContent() string {
	content := styleWarn.Render("No repos selected") + "\n" +
		ui.Muted("Select a group with @ or specific repos with Space")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Padding(1, 2).            //nolint:mnd // style padding
		Width(alertOverlayW - 4). //nolint:mnd // width minus padding
		Render(content)
}
