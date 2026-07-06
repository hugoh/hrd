package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/stretchr/testify/assert"
)

func TestFuzzyFilter(t *testing.T) {
	names := []string{"alpha", "beta", "alphabet", "gamma"}

	for _, tt := range []struct {
		name    string
		pattern string
		want    []string
	}{
		{name: "substring", pattern: "alp", want: []string{"alpha", "alphabet"}},
		{name: "fuzzy gaps", pattern: "aph", want: []string{"alpha", "alphabet"}},
		{name: "no match", pattern: "zzz", want: []string{}},
		{name: "preserves input order", pattern: "a", want: []string{"alpha", "beta", "alphabet", "gamma"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fuzzyFilter(names, tt.pattern))
		})
	}
}

func TestNameFilterNarrowsActiveRepoOrder(t *testing.T) {
	m := readyModel(
		[]string{"alpha", "beta", "gamma"},
		map[string]bool{"alpha": true, "beta": true, "gamma": true},
	)

	assert.Equal(t, []string{"alpha", "beta", "gamma"}, m.activeRepoOrder())

	m.nameFilter = "ga"
	assert.Equal(t, []string{"gamma"}, m.activeRepoOrder())
}

func TestFilterKeyFlow(t *testing.T) {
	m := readyModel(
		[]string{"alpha", "beta"},
		map[string]bool{"alpha": true, "beta": true},
	)
	m.updateTableRows()

	// "/" opens the filter input.
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: '/'})
	assert.True(t, m.filterOpen)

	// Typing filters live.
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: 'b', Text: "b"})
	assert.Equal(t, "b", m.nameFilter)
	assert.Equal(t, []string{"beta"}, m.tableRepos())

	// Enter confirms: input closes, filter stays.
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, m.filterOpen)
	assert.Equal(t, "b", m.nameFilter)

	// Esc on the main screen clears the confirmed filter.
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Empty(t, m.nameFilter)
	assert.Equal(t, []string{"alpha", "beta"}, m.tableRepos())
}

func TestAttentionFilterKeyFlow(t *testing.T) {
	m := readyModel(
		[]string{"alpha", "beta"},
		map[string]bool{"alpha": true, "beta": true},
	)
	m.statuses = map[string]runner.StatusResult{
		"alpha": {Status: backend.RepoStatus{Dirty: true}},
		"beta":  {Status: backend.RepoStatus{}},
	}
	m.updateTableRows()
	assert.Equal(t, []string{"alpha", "beta"}, m.tableRepos())

	// "*" toggles the filter on and must refresh the visible table rows.
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: '*', Text: "*"})
	assert.True(t, m.attentionFilter)
	assert.Equal(t, []string{"alpha"}, m.tableRepos())

	// Pressing it again toggles back off.
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: '*', Text: "*"})
	assert.False(t, m.attentionFilter)
	assert.Equal(t, []string{"alpha", "beta"}, m.tableRepos())
}

func TestFilterEscWhileOpenClears(t *testing.T) {
	m := readyModel([]string{"alpha", "beta"}, map[string]bool{"alpha": true, "beta": true})
	m.updateTableRows()

	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: '/'})
	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: 'a', Text: "a"})
	assert.Equal(t, "a", m.nameFilter)

	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, m.filterOpen)
	assert.Empty(t, m.nameFilter)
}

func TestFilterHeaderShowsPattern(t *testing.T) {
	m := readyModel([]string{"alpha"}, map[string]bool{"alpha": true})
	m.width = 80
	m.nameFilter = "alp"

	assert.Contains(t, m.renderHeader(), "/alp")
}
