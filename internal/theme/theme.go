// Package theme provides colored symbols and helpers for rendering VCS state.
package theme

import (
	"fmt"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
)

const (
	colorYellow  = "yellow"
	colorRed     = "red"
	colorMagenta = "magenta"
)

// ColoredSymbol pairs a color name with a text symbol for display.
type ColoredSymbol struct {
	Color  string
	Symbol string
}

// ANSIColors maps semantic color names to ANSI 256-color codes.
// Both CLI and TUI share these codes via lipgloss.
//
//nolint:gochecknoglobals // const-like lookup table shared by CLI and TUI
var ANSIColors = map[string]string{
	"green":   "2",
	"blue":    "4",
	"yellow":  "3",
	"red":     "1",
	"magenta": "5",
	"gray":    "8",
}

// DefaultColor is the ANSI code used when a color name is not found.
const DefaultColor = "15" // white

// ColorCode returns the ANSI 256-color code for the given semantic name.
// Returns DefaultColor for unknown names.
func ColorCode(name string) string {
	if code, ok := ANSIColors[name]; ok {
		return code
	}

	return DefaultColor
}

//nolint:gochecknoglobals // const-like exported vars
var (
	// Dirty is the marker for a dirty working copy.
	Dirty = ColoredSymbol{Color: colorYellow, Symbol: "*"}
	// Conflict is the marker for unresolved conflicts.
	Conflict = ColoredSymbol{Color: colorRed, Symbol: "‼"}
)

// StateColor returns a color name for the given ref sync state.
func StateColor(state backend.RefState) string {
	switch state {
	case backend.RefStateSynced:
		return "green"
	case backend.RefStateAhead:
		return "blue"
	case backend.RefStateBehind:
		return colorYellow
	case backend.RefStateDiverged:
		return colorRed
	case backend.RefStateGone, backend.RefStateNoRemote, backend.RefStateUnknown:
		return colorMagenta
	default:
		return "gray"
	}
}

// BookmarkSymbols returns colored symbols representing the bookmark's sync
// state (synced, ahead, behind, diverged, gone, or no remote).
func BookmarkSymbols(bm backend.BookmarkStatus) []ColoredSymbol {
	var syms []string

	switch bm.State {
	case backend.RefStateSynced:
		syms = append(syms, "green", "✓")
	case backend.RefStateAhead:
		syms = append(syms, "blue", fmt.Sprintf("↑%d", bm.Ahead))
	case backend.RefStateBehind:
		syms = append(syms, colorYellow, fmt.Sprintf("↓%d", bm.Behind))
	case backend.RefStateDiverged:
		syms = append(syms, colorRed, fmt.Sprintf("↑%d↓%d", bm.Ahead, bm.Behind))
	case backend.RefStateGone:
		syms = append(syms, colorMagenta, "✗")
	case backend.RefStateNoRemote:
		syms = append(syms, colorMagenta, "∅")
	case backend.RefStateUnknown:
		syms = append(syms, colorMagenta, "?")
	}

	if bm.Conflict {
		syms = append(syms, colorRed, "!")
	}

	return coloredSymbols(syms)
}

// FormatSymbols returns a string of colored symbols for the repo status,
// combining bookmark symbols with dirty and conflict markers.
// The colorFn parameter formats each symbol with its color, e.g.:
// lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorCode(color))).Render(symbol).
func FormatSymbols(status backend.RepoStatus, colorFn func(color, symbol string) string) string {
	var syms []string

	for _, bm := range status.Bookmarks {
		for _, cs := range BookmarkSymbols(bm) {
			syms = append(syms, colorFn(cs.Color, cs.Symbol))
		}
	}

	if status.Dirty {
		syms = append(syms, colorFn(Dirty.Color, Dirty.Symbol))
	}

	if status.Conflict {
		syms = append(syms, colorFn(Conflict.Color, Conflict.Symbol))
	}

	return strings.Join(syms, "")
}

func coloredSymbols(pairs []string) []ColoredSymbol {
	var result []ColoredSymbol

	for i := 0; i+1 < len(pairs); i += 2 {
		result = append(result, ColoredSymbol{Color: pairs[i], Symbol: pairs[i+1]})
	}

	return result
}
