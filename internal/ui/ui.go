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

func FormatDispatchHeader(name, vcs string) string {
	return fmt.Sprintf(" %-15s %-3s", name, vcs)
}

//nolint:gochecknoglobals // cached dispatch result styles
var (
	dispatchHeaderStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("15"))
	dispatchErrorStyle   = lipgloss.NewStyle().Foreground(lipglossColor("red"))
	dispatchSuccessStyle = lipgloss.NewStyle().Foreground(lipglossColor("green"))
)

func RenderDispatchResult(res runner.Result) string {
	header := dispatchHeaderStyle.Render(FormatDispatchHeader(res.RepoName, res.VCS))

	if runner.ResultColor(res) == "red" {
		if res.Err != nil {
			return header + " " + dispatchErrorStyle.Render("✗ "+res.Err.Error())
		}

		return header + " " + dispatchErrorStyle.Render("✗ exit "+strconv.Itoa(res.ExitCode))
	}

	return header + " " + dispatchSuccessStyle.Render("✓")
}

type StatusLineParts struct {
	Ref      string
	Symbols  string
	Detail   string
	HasRef   bool
	HasError bool
	Error    string
}

func FormatStatusLine(status backend.RepoStatus, symStr string, detail string) StatusLineParts {
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
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", args...)
}

// Out prints s followed by a newline. Unlike Outf it does not interpret
// format verbs, so it is safe for dynamic strings (paths, command output).
func Out(s string) {
	_, _ = fmt.Fprintln(os.Stdout, s)
}

func Errf(format string, args ...any) {
	logLogger().Errorf(format, args...)
}

func Success(msg string, args ...any) {
	logLogger().Infof(msg, args...)
}

func Fail(msg string, args ...any) {
	logLogger().Errorf(msg, args...)
}

func ComputeRemainderWidth(termWidth int, minWidth int, usedWidths ...int) int {
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
