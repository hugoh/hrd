package tui

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func tickOf(t *testing.T, s *spinner.Model) spinner.TickMsg {
	t.Helper()

	tick, ok := s.Tick().(spinner.TickMsg)
	require.True(t, ok)

	return tick
}

// flattenMsgs runs cmd and returns every message it produces, expanding
// (possibly nested) tea.Batch results.
func flattenMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	msg := cmd()

	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}

	var out []tea.Msg

	for _, c := range batch {
		out = append(out, flattenMsgs(c)...)
	}

	return out
}

func TestSpinnerTickReArmsOnlyWhileAnimating(t *testing.T) {
	tests := []struct {
		name      string
		loading   bool
		executing bool
		pending   map[string]bool
		wantCmd   bool
	}{
		{name: "idle stops the chain"},
		{name: "loading continues", loading: true, wantCmd: true},
		{name: "executing continues", executing: true, wantCmd: true},
		{name: "pending rows continue", pending: map[string]bool{"alpha": true}, wantCmd: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newAlphaBetaModel(t)
			m.loading = tt.loading
			m.executing = tt.executing
			m.pending = tt.pending

			_, cmd := m.handleSpinnerTick(tickOf(t, &m.spinner))

			if tt.wantCmd {
				require.NotNil(t, cmd)
			} else {
				require.Nil(t, cmd, "an idle model must not keep re-arming spinner ticks")
			}
		})
	}
}

func TestRowSpinnerRedrawsPendingRowsOnTick(t *testing.T) {
	m := newAlphaBetaModel(t)
	m.loading = true
	m.selected = map[string]bool{"alpha": true, "beta": true}
	m.pending = map[string]bool{"alpha": true}
	m.updateTableRows()
	before := m.repoTable.Rows()[0][0]

	m.handleSpinnerTick(tickOf(t, &m.rowSpinner))

	require.NotEqual(t, before, m.repoTable.Rows()[0][0], "row spinner frame should advance")
}

func TestStartingALoadRestartsSpinnerTicks(t *testing.T) {
	m := newSingleRepoModel(t)

	var ticks int

	for _, msg := range flattenMsgs(loadStatusesForCmd(m, []string{"testrepo"})) {
		if _, isTick := msg.(spinner.TickMsg); isTick {
			ticks++
		}
	}

	require.Equal(t, 2, ticks, "header and row spinners must both be restarted")
}
