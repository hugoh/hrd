package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/hugoh/hrd/internal/theme"
	"github.com/hugoh/hrd/internal/ui"
)

const (
	progressBarW = 30

	// progressPercentMax converts a done/total ratio to the 0-100 scale
	// expected by tea.ProgressBar.
	progressPercentMax = 100
)

// newProgressBar constructs a fresh exec-progress bar model, forcing the
// full-block fill/empty glyphs. The library's default fill glyph is a half
// block ('▌'), meant to double blending resolution for multi-color
// gradients, but a plain solid fill only paints that glyph's foreground
// half, leaving the other half of each cell unpainted — which renders as a
// visibly disjointed bar rather than a solid one. WithDefaultBlend matches
// bubbletea's progress-animated example (purple haze to neon pink).
func newProgressBar() progress.Model {
	return progress.New(
		progress.WithWidth(progressBarW),
		progress.WithoutPercentage(),
		progress.WithFillCharacters(
			progress.DefaultFullCharFullBlock,
			progress.DefaultEmptyCharBlock,
		),
		progress.WithDefaultBlend(),
	)
}

func (m *model) View() tea.View {
	if !m.ready {
		return tea.NewView("")
	}

	var content string

	switch {
	case m.tooSmall():
		content = m.tooSmallView()
	default:
		content = m.screenView()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.ProgressBar = m.progressBar()

	return v
}

func (m *model) tooSmall() bool {
	return m.width < minViewW || m.height < minViewH
}

func (m *model) tooSmallView() string {
	return fmt.Sprintf("Terminal too small (%dx%d)\nneed at least %dx%d",
		m.width, m.height, minViewW, minViewH)
}

func (m *model) screenView() string {
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

	return content
}

// progressBar reports run progress to terminals that show it in their
// tab/taskbar (OSC 9;4). It returns nil when nothing is running, which
// clears the indicator. Declaring it on the View lets the renderer emit it
// in step with frames instead of writing to stdout behind its back.
func (m *model) progressBar() *tea.ProgressBar {
	switch {
	case m.executing && m.execTotal > 0:
		_, failed := m.execCounts()

		return newTerminalProgress(len(m.execResults), m.execTotal, failed > 0)
	case len(m.pending) > 0 && m.statusTotal > 0:
		return newTerminalProgress(m.statusTotal-len(m.pending), m.statusTotal, m.statusAnyErr)
	}

	return nil
}

func newTerminalProgress(done, total int, failed bool) *tea.ProgressBar {
	state := tea.ProgressBarDefault
	if failed {
		state = tea.ProgressBarError
	}

	return tea.NewProgressBar(state, done*progressPercentMax/total)
}

func (m *model) mainView() string {
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))

	m.repoTable.SetHeight(m.contentHeight())
	m.repoTable.SetWidth(m.width)

	var tableContent string

	switch {
	case m.modal == modalAlert:
		tableContent = m.alertBox()
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

// alertBox renders the alert centered over the whole content area, for
// screens to show in place of their usual body.
func (m *model) alertBox() string {
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.contentHeight()).
		Align(lipgloss.Center).
		AlignVertical(lipgloss.Center).
		Render(m.alertContent())
}

func (m *model) alertContent() string {
	if m.alertMsg != "" {
		return ui.WarnStyle().Render(m.alertMsg)
	}

	return ui.WarnStyle().Render("No repos selected") + "\n" +
		ui.Muted("Select a group with @ or specific repos with x")
}

// renderHeaderLeft renders the title plus the active filter/mode/count chips.
func (m *model) renderHeaderLeft() string {
	left := styleHeader.Render(" hrd")

	if gl := m.groupLabel(); gl != "" && gl != labelAll {
		left += ui.WarnStyle().Render(" " + gl)
	}

	if m.nameFilter != "" {
		left += ui.WarnStyle().Render(" /" + m.nameFilter)
	}

	if m.attentionFilter {
		left += ui.WarnStyle().Render(" *attention")
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

	return left
}

func (m *model) renderHeader() string {
	left := m.renderHeaderLeft()

	right := m.renderHeaderRight(lipgloss.Width(left) + 1) // +1: minimum gap

	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	pad = max(pad, 1)

	line := left + strings.Repeat(" ", pad) + right
	if m.width > 0 {
		line = ansi.Truncate(line, m.width, "")
	}

	return line
}

// renderHint renders a "key:label" hint with the key highlighted and the
// label muted.
func renderHint(key, label string) string {
	return styleHintKey.Render(key) + ui.MutedStyle().Render(":"+label)
}

// headerHint is one right-hand header hint; essential hints survive when the
// terminal is too narrow to show them all.
type headerHint struct {
	text      string
	essential bool
}

// renderHeaderRight renders the right-hand hint list, dropping the
// least-essential hints (latest first) until it fits beside leftWidth cells
// of left-hand content.
func (m *model) renderHeaderRight(leftWidth int) string {
	var spin string

	if m.loading {
		spin = ui.Muted(" " + m.spinner.View())
	}

	hints := m.headerHints()

	for {
		texts := make([]string, len(hints))
		for i, h := range hints {
			texts[i] = h.text
		}

		right := spin + " " + strings.Join(texts, " ")

		if m.width <= 0 || leftWidth+lipgloss.Width(right) <= m.width {
			return right
		}

		drop := -1

		for i, hint := range slices.Backward(hints) {
			if !hint.essential {
				drop = i

				break
			}
		}

		if drop < 0 {
			return right
		}

		hints = slices.Delete(hints, drop, drop+1)
	}
}

func (m *model) headerHints() []headerHint {
	var hints []headerHint

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

		hints = append(hints, headerHint{
			text:      renderHint(dk, b.label),
			essential: b.key == "?" || b.key == ":",
		})
	}

	if m.screen == screenMain {
		hints = append(hints, headerHint{text: renderHint("q", "quit"), essential: true})
	}

	return hints
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

		parts = append(parts, renderHint(dk, b.label))
	}

	return strings.Join(parts, " ")
}

