package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitCommands(t *testing.T) {
	sample := `See 'git help <command>' to read about a specific subcommand

Main Porcelain Commands
   add                     Add file contents to the index
   branch                  List, create, or delete branches
   checkout                Switch branches or restore working tree files
   commit                  Record changes to the repository
   diff                    Show changes between commits, commit and working tree, etc
   fetch                   Download objects and refs from another repository
   log                     Show commit logs
   merge                   Join two or more development histories together
   pull                    Fetch from and integrate with another repository or a local branch
   push                    Update remote refs along with associated objects
   status                  Show the working tree status

Ancillary Commands / Manipulators
   config                  Get and set repository or global options
   help                    Display help information about Git
   mergetool               Run merge conflict resolution tools to resolve merge conflicts
`

	got := parseGitCommands(sample)
	want := []string{
		"git add", "git branch", "git checkout", "git commit",
		"git config", "git diff", "git fetch", "git help",
		"git log", "git merge", "git mergetool", "git pull",
		"git push", "git status",
	}

	require.Len(t, got, len(want))
	assert.Equal(t, want, got)
}

func TestParseGitCommandsEmpty(t *testing.T) {
	assert.Empty(t, parseGitCommands(""))
}

func TestParseGitCommandsNoSections(t *testing.T) {
	assert.Empty(t, parseGitCommands("Just some text\nNo indented commands\n"))
}

func TestParseGitCommandsDeduplicates(t *testing.T) {
	got := parseGitCommands("   status\n   log")
	require.Len(t, got, 2)
	assert.Equal(t, []string{"git log", "git status"}, got)
}

func TestParseGitCommandsTabIndent(t *testing.T) {
	got := parseGitCommands("\tstatus		Show status\n\tlog		Show log")
	require.Len(t, got, 2)
}

func jjSampleCompletion() string {
	return `
_jj() {
    local i cur prev opts cmd
    COMPREPLY=()
    case "${cmd},${i}" in
        ",$1")
            cmd="jj"
            ;;
        jj,abandon)
            cmd="jj__subcmd__abandon"
            ;;
        jj,bookmark)
            cmd="jj__subcmd__bookmark"
            ;;
        jj,commit)
            cmd="jj__subcmd__commit"
            ;;
        jj,describe)
            cmd="jj__subcmd__describe"
            ;;
        jj,diff)
            cmd="jj__subcmd__diff"
            ;;
        jj,git)
            cmd="jj__subcmd__git"
            ;;
        jj,log)
            cmd="jj__subcmd__log"
            ;;
        jj,new)
            cmd="jj__subcmd__new"
            ;;
        jj,rebase)
            cmd="jj__subcmd__rebase"
            ;;
        jj,status)
            cmd="jj__subcmd__status"
            ;;
        jj,b)
            cmd="jj__subcmd__bookmark__b"
            ;;
    esac
}
`
}

func TestParseJjCommands(t *testing.T) {
	got := parseJjCommands(jjSampleCompletion())
	want := []string{
		"jj abandon", "jj b", "jj bookmark", "jj commit",
		"jj describe", "jj diff", "jj git", "jj log",
		"jj new", "jj rebase", "jj status",
	}

	require.Len(t, got, len(want))
	assert.Equal(t, want, got)
}

func TestParseJjCommandsAliases(t *testing.T) {
	sample := `
        jj,status)
                cmd="jj__subcmd__status"
        jj,st)
                cmd="jj__subcmd__status__st"
        jj,diff)
                cmd="jj__subcmd__diff"
        jj,d)
                cmd="jj__subcmd__diff__d"
`

	got := parseJjCommands(sample)
	assert.Len(t, got, 4)
}

func TestParseJjCommandsEmpty(t *testing.T) {
	assert.Empty(t, parseJjCommands(""))
}

func TestParseJjCommandsNoEntries(t *testing.T) {
	assert.Empty(t, parseJjCommands("some random bash code\nno jj entries here\n"))
}

func TestParseJjCommandsDeduplicates(t *testing.T) {
	input := `        jj,status)
                cmd="x"
        jj,status)
                cmd="y"
        jj,log)
                cmd="z"
`

	got := parseJjCommands(input)
	assert.Len(t, got, 2)
}

func TestParseJjCommandsPrefixFormat(t *testing.T) {
	cmds := parseJjCommands("\n        jj,status)\n")
	require.Len(t, cmds, 1)
	assert.Equal(t, "jj status", cmds[0])
}

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
		gitCompletions: []string{"git status", "git log", "git pull", "git push"},
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
		jjCompletions: []string{"jj status", "jj log", "jj new", "jj describe"},
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
		gitCompletions: []string{"git status"},
		jjCompletions:  []string{"jj status"},
	}
	m.initInput()

	m.input.SetValue("!echo hello")
	m.updateCompletions()

	assert.False(t, m.input.ShowSuggestions)
}

func newModelWithCompletions(gitCmds, jjCmds []string) *model {
	m := &model{}
	m.initInput()

	if gitCmds != nil {
		m.gitCompletions = gitCmds
	}

	if jjCmds != nil {
		m.jjCompletions = jjCmds
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
	m := &model{gitCompletions: []string{}}
	m.initInput()

	m.input.SetValue("git st")
	m.updateCompletions()

	assert.False(t, m.input.ShowSuggestions)
}

func TestUpdateCompletions_LazyLoadsGit(t *testing.T) {
	m := &model{}
	m.initInput()
	m.input.SetValue("git st")

	require.Nil(t, m.gitCompletions)

	m.updateCompletions()

	require.NotNil(t, m.gitCompletions)
	require.NotEmpty(t, m.gitCompletions)
	assert.True(t, strings.HasPrefix(m.gitCompletions[0], "git "))
}

func TestUpdateCompletions_LazyLoadsJj(t *testing.T) {
	m := &model{}
	m.initInput()
	m.input.SetValue("jj status")

	require.Nil(t, m.jjCompletions)

	m.updateCompletions()

	require.NotNil(t, m.jjCompletions)
	require.NotEmpty(t, m.jjCompletions)
	assert.True(t, strings.HasPrefix(m.jjCompletions[0], "jj "))
}

func TestUpdateCompletions_OnlyLoadsOnce(t *testing.T) {
	m := &model{}
	m.initInput()

	m.input.SetValue("git st")
	m.updateCompletions()
	gitLen := len(m.gitCompletions)

	m.input.SetValue("git status")
	m.updateCompletions()

	assert.Len(t, m.gitCompletions, gitLen)

	m.input.SetValue("jj status")
	m.updateCompletions()
	jjLen := len(m.jjCompletions)

	m.input.SetValue("jj log")
	m.updateCompletions()

	assert.Len(t, m.jjCompletions, jjLen)
}

func TestHandleInputKey_UpDownWhenSuggestionsActive(t *testing.T) {
	m := &model{
		commandOpen:    true,
		gitCompletions: []string{"git status", "git log", "git diff"},
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
