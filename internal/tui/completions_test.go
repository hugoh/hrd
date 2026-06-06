package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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

	if len(got) != len(want) {
		t.Fatalf("parseGitCommands() returned %d commands, want %d: %v", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseGitCommands()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseGitCommandsEmpty(t *testing.T) {
	got := parseGitCommands("")
	if len(got) != 0 {
		t.Errorf("parseGitCommands('') = %v, want empty", got)
	}
}

func TestParseGitCommandsNoSections(t *testing.T) {
	got := parseGitCommands("Just some text\nNo indented commands\n")
	if len(got) != 0 {
		t.Errorf("parseGitCommands() = %v, want empty for non-indented lines", got)
	}
}

func TestParseGitCommandsDeduplicates(t *testing.T) {
	input := "   status\n   log"

	got := parseGitCommands(input)
	if len(got) != 2 {
		t.Fatalf("parseGitCommands() = %v, want 2 unique commands", got)
	}

	if got[0] != "git log" || got[1] != "git status" {
		t.Errorf("parseGitCommands() = %v, want [git log git status]", got)
	}
}

func TestParseGitCommandsTabIndent(t *testing.T) {
	input := "\tstatus		Show status\n\tlog		Show log"

	got := parseGitCommands(input)
	if len(got) != 2 {
		t.Fatalf("parseGitCommands() = %v, want 2 commands", got)
	}
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

	if len(got) != len(want) {
		t.Fatalf("parseJjCommands() returned %d commands, want %d: %v", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseJjCommands()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
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
	if len(got) != 4 {
		t.Fatalf("expected 4 commands from aliased output, got %d: %v", len(got), got)
	}
}

func TestParseJjCommandsEmpty(t *testing.T) {
	got := parseJjCommands("")
	if len(got) != 0 {
		t.Errorf("parseJjCommands('') = %v, want empty", got)
	}
}

func TestParseJjCommandsNoEntries(t *testing.T) {
	got := parseJjCommands("some random bash code\nno jj entries here\n")
	if len(got) != 0 {
		t.Errorf("parseJjCommands() = %v, want empty", got)
	}
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
	if len(got) != 2 {
		t.Fatalf("parseJjCommands() = %v, want 2 unique commands", got)
	}
}

func TestParseJjCommandsPrefixFormat(t *testing.T) {
	cmds := parseJjCommands("\n        jj,status)\n")
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %v", cmds)
	}

	if cmds[0] != "jj status" {
		t.Errorf("expected 'jj status', got %q", cmds[0])
	}
}

func TestSuggestionsActive(t *testing.T) {
	m := &model{}
	m.initInput()

	t.Run("no suggestions", func(t *testing.T) {
		m.input.ShowSuggestions = false
		if m.suggestionsActive() {
			t.Error("suggestionsActive() = true, want false")
		}
	})

	t.Run("shown but empty matches", func(t *testing.T) {
		m.input.ShowSuggestions = true
		m.input.SetSuggestions(nil)

		if m.suggestionsActive() {
			t.Error("suggestionsActive() = true, want false when no matches")
		}
	})

	t.Run("shown with matches", func(t *testing.T) {
		m.input.SetValue("git ")
		m.input.ShowSuggestions = true
		m.input.SetSuggestions([]string{"git status", "git log"})

		if !m.suggestionsActive() {
			t.Error("suggestionsActive() = false, want true")
		}
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

		if !m.input.ShowSuggestions {
			t.Error("ShowSuggestions should be true for git prefix")
		}

		matches := m.input.MatchedSuggestions()
		found := slices.Contains(matches, "git status")

		if !found {
			t.Errorf("MatchedSuggestions should contain 'git status', got %v", matches)
		}
	})

	t.Run("shows VCS for any input in unified bar", func(t *testing.T) {
		m.input.SetValue("hello")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		if !m.input.ShowSuggestions {
			t.Error("ShowSuggestions should be true for unified bar (VCS completions)")
		}

		matches := m.input.MatchedSuggestions()
		if len(matches) != 0 {
			t.Errorf("MatchedSuggestions should be empty for 'hello', got %v", matches)
		}
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

		if !m.input.ShowSuggestions {
			t.Error("ShowSuggestions should be true for jj prefix")
		}

		matches := m.input.MatchedSuggestions()
		found := slices.Contains(matches, "jj describe")

		if !found {
			t.Errorf("MatchedSuggestions should contain 'jj describe', got %v", matches)
		}
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

	if m.input.ShowSuggestions {
		t.Error("ShowSuggestions should be false for shell prefix")
	}
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

//nolint:cyclop,funlen // test with multiple subtests for each prefix phase
func TestUpdateCompletions_PartialPrefixGit(t *testing.T) {
	t.Run("single char g suggests git", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status", "git log", "git pull", "git push"}, nil)
		m.input.SetValue("g")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		if !m.input.ShowSuggestions {
			t.Fatal("ShowSuggestions should be true for 'g'")
		}

		matches := m.input.MatchedSuggestions()
		if len(matches) != 1 {
			t.Fatalf("expected 1 match ('git') for 'g', got %v", matches)
		}

		if matches[0] != "git" {
			t.Errorf("matched suggestion = %q, want 'git'", matches[0])
		}
	})

	t.Run("two chars gi still suggests git", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status", "git log", "git pull", "git push"}, nil)
		m.input.SetValue("gi")
		m.updateCompletions()

		if !m.input.ShowSuggestions {
			t.Fatal("ShowSuggestions should be true for 'gi'")
		}

		matches := m.input.MatchedSuggestions()
		if len(matches) != 1 || matches[0] != "git" {
			t.Errorf("expected ['git'] for 'gi', got %v", matches)
		}
	})

	t.Run("full prefix git loads subcommands", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status", "git log", "git pull", "git push"}, nil)
		m.input.SetValue("git ")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		if !m.input.ShowSuggestions {
			t.Fatal("ShowSuggestions should be true for 'git '")
		}

		matches := m.input.MatchedSuggestions()
		if len(matches) == 0 {
			t.Fatal("expected at least one subcommand match for 'git ', got none")
		}

		for _, want := range []string{"git status", "git log", "git pull", "git push"} {
			if !slices.Contains(matches, want) {
				t.Errorf("MatchedSuggestions should contain %q, got %v", want, matches)
			}
		}
	})

	t.Run("empty input does not trigger git", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status"}, nil)
		m.input.SetValue("")
		m.updateCompletions()

		if m.input.ShowSuggestions {
			t.Error("ShowSuggestions should be false for empty input")
		}
	})

	t.Run("non-matching does not trigger git", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status"}, nil)
		m.input.SetValue("x")
		m.updateCompletions()

		if !m.input.ShowSuggestions {
			t.Error("ShowSuggestions should be true for VCS fallback with 'x'")
		}
	})

	t.Run("shell bang does not trigger git", func(t *testing.T) {
		m := newModelWithCompletions([]string{"git status"}, nil)
		m.input.SetValue("!g")
		m.updateCompletions()

		if m.input.ShowSuggestions {
			t.Error("ShowSuggestions should be false for '!g'")
		}
	})
}

func TestUpdateCompletions_PartialPrefixJj(t *testing.T) {
	t.Run("single char j suggests jj", func(t *testing.T) {
		m := newModelWithCompletions(nil, []string{"jj status", "jj log", "jj new", "jj describe"})
		m.input.SetValue("j")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		if !m.input.ShowSuggestions {
			t.Fatal("ShowSuggestions should be true for 'j'")
		}

		matches := m.input.MatchedSuggestions()
		if len(matches) != 1 {
			t.Fatalf("expected 1 match ('jj') for 'j', got %v", matches)
		}

		if matches[0] != "jj" {
			t.Errorf("matched suggestion = %q, want 'jj'", matches[0])
		}
	})

	t.Run("full prefix jj loads subcommands", func(t *testing.T) {
		m := newModelWithCompletions(nil, []string{"jj status", "jj log", "jj new", "jj describe"})
		m.input.SetValue("jj ")
		m.input.ShowSuggestions = false
		m.updateCompletions()

		if !m.input.ShowSuggestions {
			t.Fatal("ShowSuggestions should be true for 'jj '")
		}

		matches := m.input.MatchedSuggestions()
		if len(matches) == 0 {
			t.Fatal("expected at least one subcommand match for 'jj ', got none")
		}

		for _, want := range []string{"jj status", "jj log", "jj new", "jj describe"} {
			if !slices.Contains(matches, want) {
				t.Errorf("MatchedSuggestions should contain %q, got %v", want, matches)
			}
		}
	})
}

func TestUpdateCompletions_EmptyCompletions(t *testing.T) {
	m := &model{
		gitCompletions: []string{},
	}
	m.initInput()

	m.input.SetValue("git st")
	m.updateCompletions()

	if m.input.ShowSuggestions {
		t.Error("ShowSuggestions should be false when gitCompletions is empty")
	}
}

func TestUpdateCompletions_LazyLoadsGit(t *testing.T) {
	m := &model{}
	m.initInput()
	m.input.SetValue("git st")

	if m.gitCompletions != nil {
		t.Fatal("gitCompletions should be nil before first access")
	}

	m.updateCompletions()

	if m.gitCompletions == nil {
		t.Fatal("gitCompletions should be populated after lazy load")
	}

	if len(m.gitCompletions) == 0 {
		t.Fatal("gitCompletions should not be empty after lazy load")
	}

	if !strings.HasPrefix(m.gitCompletions[0], "git ") {
		t.Errorf("first completion %q should start with 'git '", m.gitCompletions[0])
	}
}

func TestUpdateCompletions_LazyLoadsJj(t *testing.T) {
	m := &model{}
	m.initInput()
	m.input.SetValue("jj status")

	if m.jjCompletions != nil {
		t.Fatal("jjCompletions should be nil before first access")
	}

	m.updateCompletions()

	if m.jjCompletions == nil {
		t.Fatal("jjCompletions should be populated after lazy load")
	}

	if len(m.jjCompletions) == 0 {
		t.Fatal("jjCompletions should not be empty after lazy load")
	}

	if !strings.HasPrefix(m.jjCompletions[0], "jj ") {
		t.Errorf("first completion %q should start with 'jj '", m.jjCompletions[0])
	}
}

func TestUpdateCompletions_OnlyLoadsOnce(t *testing.T) {
	m := &model{}
	m.initInput()

	m.input.SetValue("git st")
	m.updateCompletions()
	gitLen := len(m.gitCompletions)

	m.input.SetValue("git status")
	m.updateCompletions()

	if len(m.gitCompletions) != gitLen {
		t.Error("gitCompletions should be cached, not reloaded")
	}

	m.input.SetValue("jj status")
	m.updateCompletions()
	jjLen := len(m.jjCompletions)

	m.input.SetValue("jj log")
	m.updateCompletions()

	if len(m.jjCompletions) != jjLen {
		t.Error("jjCompletions should be cached, not reloaded")
	}
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

	if !m.suggestionsActive() {
		t.Fatal("suggestions should be active for 'git '")
	}

	initialIdx := m.input.CurrentSuggestionIndex()

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Error("expected nil cmd for down key")
	}

	newIdx := m.input.CurrentSuggestionIndex()
	if newIdx != initialIdx+1 {
		t.Errorf("suggestion index should increase by 1, got %d (was %d)", newIdx, initialIdx)
	}
}

func TestHandleInputKey_UpDownWhenNoSuggestionsRoutesToHistory(t *testing.T) {
	m := &model{
		commandOpen: true,
		historyIdx:  -1,
	}
	m.initInput()
	m.input.SetValue("hello")
	m.updateCompletions()

	if m.suggestionsActive() {
		t.Fatal("suggestions should not be active for 'hello'")
	}

	m.persState.History = append(
		m.persState.History,
		HistoryEntry{Prefix: "sh", Command: "echo hi"},
	)

	_, cmd := m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyUp})
	if cmd != nil {
		t.Error("expected nil cmd for up key")
	}

	if m.historyIdx != 0 {
		t.Errorf("historyIdx should be 0 after up, got %d", m.historyIdx)
	}
}
