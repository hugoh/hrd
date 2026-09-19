package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

type componentSizes struct {
	table, outW, outH, helpW, helpH, historyW, historyH, groupW, groupH, bar int
}

func sizesOf(m *model) componentSizes {
	return componentSizes{
		table:    m.repoTable.Height(),
		outW:     m.output.Width(),
		outH:     m.output.Height(),
		helpW:    m.helpViewport.Width(),
		helpH:    m.helpViewport.Height(),
		historyW: m.historyList.Width(),
		historyH: m.historyList.Height(),
		groupW:   m.groupList.Width(),
		groupH:   m.groupList.Height(),
		bar:      m.progress.Width(),
	}
}

func TestViewDoesNotResizeComponents(t *testing.T) {
	for _, scr := range []screen{screenMain, screenOutput, screenHelp, screenGroup, screenSelHistory} {
		m := newAlphaBetaModel(t)
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
		m.screen = scr
		m.executing = true
		m.execTotal = 3

		// Change what layout depends on without going through Update.
		m.width, m.height = 60, 20
		before := sizesOf(m)

		m.View()

		require.Equal(t, before, sizesOf(m), "screen %d: View must not mutate the model", scr)
	}
}

func TestUpdateKeepsLayoutInSync(t *testing.T) {
	m := newAlphaBetaModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	full := sizesOf(m)
	require.Equal(t, m.contentHeight(), full.outH)
	require.Equal(t, 100, full.outW)

	m.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	require.True(t, m.commandOpen)
	require.Equal(
		t,
		full.table-inputLineH,
		m.repoTable.Height(),
		"table shrinks for the input line",
	)
	require.Equal(t, full.outH-inputLineH, m.output.Height())

	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Equal(t, full, sizesOf(m), "closing the input line restores the layout")

	m.Update(tea.WindowSizeMsg{Width: 70, Height: 24})
	require.Equal(t, 70, m.output.Width())
	require.Equal(t, m.contentHeight(), m.output.Height())
}

func TestExecBarWidthAdaptsToTerminalWidth(t *testing.T) {
	barCells := func(width int) int {
		m := newAlphaBetaModel(t)
		m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		m.screen = screenOutput
		m.executing = true
		m.execTotal = 5
		m.execResults = []execResult{{name: "alpha"}}

		view := m.outputView()

		return strings.Count(view, "░") + strings.Count(view, "█")
	}

	wide, narrow := barCells(172), barCells(40)

	require.Greater(t, wide, progressBarW, "bar widens beyond its initial default")
	require.Less(t, narrow, wide, "bar shrinks on a narrower terminal")
}
