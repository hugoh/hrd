package ui

import (
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/theme"
	"golang.org/x/term"
)

//nolint:gochecknoglobals // lazily initialized logger, effectively package-level singleton
var (
	loggerVal *log.Logger
	loggerMu  sync.Mutex
)

func logLogger() *log.Logger {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if loggerVal == nil {
		loggerVal = log.NewWithOptions(os.Stderr, log.Options{
			ReportTimestamp: false,
			ReportCaller:    false,
		})
	}

	return loggerVal
}

// SetLogger sets the package-level logger. Used in tests to redirect output.
// Passing nil restores the default logger.
func SetLogger(l *log.Logger) {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	loggerVal = l
}

func lipglossColor(colorName string) color.Color {
	return lipgloss.Color(theme.ColorCode(colorName))
}

func MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipglossColor("gray"))
}

func Muted(s string) string {
	return MutedStyle().Render(s)
}

func WarnStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipglossColor("yellow"))
}

func StateStyle(state backend.RefState) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipglossColor(theme.StateColor(state)))
}

func ApplyColor(colorName, symbol string) string {
	return lipgloss.NewStyle().Foreground(lipglossColor(colorName)).Render(symbol)
}

// RenderDispatchResult renders one repo's dispatch result as a delimited
// block: a "=== <name> (<vcs>) ===" open line, the command output verbatim
// (with the error message appended when the command could not run), and a
// "=== <name> <status> ===" close line where status is "✓ exit 0",
// "✗ exit N", or "✗ error". When there is no body the two lines collapse
// into one. The delimiter lines are tinted by result status; ui.Print strips
// the color when stdout is redirected. The TUI renders the same block via
// RenderDispatchResultBar.
func RenderDispatchResult(res runner.Result) string {
	tier := theme.BarTierFor(res.Err, res.ExitCode)
	style := lipgloss.NewStyle().Bold(true).Foreground(lipglossColor(theme.BarTierColor(tier)))

	return renderDispatchBlock(res, func(s string) string { return style.Render(s) })
}

// RenderDispatchResultBar renders the same block as RenderDispatchResult with
// the delimiter lines drawn as full-width status-tinted background bars,
// resolved for a dark (dark=true) or light terminal background.
func RenderDispatchResultBar(res runner.Result, width int, dark bool) string {
	tier := theme.BarTierFor(res.Err, res.ExitCode)
	style := lipgloss.NewStyle().
		Background(lipgloss.Color(theme.BarBackground(tier, dark))).
		Foreground(lipgloss.Color(theme.BarForeground(tier, dark))).
		Width(width)

	return renderDispatchBlock(res, func(s string) string { return style.Render(s) })
}

func renderDispatchBlock(res runner.Result, delim func(string) string) string {
	label := res.RepoName
	if res.VCS != "" {
		label += " (" + res.VCS + ")"
	}

	body := strings.TrimRight(res.Output, "\n")
	if res.Err != nil {
		if body != "" {
			body += "\n"
		}

		body += "error: " + res.Err.Error()
	}

	status := dispatchStatus(res)

	if body == "" {
		return delim(fmt.Sprintf("=== %s %s ===", label, status))
	}

	return delim(fmt.Sprintf("=== %s ===", label)) + "\n" +
		body + "\n" +
		delim(fmt.Sprintf("=== %s %s ===", res.RepoName, status))
}

func dispatchStatus(res runner.Result) string {
	if res.Err != nil {
		return "✗ error"
	}

	glyph := "✓"
	if res.ExitCode != 0 {
		glyph = "✗"
	}

	return glyph + " exit " + strconv.Itoa(res.ExitCode)
}

type StatusLineParts struct {
	Ref      string
	Symbols  string
	Detail   string
	HasRef   bool
	HasError bool
	Error    string
}

func FormatStatusLine(status backend.RepoStatus, symStr, detail string) StatusLineParts {
	refStr := ""
	hasRef := false

	if len(status.Bookmarks) > 0 {
		refStr = status.Bookmarks[0].Name
		hasRef = true
	} else if status.Ref != "" {
		refStr = status.Ref
		hasRef = true
	}

	return StatusLineParts{
		Ref:     refStr,
		Symbols: symStr,
		Detail:  detail,
		HasRef:  hasRef,
	}
}

func FormatDispatchStatusLine(status backend.RepoStatus, includeDetail bool) string {
	symStr := theme.FormatSymbols(status, ApplyColor)
	parts := FormatStatusLine(status, symStr, "")
	style := StateStyle(status.OverallState)

	if parts.HasRef {
		combined := style.Render(fmt.Sprintf("%s %s", parts.Ref, parts.Symbols))

		if includeDetail {
			detail := FormatDetail(status.CommitMsg, status.CommitTime)
			if detail != "" {
				combined += "  " + Muted(detail)
			}
		}

		return combined
	}

	return ""
}

func Outf(format string, args ...any) {
	Print(fmt.Sprintf(format, args...) + "\n")
}

// Out prints s followed by a newline. Unlike Outf it does not interpret
// format verbs, so it is safe for dynamic strings (paths, command output).
func Out(s string) {
	Print(s + "\n")
}

// Print writes s to stdout, downsampling or stripping ANSI color to match
// the terminal's capabilities. Color is removed entirely when stdout is not
// a TTY (e.g. piped into pbcopy or a file) or when NO_COLOR is set.
func Print(s string) {
	w := colorprofile.NewWriter(os.Stdout, os.Environ())
	_, _ = w.WriteString(s)
}

func Errf(format string, args ...any) {
	logLogger().Errorf(format, args...)
}

func Warnf(format string, args ...any) {
	logLogger().Warnf(format, args...)
}

func Infof(msg string, args ...any) {
	logLogger().Infof(msg, args...)
}

func ComputeRemainderWidth(termWidth, minWidth int, usedWidths ...int) int {
	var total, numSeparators int

	for _, w := range usedWidths {
		total += w
		numSeparators++
	}

	const separatorWidth = 2

	return max(termWidth-total-numSeparators*separatorWidth, minWidth)
}

func FormatSummary(total int, failed []string) string {
	success := total - len(failed)

	if len(failed) > 0 {
		return fmt.Sprintf(
			"%d/%d repos completed successfully; failed: %s",
			success, total, strings.Join(failed, ", "),
		)
	}

	return fmt.Sprintf("%d/%d repos completed successfully", success, total)
}

func FormatDetail(msg, time string) string {
	switch {
	case msg == "":
		return time
	case time == "":
		return msg
	default:
		return msg + " " + time
	}
}

func GetTermWidth() int {
	const defaultTermWidth = 80

	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}

	return defaultTermWidth
}
