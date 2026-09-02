package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuggestionsActive(t *testing.T) {
	m := &model{}
	m.initInput()

	t.Run("no suggestions", func(t *testing.T) {
		m.input.ShowSuggestions = false
		assert.False(t, m.suggestionsActive())
	})

	t.Run("shown but empty matches", func(t *testing.T) {
		m.input.ShowSuggestions = true
		m.input.SetSuggestions(nil)
		assert.False(t, m.suggestionsActive())
	})

	t.Run("shown with matches", func(t *testing.T) {
		m.input.SetValue("git ")
		m.input.ShowSuggestions = true
		m.input.SetSuggestions([]string{"git status", "git log"})
		assert.True(t, m.suggestionsActive())
	})
}

func TestUpdateCompletions_GitPrefix(t *testing.T) {
	m := &model{
		vcsCompletions: map[string][]string{
			"git": {"git status", "git log", "git pull", "git push"},
		},
	}
	m.initInput()

	t.Run("activates on git prefix", func(t *testing.T) {
		m.input.SetValue("git st")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		assert.True(t, m.input.ShowSuggestions)
		assert.Contains(t, m.input.MatchedSuggestions(), "git status")
	})

	t.Run("shows VCS for any input in unified bar", func(t *testing.T) {
		m.input.SetValue("hello")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		assert.True(t, m.input.ShowSuggestions)
		assert.Empty(t, m.input.MatchedSuggestions())
	})
}

func TestUpdateCompletions_JjPrefix(t *testing.T) {
	m := &model{
		vcsCompletions: map[string][]string{
			"jj": {"jj status", "jj log", "jj new", "jj describe"},
		},
	}
	m.initInput()

	t.Run("activates on jj prefix", func(t *testing.T) {
		m.input.SetValue("jj des")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		assert.True(t, m.input.ShowSuggestions)
		assert.Contains(t, m.input.MatchedSuggestions(), "jj describe")
	})
}

func TestUpdateCompletions_ShellPrefixNoop(t *testing.T) {
	m := &model{
		vcsCompletions: map[string][]string{
			"git": {"git status"},
			"jj":  {"jj status"},
		},
	}
	m.initInput()

	m.input.SetValue("!echo hello")
	m.updateCompletions()

	assert.False(t, m.input.ShowSuggestions)
}

func newModelWithCompletions(gitCmds, jjCmds []string) *model {
	m := &model{}
	m.initInput()

	m.vcsCompletions = make(map[string][]string)

	if gitCmds != nil {
		m.vcsCompletions["git"] = gitCmds
	}

	if jjCmds != nil {
		m.vcsCompletions["jj"] = jjCmds
	}

	return m
}

// matchedSuggestionsFor types input into a fresh model (built from gitCmds
// and jjCmds), runs updateCompletions, and returns the resulting matches —
// requiring that suggestions ended up shown.
func matchedSuggestionsFor(t *testing.T, gitCmds, jjCmds []string, input string) []string {
	t.Helper()

	m := newModelWithCompletions(gitCmds, jjCmds)
	m.input.SetValue(input)
	m.input.ShowSuggestions = false
	m.updateCompletions()

	require.True(t, m.input.ShowSuggestions)

	return m.input.MatchedSuggestions()
}

// assertSinglePrefixMatch asserts updateCompletions narrows input to
// exactly one suggestion, want.
func assertSinglePrefixMatch(t *testing.T, gitCmds, jjCmds []string, input, want string) {
	t.Helper()

	matches := matchedSuggestionsFor(t, gitCmds, jjCmds, input)
	require.Len(t, matches, 1)
	assert.Equal(t, want, matches[0])
}

// assertFullPrefixLoadsSubcommands asserts updateCompletions surfaces every
// command in want among input's suggestions.
func assertFullPrefixLoadsSubcommands(
	t *testing.T,
	gitCmds, jjCmds []string,
	input string,
	want []string,
) {
	t.Helper()

	matches := matchedSuggestionsFor(t, gitCmds, jjCmds, input)
	require.NotEmpty(t, matches)

	for _, cmd := range want {
		assert.Contains(t, matches, cmd)
	}
}

func TestUpdateCompletions_PartialPrefixGit(t *testing.T) {
	gitCmds := []string{"git status", "git log", "git pull", "git push"}

	t.Run("single char g suggests git", func(t *testing.T) {
		assertSinglePrefixMatch(t, gitCmds, nil, "g", "git")
	})

	t.Run("two chars gi still suggests git", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status", "git log", "git pull", "git push"}, nil)
		m.input.SetValue("gi")
		m.updateCompletions()

		require.True(t, m.input.ShowSuggestions)

		matches := m.input.MatchedSuggestions()
		assert.Equal(t, []string{"git"}, matches)
	})

	t.Run("full prefix git loads subcommands", func(t *testing.T) {
		assertFullPrefixLoadsSubcommands(t, gitCmds, nil, "git ", gitCmds)
	})

	t.Run("empty input does not trigger git", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status"}, nil)
		m.input.SetValue("")
		m.updateCompletions()

		assert.False(t, m.input.ShowSuggestions)
	})

	t.Run("non-matching does not trigger git", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status"}, nil)
		m.input.SetValue("x")
		m.updateCompletions()

		assert.True(t, m.input.ShowSuggestions)
	})

	t.Run("shell bang does not trigger git", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status"}, nil)
		m.input.SetValue("!g")
		m.updateCompletions()

		assert.False(t, m.input.ShowSuggestions)
	})
}

func TestUpdateCompletions_PartialPrefixJj(t *testing.T) {
	jjCmds := []string{"jj status", "jj log", "jj new", "jj describe"}

	t.Run("single char j suggests jj", func(t *testing.T) {
		assertSinglePrefixMatch(t, nil, jjCmds, "j", "jj")
	})

	t.Run("full prefix jj loads subcommands", func(t *testing.T) {
		assertFullPrefixLoadsSubcommands(t, nil, jjCmds, "jj ", jjCmds)
	})
}

func TestUpdateCompletions_EmptyCompletions(t *testing.T) {
	m := &model{vcsCompletions: map[string][]string{"git": {}}}
	m.initInput()

	m.input.SetValue("git st")
	m.updateCompletions()

	assert.False(t, m.input.ShowSuggestions)
}

func TestHandleInputKey_UpDownWhenSuggestionsActive(t *testing.T) {
	m := &model{
		commandOpen: true,
		vcsCompletions: map[string][]string{
			"git": {"git status", "git log", "git diff"},
		},
	}
	m.initInput()
	m.input.Focus()
	m.input.SetValue("git ")
	m.updateCompletions()

	require.True(t, m.suggestionsActive())

	initialIdx := m.input.CurrentSuggestionIndex()

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Nil(t, cmd)

	newIdx := m.input.CurrentSuggestionIndex()
	assert.Equal(t, initialIdx+1, newIdx)
}

func TestHandleInputKey_UpDownWhenNoSuggestionsRoutesToHistory(t *testing.T) {
	m := &model{
		commandOpen: true,
		historyIdx:  -1,
	}
	m.initInput()
	m.input.SetValue("hello")
	m.updateCompletions()

	require.False(t, m.suggestionsActive())

	m.persState.History = append(
		m.persState.History,
		HistoryEntry{Prefix: "sh", Command: "echo hi"},
	)

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Nil(t, cmd)
	assert.Equal(t, 0, m.historyIdx)
}
