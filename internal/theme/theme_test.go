package theme_test

import (
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/theme"
	"github.com/stretchr/testify/assert"
)

func TestStateColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state backend.RefState
		want  string
	}{
		{backend.RefStateSynced, "green"},
		{backend.RefStateAhead, "blue"},
		{backend.RefStateBehind, "yellow"},
		{backend.RefStateDiverged, "red"},
		{backend.RefStateGone, "magenta"},
		{backend.RefStateNoRemote, "magenta"},
		{backend.RefStateUnknown, "magenta"},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, theme.StateColor(tt.state), "StateColor(%v)", tt.state)
		})
	}
}

func TestStateColor_Default(t *testing.T) {
	assert.Equal(t, "gray", theme.StateColor(backend.RefState(99)))
}

func TestBookmarkSymbols_Synced(t *testing.T) {
	syms := theme.BookmarkSymbols(backend.BookmarkStatus{State: backend.RefStateSynced})
	assert.Equal(t, []theme.ColoredSymbol{{Color: "green", Symbol: "✓"}}, syms)
}

func TestBookmarkSymbols_Ahead(t *testing.T) {
	syms := theme.BookmarkSymbols(backend.BookmarkStatus{State: backend.RefStateAhead, Ahead: 3})
	assert.Equal(t, []theme.ColoredSymbol{{Color: "blue", Symbol: "↑3"}}, syms)
}

func TestBookmarkSymbols_Behind(t *testing.T) {
	syms := theme.BookmarkSymbols(backend.BookmarkStatus{State: backend.RefStateBehind, Behind: 5})
	assert.Equal(t, []theme.ColoredSymbol{{Color: "yellow", Symbol: "↓5"}}, syms)
}

func TestBookmarkSymbols_Diverged(t *testing.T) {
	syms := theme.BookmarkSymbols(
		backend.BookmarkStatus{State: backend.RefStateDiverged, Ahead: 2, Behind: 1},
	)
	assert.Equal(t, []theme.ColoredSymbol{{Color: "red", Symbol: "↑2↓1"}}, syms)
}

func TestBookmarkSymbols_Gone(t *testing.T) {
	syms := theme.BookmarkSymbols(backend.BookmarkStatus{State: backend.RefStateGone})
	assert.Equal(t, []theme.ColoredSymbol{{Color: "magenta", Symbol: "✗"}}, syms)
}

func TestBookmarkSymbols_NoRemote(t *testing.T) {
	syms := theme.BookmarkSymbols(backend.BookmarkStatus{State: backend.RefStateNoRemote})
	assert.Equal(t, []theme.ColoredSymbol{{Color: "magenta", Symbol: "∅"}}, syms)
}

func TestBookmarkSymbols_Unknown(t *testing.T) {
	syms := theme.BookmarkSymbols(backend.BookmarkStatus{State: backend.RefStateUnknown})
	assert.Equal(t, []theme.ColoredSymbol{{Color: "magenta", Symbol: "?"}}, syms)
}

func TestBookmarkSymbols_Conflict(t *testing.T) {
	syms := theme.BookmarkSymbols(backend.BookmarkStatus{
		State:    backend.RefStateSynced,
		Conflict: true,
	})
	assert.Equal(t, []theme.ColoredSymbol{
		{Color: "green", Symbol: "✓"},
		{Color: "red", Symbol: "!"},
	}, syms)
}

func TestBookmarkSymbols_UnknownState(t *testing.T) {
	syms := theme.BookmarkSymbols(backend.BookmarkStatus{State: backend.RefState(99)})
	assert.Empty(t, syms)
}

func TestDirtyConflict(t *testing.T) {
	assert.Equal(t, "yellow", theme.Dirty.Color)
	assert.Equal(t, "*", theme.Dirty.Symbol)
	assert.Equal(t, "red", theme.Conflict.Color)
	assert.Equal(t, "‼", theme.Conflict.Symbol)
}

func TestColorCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"green", "green", "2"},
		{"blue", "blue", "4"},
		{"yellow", "yellow", "3"},
		{"red", "red", "1"},
		{"magenta", "magenta", "5"},
		{"gray", "gray", "8"},
		{"unknown", "unknown", "15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, theme.ColorCode(tt.input))
		})
	}
}

func TestFormatSymbols(t *testing.T) {
	colorFn := func(color, symbol string) string {
		return "[" + color + ":" + symbol + "]"
	}

	t.Run("synced bookmark", func(t *testing.T) {
		status := backend.RepoStatus{
			Bookmarks: []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
		}
		result := theme.FormatSymbols(status, colorFn)
		assert.Equal(t, "[green:✓]", result)
	})

	t.Run("dirty", func(t *testing.T) {
		status := backend.RepoStatus{Dirty: true}
		result := theme.FormatSymbols(status, colorFn)
		assert.Equal(t, "[yellow:*]", result)
	})

	t.Run("conflict", func(t *testing.T) {
		status := backend.RepoStatus{Conflict: true}
		result := theme.FormatSymbols(status, colorFn)
		assert.Equal(t, "[red:‼]", result)
	})

	t.Run("empty status", func(t *testing.T) {
		status := backend.RepoStatus{}
		result := theme.FormatSymbols(status, colorFn)
		assert.Empty(t, result)
	})

	t.Run("combined bookmarks and dirty", func(t *testing.T) {
		status := backend.RepoStatus{
			Bookmarks: []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
			Dirty:     true,
		}
		result := theme.FormatSymbols(status, colorFn)
		assert.Contains(t, result, "[green:✓]")
		assert.Contains(t, result, "[yellow:*]")
	})

	t.Run("local ahead", func(t *testing.T) {
		status := backend.RepoStatus{
			Bookmarks:  []backend.BookmarkStatus{{Name: "main", State: backend.RefStateSynced}},
			LocalAhead: 5,
		}
		result := theme.FormatSymbols(status, colorFn)
		assert.Contains(t, result, "[green:✓]")
		assert.Contains(t, result, "[blue:⇡5]")
	})

	t.Run("local ahead without bookmark", func(t *testing.T) {
		status := backend.RepoStatus{LocalAhead: 3}
		result := theme.FormatSymbols(status, colorFn)
		assert.Equal(t, "[blue:⇡3]", result)
	})
}
