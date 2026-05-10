// Package ui provides terminal UI rendering for repo status and dispatch results.
package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/muesli/reflow/truncate"
	"github.com/muesli/reflow/wordwrap"
	"golang.org/x/term"
)

const (
	separatorWidth   = 2
	defaultTermWidth = 80
	colName          = 20
	colVCS           = 5
	colRef           = 14
	colFlags         = 4
)

//nolint:gochecknoglobals // UI colour and style definitions are inherently global
var (
	colorSynced   = lipgloss.Color("2")
	colorAhead    = lipgloss.Color("99")
	colorBehind   = lipgloss.Color("214")
	colorDiverged = lipgloss.Color("196")
	colorGone     = lipgloss.Color("238")
	colorNoRemote = lipgloss.Color("244")
	colorConflict = lipgloss.Color("196")
	colorDirty    = lipgloss.Color("214")
	colorMuted    = lipgloss.Color("240")
	colorLabel    = lipgloss.Color("252")
	colorVCS      = lipgloss.Color("33")
	colorVCSGit   = lipgloss.Color("244")
)

//nolint:gochecknoglobals // UI color and style definitions are inherently global
var (
	styleName   = lipgloss.NewStyle().Bold(true).Width(colName).MaxWidth(colName)
	styleVCSjj  = lipgloss.NewStyle().Foreground(colorVCS).Width(colVCS).Bold(true)
	styleVCSgit = lipgloss.NewStyle().Foreground(colorVCSGit).Width(colVCS)
	styleRef    = lipgloss.NewStyle().Foreground(colorLabel).Width(colRef).MaxWidth(colRef)
	styleMono   = lipgloss.NewStyle().Foreground(colorMuted)
	styleErr    = lipgloss.NewStyle().Foreground(colorDiverged)
	styleDim    = lipgloss.NewStyle().Foreground(colorMuted)
	styleBold   = lipgloss.NewStyle().Bold(true)
)

func badgeStyle(state backend.RefState, conflict bool) lipgloss.Style {
	if conflict {
		return lipgloss.NewStyle().Foreground(colorConflict).Bold(true).Padding(0, 1)
	}

	switch state {
	case backend.RefStateSynced:
		return lipgloss.NewStyle().Foreground(colorSynced).Padding(0, 1)
	case backend.RefStateAhead:
		return lipgloss.NewStyle().Foreground(colorAhead).Bold(true).Padding(0, 1)
	case backend.RefStateBehind:
		return lipgloss.NewStyle().Foreground(colorBehind).Bold(true).Padding(0, 1)
	case backend.RefStateDiverged:
		return lipgloss.NewStyle().Foreground(colorDiverged).Bold(true).Padding(0, 1)
	case backend.RefStateGone:
		return lipgloss.NewStyle().Foreground(colorGone).Strikethrough(true).Padding(0, 1)
	case backend.RefStateUnknown:
		return lipgloss.NewStyle().Foreground(colorNoRemote).Padding(0, 1)
	case backend.RefStateNoRemote:
		return lipgloss.NewStyle().Foreground(colorNoRemote).Padding(0, 1)
	default:
		return lipgloss.NewStyle().Foreground(colorNoRemote).Padding(0, 1)
	}
}

// RenderBookmarkBadge renders a bookmark with sync state indicator.
func RenderBookmarkBadge(bmk backend.BookmarkStatus) string {
	var label strings.Builder
	label.WriteString(bmk.Name)

	switch {
	case bmk.Conflict:
		label.WriteString(" !")
	case bmk.State == backend.RefStateSynced:
		label.WriteString(" ✓")
	case bmk.State == backend.RefStateAhead:
		label.WriteString(" ↑" + strconv.Itoa(bmk.Ahead))
	case bmk.State == backend.RefStateBehind:
		label.WriteString(" ↓" + strconv.Itoa(bmk.Behind))
	case bmk.State == backend.RefStateDiverged:
		if bmk.Ahead > 0 {
			label.WriteString(" ↑" + strconv.Itoa(bmk.Ahead))
		}

		if bmk.Behind > 0 {
			label.WriteString("↓" + strconv.Itoa(bmk.Behind))
		}
	case bmk.State == backend.RefStateGone:
		label.WriteString(" ✗")
	}

	return badgeStyle(bmk.State, bmk.Conflict).Render(label.String())
}

