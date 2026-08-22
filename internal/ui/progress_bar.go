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
	progressBarMaxWidth = 40
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
func RenderProgressBar(pct float64, width int) string {
	return progress.New(progress.WithWidth(width), progress.WithoutPercentage()).ViewAs(pct)
}

// ProgressBarWidth computes a bar width that fills the terminal line after
// reserving room for reservedWidth columns of surrounding status text,
// clamped to [progressBarMinWidth, progressBarMaxWidth] so the bar neither
// collapses on a narrow terminal nor dominates an ultra-wide one.
func ProgressBarWidth(reservedWidth int) int {
	return min(ComputeRemainderWidth(GetTermWidth(), progressBarMinWidth, reservedWidth), progressBarMaxWidth)
}

// TextWidth returns the rendered terminal width of s, ignoring ANSI escape
// sequences (e.g. color codes applied by ApplyColor/MutedStyle).
func TextWidth(s string) int {
	return lipgloss.Width(s)
}
