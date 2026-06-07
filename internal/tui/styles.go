package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/theme"
)

//nolint:gochecknoglobals // effectively constant, lipgloss styles
var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(theme.ColorCode("cyan")))

	styleSeparator = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.ColorCode("gray")))

	styleBold = lipgloss.NewStyle().
			Bold(true)
)

const (
	checkboxSelected = "✓"
	checkboxUnsel    = "·"
	separatorChar    = "─"
)
