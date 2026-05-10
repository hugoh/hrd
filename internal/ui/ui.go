// Package ui provides terminal UI rendering for repo status and dispatch results.
package ui

import (
	"fmt"
	"os"
	"strconv"

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
)

var tableStyle = table.Style{ //nolint:gochecknoglobals
	Box: table.StyleBoxDefault,
	Color: table.ColorOptions{
		Header: text.Colors{text.Bold, text.FgHiCyan},
	},
}

// RenderDispatchResult renders a dispatch result (success/failure).
func RenderDispatchResult(res runner.Result) string {
	header := text.Colors{text.BgHiBlack, text.FgHiWhite}.Sprintf(" %-19s", res.RepoName)

	if res.Err != nil {
		return header + " " + text.FgRed.Sprint("✗ "+res.Err.Error())
	}

	if res.ExitCode != 0 {
		return header + " " + text.FgRed.Sprint("✗ exit "+strconv.Itoa(res.ExitCode))
	}

	return header + " " + text.Colors{text.Bold, text.FgGreen}.Sprint("✓")
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

// Truncate truncates a string to the given maximum length.
func Truncate(s string, maxLen int) string {
	return truncate.String(s, uint(maxLen))
}

// Wrap wraps text to the given width.
func Wrap(s string, maxLen int) string {
	return wordwrap.String(s, maxLen)
}

// ComputeRemainderWidth calculates remaining width after accounting for used columns.
func ComputeRemainderWidth(termWidth int, minWidth int, usedWidths ...int) int {
	var total, numSeparators int

	for _, w := range usedWidths {
		total += w
		numSeparators++
	}

	return max(termWidth-total-numSeparators*separatorWidth, minWidth)
}

// NewTable creates a new table writer for status output.
//
//nolint:ireturn // factory returning library interface is the canonical usage
func NewTable() table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(tableStyle)

	return t
}

// GetTermWidth returns the current terminal width.
func GetTermWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}

	return defaultTermWidth
}
