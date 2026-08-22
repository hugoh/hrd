package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

//nolint:gochecknoglobals // overridden in tests to avoid touching real stdout
var (
	progressWriter io.Writer = os.Stdout
	isProgressTTY            = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }
)

// ProgressOSC reports dispatch progress (0-100) to terminals that support
// the OSC 9;4 taskbar/tab progress protocol (Windows Terminal, ConEmu,
// WezTerm). failed switches the indicator to its "error" state. A no-op
// when stdout isn't a terminal.
func ProgressOSC(pct int, failed bool) {
	if !isProgressTTY() {
		return
	}

	if failed {
		_, _ = fmt.Fprint(progressWriter, ansi.SetErrorProgressBar(pct))

		return
	}

	_, _ = fmt.Fprint(progressWriter, ansi.SetProgressBar(pct))
}

// ProgressOSCDone clears the terminal progress indicator set by ProgressOSC.
func ProgressOSCDone() {
	if !isProgressTTY() {
		return
	}

	_, _ = fmt.Fprint(progressWriter, ansi.ResetProgressBar)
}

// IsTTY reports whether stdout is a terminal — the same check ProgressOSC
// gates on, exported so callers (e.g. a live-redrawn progress bar) can
// decide up front whether to engage terminal-only behavior at all, rather
// than duplicating the term.IsTerminal call.
func IsTTY() bool {
	return isProgressTTY()
}

// clearLineSeq moves the cursor to column 0 and erases the rest of the
// line — the standard "redraw a status line in place" ANSI sequence.
const clearLineSeq = "\r\x1b[2K"

// ClearLiveLine erases whatever was last drawn by DrawLiveLine, leaving the
// cursor at column 0 of that line. Safe to call even if nothing has been
// drawn yet (erasing a blank line is a no-op). Callers are expected to gate
// on IsTTY() themselves before drawing/clearing at all — unlike
// ProgressOSC/ProgressOSCDone, this does not self-gate, since a caller that
// only ever draws when IsTTY() is true never needs to clear when it isn't.
func ClearLiveLine() {
	_, _ = fmt.Fprint(progressWriter, clearLineSeq)
}

// DrawLiveLine clears the previously drawn live line and writes s in its
// place, without a trailing newline, so the cursor stays ready for the next
// ClearLiveLine/DrawLiveLine pair or for a caller to move past it with a
// plain "\n" write of its own.
func DrawLiveLine(s string) {
	_, _ = fmt.Fprint(progressWriter, clearLineSeq+s)
}

// SetProgressOutput overrides where ProgressOSC/ProgressOSCDone write and
// whether that destination is treated as a terminal. Used by tests, in this
// package and others, to observe the sequences without a real stdout tty.
// Pass w nil to restore the real os.Stdout/term.IsTerminal behavior.
func SetProgressOutput(w io.Writer, tty bool) {
	if w == nil {
		progressWriter = os.Stdout
		isProgressTTY = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

		return
	}

	progressWriter = w
	isProgressTTY = func() bool { return tty }
}
