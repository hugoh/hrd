package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatETA(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0:00"},
		{"seconds only", 8 * time.Second, "0:08"},
		{"rounds up", 8*time.Second + 600*time.Millisecond, "0:09"},
		{"minutes and seconds", 2*time.Minute + 3*time.Second, "2:03"},
		{"over an hour still m:ss", 61 * time.Minute, "61:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatETA(tt.d))
		})
	}
}

func TestEstimateETA(t *testing.T) {
	t.Run("no progress yet", func(t *testing.T) {
		_, ok := EstimateETA(time.Now(), 0, 10)
		assert.False(t, ok)
	})

	t.Run("done reached total", func(t *testing.T) {
		_, ok := EstimateETA(time.Now().Add(-time.Second), 10, 10)
		assert.False(t, ok)
	})

	t.Run("zero start", func(t *testing.T) {
		_, ok := EstimateETA(time.Time{}, 1, 10)
		assert.False(t, ok)
	})

	t.Run("linear extrapolation", func(t *testing.T) {
		start := time.Now().Add(-10 * time.Second)

		eta, ok := EstimateETA(start, 2, 10)
		assert.True(t, ok)
		// elapsed ~10s over 2 done => ~5s/item * 8 remaining ~= 40s
		assert.InDelta(t, 40*time.Second, eta, float64(2*time.Second))
	})
}

func TestRenderProgressBar(t *testing.T) {
	bar := RenderProgressBar(0.5, 20)
	assert.Equal(t, 20, TextWidth(bar))
}

// TestRenderProgressBarUsesFullBlockGlyphs guards against the underlying
// library's default fill glyph ('▌', a half block reserved for multi-color
// blend rendering): a plain solid fill using that glyph only paints half of
// each cell, rendering as visibly disjointed rather than a solid bar.
func TestRenderProgressBarUsesFullBlockGlyphs(t *testing.T) {
	bar := RenderProgressBar(0.5, 20)
	assert.Contains(t, bar, "█", "filled portion should use the full block glyph")
	assert.NotContains(t, bar, "▌", "must not use the library's disjointed half-block default")
}

func TestTextWidth(t *testing.T) {
	plain := "hello"
	colored := ApplyColor("green", plain)
	assert.NotEqual(t, plain, colored, "ApplyColor should add escape codes")
	assert.Equal(t, len(plain), TextWidth(colored))
}

func TestProgressBarWidth(t *testing.T) {
	// GetTermWidth falls back to 80 when stdout isn't a real terminal, as
	// is the case under `go test` — which happens to equal progressBarMaxWidth,
	// so ProgressBarWidth itself can never observe the max clamp engaging
	// here. ProgressBarWidthFor (below) tests that path directly against an
	// arbitrary width instead of the real terminal fallback.
	const termWidth = 80

	t.Run("clamped to min on very little room", func(t *testing.T) {
		assert.Equal(t, progressBarMinWidth, ProgressBarWidth(termWidth))
	})

	t.Run("fills remaining space between bounds", func(t *testing.T) {
		reserved := 50
		want := termWidth - reserved - 2 // ComputeRemainderWidth's separator accounting
		assert.Equal(t, want, ProgressBarWidth(reserved))
	})
}

func TestProgressBarWidthFor(t *testing.T) {
	t.Run("clamped to max on a very wide terminal", func(t *testing.T) {
		assert.Equal(t, progressBarMaxWidth, ProgressBarWidthFor(300, 5))
	})

	t.Run("clamped to min on very little room", func(t *testing.T) {
		assert.Equal(t, progressBarMinWidth, ProgressBarWidthFor(80, 80))
	})

	t.Run("fills remaining space between bounds", func(t *testing.T) {
		want := 80 - 50 - 2 // ComputeRemainderWidth's separator accounting
		assert.Equal(t, want, ProgressBarWidthFor(80, 50))
	})
}
