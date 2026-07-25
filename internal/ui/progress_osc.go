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