func (m *model) outputView() string {
	label := m.execLabel
	if label == "" {
		label = "Output"
	}

	var header string
	if m.executing {
		header = styleHeader.Render(" " + label + " " + m.spinner.View() + " ")
	} else {
		header = styleHeader.Render(" " + label + " ")
	}

	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	m.output.SetWidth(m.width)
	m.output.SetHeight(m.contentHeight())

	var left, right string

	right = ui.MutedStyle().Render(" Enter/o/Esc/q:close")
	if len(m.execResultOffsets) > 1 {
		right = ui.MutedStyle().Render(" ,/.:prev/next  Enter/o/Esc/q:close")
	}

	if m.executing && m.execTotal > 0 {
		done := len(m.execResults)

		succeeded, failed := m.execCounts()
		counts := ui.ApplyColor("green", fmt.Sprintf("✓%d", succeeded))

		if failed > 0 {
			counts += " " + ui.ApplyColor("red", fmt.Sprintf("✗%d", failed))
		}

		suffix := ui.MutedStyle().Render(fmt.Sprintf(" [%d/%d]", done, m.execTotal)) + " " + counts

		if eta, ok := m.execETA(done); ok {
			suffix += ui.MutedStyle().Render("  ETA " + formatETA(eta))
		}

		// Width adapts to the TUI's own tracked width (from
		// tea.WindowSizeMsg) rather than a fixed constant, so the bar grows
		// on a wide terminal instead of staying pinned at its initial size.
		m.progress.SetWidth(ui.ProgressBarWidthFor(m.width, lipgloss.Width(suffix)+1))

		left = " " + m.progress.View() + suffix
	} else if len(m.execResults) > 0 {
		left = m.coloredSummary()
	}

	pad := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))

	footer := left + strings.Repeat(" ", pad) + right

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, m.output.View(), sep, footer)
}

// execCounts tallies how many completed results so far succeeded vs.
// failed, for the live progress bar footer.
func (m *model) execCounts() (int, int) {
	var succeeded, failed int

	for _, er := range m.execResults {
		if er.result.Err != nil || er.result.ExitCode != 0 {
			failed++
		} else {
			succeeded++
		}
	}

	return succeeded, failed
}

// execETA linearly extrapolates a remaining-time estimate from overall
// wall-clock progress so far. This implicitly accounts for the configured
// concurrency, since elapsed time already reflects however many repos ran
// in parallel. Returns false before enough progress has been made to give
// a meaningful estimate, or once the run is complete.
func (m *model) execETA(done int) (time.Duration, bool) {
	return ui.EstimateETA(m.execStartTime, done, m.execTotal)
}

// formatETA renders a duration as "m:ss", rounded to the nearest second.
func formatETA(d time.Duration) string {
	return ui.FormatETA(d)
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
	footer := " " + renderHint(
		"↑/↓",
		"scroll",
	) + "  " + renderHint(
		"j/k",
		"scroll",
	) + "  " + renderHint(
		"Esc/q",
		"close",
	)

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

	body := m.groupList.View()
	if m.modal == modalAlert {
		body = m.alertBox()
	}

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, body, sep, footer)
}

func (m *model) groupNewInputView() string {
	header := styleHeader.Render(" New group name ")
	sep := styleSeparator.Render(strings.Repeat(separatorChar, m.width))
	footer := ui.MutedStyle().Render(" Enter:confirm  Esc:back")

	prompt := ui.WarnStyle().Render("name: ")

	body := prompt + m.input.View()
	if m.modal == modalAlert {
		body = m.alertBox()
	}

	return lipgloss.JoinVertical(lipgloss.Top, header, sep, body, sep, footer)
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
			fmt.Fprintf(&bld, "  %s %s\n", styleHintKey.Render(fmt.Sprintf("%-6s", e.key)), e.desc)
		}

		bld.WriteString("\n")
	}

	bld.WriteString("Status symbols:\n")

	for _, d := range theme.StatusSymbolDocs {
		fmt.Fprintf(&bld, "  %-6s %s\n", d.Symbol, d.Description)
	}

	bld.WriteString("\n")
	bld.WriteString("General:\n")
	fmt.Fprintf(
		&bld,
		"  %s Quit (or go back from screens)\n",
		styleHintKey.Render(fmt.Sprintf("%-6s", "q")),
	)
	fmt.Fprintf(
		&bld,
		"  %s Cancel execution / Quit",
		styleHintKey.Render(fmt.Sprintf("%-6s", "Ctrl+C")),
	)

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
