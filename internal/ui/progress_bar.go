package ui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
)

// Bar width bounds for ProgressBarWidth: kept small enough to stay legible
// alongside status text on a narrow terminal, capped so it doesn't
// dominate the line on an ultra-wide one.
const (
	progressBarMinWidth = 10
	progressBarMaxWidth = 80
)

// FormatETA renders a duration as "m:ss", rounded to the nearest second.
func FormatETA(d time.Duration) string {
	d = d.Round(time.Second)
	mins := d / time.Minute
	secs := (d % time.Minute) / time.Second

	return fmt.Sprintf("%d:%02d", mins, secs)
}

// EstimateETA linearly extrapolates a remaining-time estimate from overall
// wall-clock progress so far: (elapsed / done) * remaining. This implicitly
// accounts for concurrency, since elapsed time already reflects however
// many items ran in parallel. Returns false before enough progress has
// been made for a meaningful estimate (done == 0), once the run is
// complete (done >= total), or if start is zero.
func EstimateETA(start time.Time, done, total int) (time.Duration, bool) {
	if done == 0 || done >= total || start.IsZero() {
		return 0, false
	}

	elapsed := time.Since(start)
	remaining := total - done

	return elapsed / time.Duration(done) * time.Duration(remaining), true
}

// RenderProgressBar renders a filled/empty bar of the given width for pct
// (0-1), matching the TUI's exec progress bar styling.
//
// WithFillCharacters forces full-block glyphs: the library's default fill
// glyph is a half block ('▌'), meant to double blending resolution for
// multi-color gradients, but a plain solid fill only paints that glyph's
// foreground half, leaving the other half of each cell unpainted — which
// renders as a visibly disjointed bar rather than a solid one.
func RenderProgressBar(pct float64, width int) string {
	return progress.New(
		progress.WithWidth(width),
		progress.WithoutPercentage(),
		progress.WithFillCharacters(progress.DefaultFullCharFullBlock, progress.DefaultEmptyCharBlock),
	).ViewAs(pct)
}

// ProgressBarWidth computes a bar width that fills the terminal line after
// reserving room for reservedWidth columns of surrounding status text,
// clamped to [progressBarMinWidth, progressBarMaxWidth] so the bar neither
// collapses on a narrow terminal nor dominates an ultra-wide one.
func ProgressBarWidth(reservedWidth int) int {
	return ProgressBarWidthFor(GetTermWidth(), reservedWidth)
}

// ProgressBarWidthFor is ProgressBarWidth parametrized on the available
// width instead of querying the real terminal via GetTermWidth() — for a
// caller that already tracks its own width (e.g. the TUI, via bubbletea's
// WindowSizeMsg), where querying the terminal directly would be redundant
// and, inside the TUI's raw-mode input handling, unsafe.
func ProgressBarWidthFor(width, reservedWidth int) int {
	return min(ComputeRemainderWidth(width, progressBarMinWidth, reservedWidth), progressBarMaxWidth)
}

// TextWidth returns the rendered terminal width of s, ignoring ANSI escape
// sequences (e.g. color codes applied by ApplyColor/MutedStyle).
func TextWidth(s string) int {
	return lipgloss.Width(s)
}
