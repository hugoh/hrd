// Package theme provides colored symbols and helpers for rendering VCS state.
package theme

import (
	"fmt"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
)

// SymbolDoc documents a status symbol and its meaning.
type SymbolDoc struct {
	Symbol      string
	Description string
}

// StatusSymbolDocs returns documentation entries for all status symbols.
//
//nolint:gochecknoglobals // const-like documentation table
var StatusSymbolDocs = []SymbolDoc{
	{Symbol: "✓", Description: "Synced with remote"},
	{Symbol: "↑N", Description: "N commits ahead of remote"},
	{Symbol: "↓N", Description: "N commits behind remote"},
	{Symbol: "⇡N", Description: "Working copy ahead of bookmark (local)"},
	{Symbol: "↑N↓N", Description: "Diverged (ahead and behind)"},
	{Symbol: "∅", Description: "Local only, no remote"},
	{Symbol: "‼", Description: "Unresolved conflict"},
	{Symbol: "!", Description: "Bookmark conflict (jj)"},
	{Symbol: "✗", Description: "Remote was deleted"},
	{Symbol: "*", Description: "Dirty working copy"},
	{Symbol: "?", Description: "Unknown remote state"},
}

const (
	colorGreen   = "green"
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
	"cyan":    "14",
	"purple":  "62",
}

// DefaultColor is the ANSI code used when a color name is not found.
const DefaultColor = "15" // white

// BarTier classifies a dispatch result for the purpose of tinting its
// header bar.
type BarTier int

const (
	BarSuccess BarTier = iota
	BarWarning
	BarError
)

// LightDarkPair holds an ANSI 256-color code for light and dark terminal
// backgrounds; lipgloss/v2 dropped AdaptiveColor in favor of resolving
// light/dark manually, so this fills that role for this package.
type LightDarkPair struct {
	Light string
	Dark  string
}

// Resolve returns the Dark code when dark is true, else the Light code.
func (p LightDarkPair) Resolve(dark bool) string {
	if dark {
		return p.Dark
	}

	return p.Light
}

type barColorPair struct {
	bg LightDarkPair
	fg LightDarkPair
}

//nolint:gochecknoglobals // const-like lookup table shared by CLI and TUI
var barColors = map[BarTier]barColorPair{
	BarSuccess: {
		bg: LightDarkPair{Light: "150", Dark: "22"},
		fg: LightDarkPair{Light: "0", Dark: "15"},
	},
	BarWarning: {
		bg: LightDarkPair{Light: "223", Dark: "94"},
		fg: LightDarkPair{Light: "0", Dark: "15"},
	},
	BarError: {
		bg: LightDarkPair{Light: "217", Dark: "88"},
		fg: LightDarkPair{Light: "0", Dark: "15"},
	},
}

// BarBackground returns the background ANSI code for a result tier, resolved
// for a dark (dark=true) or light (dark=false) terminal background.
func BarBackground(tier BarTier, dark bool) string {
	return barColors[tier].bg.Resolve(dark)
}

// BarForeground returns the foreground ANSI code for a result tier, resolved
// for a dark (dark=true) or light (dark=false) terminal background.
func BarForeground(tier BarTier, dark bool) string {
	return barColors[tier].fg.Resolve(dark)
}

// BarTierColor returns the semantic color name (green/yellow/red) for a
// result tier, used to tint dispatch result delimiter lines.
func BarTierColor(tier BarTier) string {
	switch tier {
	case BarWarning:
		return colorYellow
	case BarError:
		return colorRed
	case BarSuccess:
		return colorGreen
	default:
		return colorGreen
	}
}

// BarTierFor classifies a dispatch result into a bar tier based on whether
// it errored or exited non-zero.
func BarTierFor(err error, exitCode int) BarTier {
	switch {
	case err != nil:
		return BarError
	case exitCode != 0:
		return BarWarning
	default:
		return BarSuccess
	}
}

// SelectionBackground is the highlight color for the currently selected
// table row / list item, resolved for a dark or light terminal background.
//
//nolint:gochecknoglobals // const-like value shared by CLI and TUI
var SelectionBackground = LightDarkPair{Light: "111", Dark: "62"}

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
		return colorGreen
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
		syms = append(syms, colorGreen, "✓")
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

	if status.LocalAhead > 0 {
		syms = append(syms, colorFn("blue", fmt.Sprintf("⇡%d", status.LocalAhead)))
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
