package ui

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func withProgressOverrides(t *testing.T, tty bool) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer

	t.Cleanup(func() { SetProgressOutput(nil, false) })
	SetProgressOutput(&buf, tty)

	return &buf
}

func TestProgressOSC(t *testing.T) {
	buf := withProgressOverrides(t, true)

	ProgressOSC(42, false)

	if got, want := buf.String(), ansi.SetProgressBar(42); got != want {
		t.Fatalf("ProgressOSC(42, false) wrote %q, want %q", got, want)
	}

	buf.Reset()
	ProgressOSC(10, true)

	if got, want := buf.String(), ansi.SetErrorProgressBar(10); got != want {
		t.Fatalf("ProgressOSC(10, true) wrote %q, want %q", got, want)
	}
}

func TestProgressOSCDone(t *testing.T) {
	buf := withProgressOverrides(t, true)

	ProgressOSCDone()

	if got, want := buf.String(), ansi.ResetProgressBar; got != want {
		t.Fatalf("ProgressOSCDone() wrote %q, want %q", got, want)
	}
}

func TestProgressOSCNotATerminal(t *testing.T) {
	buf := withProgressOverrides(t, false)

	ProgressOSC(50, false)
	ProgressOSCDone()

	if buf.Len() != 0 {
		t.Fatalf("expected no output when stdout isn't a terminal, got %q", buf.String())
	}
}

func TestIsTTY(t *testing.T) {
	withProgressOverrides(t, true)

	if !IsTTY() {
		t.Fatal("IsTTY() = false, want true")
	}

	withProgressOverrides(t, false)

	if IsTTY() {
		t.Fatal("IsTTY() = true, want false")
	}
}

func TestClearLiveLine(t *testing.T) {
	buf := withProgressOverrides(t, true)

	ClearLiveLine()

	if got, want := buf.String(), clearLineSeq; got != want {
		t.Fatalf("ClearLiveLine() wrote %q, want %q", got, want)
	}
}

func TestDrawLiveLine(t *testing.T) {
	buf := withProgressOverrides(t, true)

	DrawLiveLine("hello")

	if got, want := buf.String(), clearLineSeq+"hello"; got != want {
		t.Fatalf("DrawLiveLine(%q) wrote %q, want %q", "hello", got, want)
	}
}
