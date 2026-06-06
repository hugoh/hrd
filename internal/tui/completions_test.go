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

//nolint:funlen // multiple subtests for each prefix phase
func TestUpdateCompletions_PartialPrefixGit(t *testing.T) {
	t.Run("single char g suggests git", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status", "git log", "git pull", "git push"}, nil)
		m.input.SetValue("g")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		require.True(t, m.input.ShowSuggestions)

		matches := m.input.MatchedSuggestions()
		require.Len(t, matches, 1)
		assert.Equal(t, "git", matches[0])
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
		m := newModelWithCompletions([]string{"git status", "git log", "git pull", "git push"}, nil)
		m.input.SetValue("git ")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		require.True(t, m.input.ShowSuggestions)

		matches := m.input.MatchedSuggestions()
		require.NotEmpty(t, matches)

		for _, want := range []string{"git status", "git log", "git pull", "git push"} {
			assert.Contains(t, matches, want)
		}
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
	t.Run("single char j suggests jj", func(t *testing.T) {
		m := newModelWithCompletions(nil, []string{"jj status", "jj log", "jj new", "jj describe"})
		m.input.SetValue("j")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		require.True(t, m.input.ShowSuggestions)

		matches := m.input.MatchedSuggestions()
		require.Len(t, matches, 1)
		assert.Equal(t, "jj", matches[0])
	})

	t.Run("full prefix jj loads subcommands", func(t *testing.T) {
		m := newModelWithCompletions(nil, []string{"jj status", "jj log", "jj new", "jj describe"})
		m.input.SetValue("jj ")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		require.True(t, m.input.ShowSuggestions)

		matches := m.input.MatchedSuggestions()
		require.NotEmpty(t, matches)

		for _, want := range []string{"jj status", "jj log", "jj new", "jj describe"} {
			assert.Contains(t, matches, want)
		}
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
