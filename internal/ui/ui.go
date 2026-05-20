package ui

import (
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/theme"
	"golang.org/x/term"
)

func lipglossColor(colorName string) color.Color {
	return lipgloss.Color(theme.ColorCode(colorName))
}

func Muted(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(s)
}

func MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
}

func StateStyle(state backend.RefState) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipglossColor(theme.StateColor(state)))
}

func ApplyColor(colorName, symbol string) string {
	return lipgloss.NewStyle().Foreground(lipglossColor(colorName)).Render(symbol)
}

func ColorSprint(colorName, s string) string {
	return lipgloss.NewStyle().Foreground(lipglossColor(colorName)).Render(s)
}

func FormatDispatchHeader(name, vcs string) string {
	return fmt.Sprintf(" %-15s %-3s", name, vcs)
}

func RenderDispatchResult(res runner.Result) string {
	headerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("15"))
	header := headerStyle.Render(FormatDispatchHeader(res.RepoName, res.VCS))

	if runner.ResultColor(res) == "red" {
		if res.Err != nil {
			return header + " " + lipgloss.NewStyle().
				Foreground(lipgloss.Color("1")).
				Render("✗ "+res.Err.Error())
		}

		return header + " " + lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Render("✗ exit "+strconv.Itoa(res.ExitCode))
	}

	return header + " " + lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓")
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

func Errf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func Success(msg string, args ...any) {
	fmt.Fprintf(
		os.Stderr,
		"%s\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(fmt.Sprintf(msg, args...)),
	)
}

func Fail(msg string, args ...any) {
	fmt.Fprintf(
		os.Stderr,
		"%s\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(fmt.Sprintf(msg, args...)),
	)
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