// RenderFlags renders dirty and conflict flags for a repo.
func RenderFlags(status backend.RepoStatus) string {
	var builder strings.Builder
	if status.Dirty {
		builder.WriteString(lipgloss.NewStyle().Foreground(colorDirty).Bold(true).Render("*"))
	} else {
		builder.WriteString(" ")
	}

	if status.Conflict {
		builder.WriteString(lipgloss.NewStyle().Foreground(colorConflict).Bold(true).Render("‼"))
	} else {
		builder.WriteString(" ")
	}

	return builder.String()
}

// RenderVCS renders the VCS name with appropriate styling.
func RenderVCS(vcs string) string {
	if vcs == "jj" {
		return styleVCSjj.Render("jj")
	}

	return styleVCSgit.Render("git")
}

// RenderStatusLine renders a full status line for a repo.
func RenderStatusLine(name, vcs string, status backend.RepoStatus) string {
	var ref string
	if vcs == "jj" {
		ref = styleRef.Inherit(styleMono).Render(status.Ref)
	} else {
		ref = styleRef.Render(status.Ref)
	}

	badges := make([]string, 0, len(status.Bookmarks))
	for _, bm := range status.Bookmarks {
		badges = append(badges, RenderBookmarkBadge(bm))
	}

	bookmarkCol := strings.Join(badges, " ")
	if len(badges) == 0 {
		bookmarkCol = styleDim.Render("(no bookmarks)")
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		styleName.Render(name),
		RenderVCS(vcs),
		ref,
		lipgloss.NewStyle().Width(colFlags).Render(RenderFlags(status)),
		bookmarkCol,
	)
}

// RenderDispatchResult renders a dispatch result (success/failure).
func RenderDispatchResult(res runner.Result) string {
	name := styleBold.Width(colName).Render(res.RepoName)
	if res.Err != nil {
		return name + " " + styleErr.Render("✗ "+res.Err.Error())
	}

	if res.ExitCode != 0 {
		return name + " " + styleErr.Render("✗ exit "+strconv.Itoa(res.ExitCode))
	}

	return name + " " + lipgloss.NewStyle().Foreground(colorSynced).Bold(true).Render("✓")
}

// Outf prints to stdout with formatting.
func Outf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", args...)
}

// Errf prints to stderr with formatting.
func Errf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Success prints a green success message to stderr.
func Success(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s\n", text.Colors{text.FgGreen}.Sprintf(msg, args...))
}

// Warn prints a yellow warning message to stderr.
func Warn(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s\n", text.Colors{text.FgYellow}.Sprintf(msg, args...))
}

// Info prints an cyan info message to stderr.
func Info(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s\n", text.Colors{text.FgCyan}.Sprintf(msg, args...))
}

// Fail prints a red error message to stderr.
func Fail(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s\n", text.Colors{text.FgRed}.Sprintf(msg, args...))
}

// ColorSprint prints a string with the given text colors.
func ColorSprint(c text.Colors, s string) string {
	return c.Sprint(s)
}

// TableStyle returns the table styling for status output.
func TableStyle() table.Style {
	return table.Style{
		Box: table.StyleBoxDefault,
		Color: table.ColorOptions{
			Header: text.Colors{text.Bold, text.FgHiCyan},
		},
	}
}

// Truncate truncates a string to the given maximum length.
func Truncate(s string, maxLen int) string {
	return truncate.String(s, uint(maxLen))
}

// Wrap wraps text to the given width.
func Wrap(s string, maxLen int) string {
	return wordwrap.String(s, maxLen)
}

// ComputeRemainderWidth calculates remaining width after accounting for used columns.
func ComputeRemainderWidth(termWidth, minWidth, numSeparators int, usedWidths ...int) int {
	total := 0

	for _, w := range usedWidths {
		total += w
	}

	return max(termWidth-total-numSeparators*separatorWidth, minWidth)
}

// NewTable creates a new table writer for status output.
//
//nolint:ireturn // factory returning library interface is the canonical usage
func NewTable() table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(TableStyle())

	return t
}

// GetTermWidth returns the current terminal width.
func GetTermWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}

	return defaultTermWidth
}
