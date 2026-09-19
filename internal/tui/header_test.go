package tui

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func headerModel(width int) *model {
	return testModel(func(m *model) {
		m.width = width
		m.nameFilter = "abc"
		m.attentionFilter = true
		m.mode = modeSelect
		m.selected = map[string]bool{"a": true}
	})
}

func TestHeaderFitsWidth(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		wantHints []string
	}{
		{name: "60 cols", width: 60, wantHints: []string{"help", "cmd", "quit"}},
		{name: "80 cols", width: 80, wantHints: []string{"help", "cmd", "quit"}},
		{name: "100 cols", width: 100, wantHints: []string{"help", "cmd", "quit"}},
		{
			name:      "wide keeps everything",
			width:     300,
			wantHints: []string{"history", "refresh", "quit"},
		},
		{name: "tiny is hard-truncated", width: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := headerModel(tt.width).renderHeader()

			require.LessOrEqual(t, lipgloss.Width(h), tt.width)

			for _, hint := range tt.wantHints {
				require.Contains(t, h, hint)
			}
		})
	}
}

func TestViewTooSmall(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		wantTooSmall  bool
	}{
		{name: "narrow", width: minViewW - 1, height: 30, wantTooSmall: true},
		{name: "short", width: 80, height: minViewH - 1, wantTooSmall: true},
		{name: "minimum fits", width: minViewW, height: minViewH},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testModel(func(m *model) {
				m.width = tt.width
				m.height = tt.height
			})

			content := m.View().Content

			if tt.wantTooSmall {
				require.Contains(t, content, "too small")
			} else {
				require.NotContains(t, content, "too small")
			}
		})
	}
}
