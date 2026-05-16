package tui

import "charm.land/lipgloss/v2"

//nolint:gochecknoglobals // effectively constant, lipgloss styles
var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("14"))

	styleFooter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	styleSeparator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	styleGroupLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))

	styleSelectMarker = lipgloss.NewStyle().
				Foreground(lipgloss.Color("3"))

	styleSelectedCount = lipgloss.NewStyle().
				Foreground(lipgloss.Color("3"))

	styleWarn = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))

	styleBold = lipgloss.NewStyle().
			Bold(true)
)

const (
	checkboxSelected = "✓"
	checkboxUnsel    = "·"
	separatorChar    = "─"
)
