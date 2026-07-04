package tui

import (
	"path/filepath"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleModalKeyAlert(t *testing.T) {
	m := &model{
		modal: modalAlert,
	}
	m.initTable()

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: ' '})

	assert.Nil(t, cmd, "expected nil cmd after closing alert")
	assert.Equal(t, modalNone, m.modal, "alert should be cleared after any key")
}

func TestHandleModalKeyAlertAt(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Groups: map[string]config.Group{"work": {}},
		},
		modal: modalAlert,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: '@'})

	assert.Equal(t, screenGroup, m.screen, "screen should be screenGroup after @ in alert")
	assert.Nil(t, cmd, "expected nil cmd after closing alert")
}

// newGroupFilterPopupModel builds a model with one repo ("r1") in a "work"
// group, with the group-filter popup already open, ready for the caller to
// select an item and press enter.
func newGroupFilterPopupModel(t *testing.T) *model {
	t.Helper()

	m := &model{
		ctx: t.Context(),
		cfg: config.Config{
			Repos:  map[string]config.Repo{"r1": {}},
			Groups: map[string]config.Group{"work": {Repos: []string{"r1"}}},
		},
		repoOrder: []string{"r1"},
		selected:  map[string]bool{},
	}
	m.initTable()
	m.initGroupList()
	openGroupPopup(m, groupFilterMode)

	return m
}

func TestHandleGroupKeyEnterNonAll(t *testing.T) {
	m := newGroupFilterPopupModel(t)
	m.groupList.Select(1)

	_, _ = m.handleGroupKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, "work", m.groupFilter, "groupFilter")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.True(t, m.loading, "loading should be true after group selection")
}

func TestHandleKeyMsgRefresh(t *testing.T) {
	m := &model{
		loading:  true,
		statuses: map[string]runner.StatusResult{},
	}
	m.repoOrder = []string{}
	m.selected = map[string]bool{}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'r'})

	assert.NotNil(t, cmd, "expected non-nil cmd for refresh")
}

func TestHandleKeyMsgHelp(t *testing.T) {
	m := &model{}
	m.initHelpViewport()

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: '?'})

	assert.Equal(t, screenHelp, m.screen, "screen")
	assert.Nil(t, cmd, "expected nil cmd for help")
}

func TestHandleKeyMsgQOutputScreen(t *testing.T) {
	m := &model{
		screen: screenOutput,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	assert.Nil(t, cmd, "expected nil cmd when q pressed in output mode")
}

func TestHandleKeyMsgQWithHelpScreen(t *testing.T) {
	m := &model{
		screen: screenHelp,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	assert.Nil(t, cmd, "expected nil cmd when q pressed on help screen")
	assert.Equal(t, screenMain, m.screen, "should return to main screen after q on help")
}

func TestHandleKeyMsgQWithAlert(t *testing.T) {
	m := &model{
		modal: modalAlert,
	}

	_, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'q'})
	assert.Nil(t, cmd, "expected nil cmd when q pressed with alert")
	assert.Equal(t, modalNone, m.modal, "alert should be dismissed on q")
}

func TestOpenGroupPopup(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Groups: map[string]config.Group{
				"work":     {},
				"personal": {},
			},
		},
		groupFilter: "work",
	}
	m.initGroupList()

	openGroupPopup(m, groupFilterMode)

	assert.Equal(t, screenGroup, m.screen, "screen")
	assert.Len(t, m.groupList.Items(), 3, "expected 3 popup items")
	assert.Equal(t, 2, m.groupList.Index(), "cursor should point to work")
}

func TestOpenGroupPopupNoGroupFilter(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Groups: map[string]config.Group{
				"work": {},
			},
		},
	}
	m.initGroupList()

	openGroupPopup(m, groupFilterMode)

	assert.Equal(t, screenGroup, m.screen, "screen")
	assert.Len(t, m.groupList.Items(), 2, "expected 2 popup items")
	assert.Equal(t, 0, m.groupList.Index(), "cursor should be at [all]")
}

func TestHandleOutputKeyEsc(t *testing.T) {
	m := &model{
		screen: screenOutput,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleOutputKey(tea.KeyPressMsg{Code: tea.KeyEsc})

	assert.Equal(t, screenMain, m.screen, "screen")
	assert.Nil(t, cmd, "expected nil cmd when closing output")
}

func TestHandleOutputKeyQ(t *testing.T) {
	m := &model{
		screen: screenOutput,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleOutputKey(tea.KeyPressMsg{Code: 'q'})

	assert.Equal(t, screenMain, m.screen, "screen")
	assert.Nil(t, cmd, "expected nil cmd when closing output with q")
}

func TestHandleOutputKeyEnterClosesWhenDone(t *testing.T) {
	m := &model{
		screen:    screenOutput,
		executing: false,
		output:    viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleOutputKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, screenMain, m.screen, "Enter should close output when not executing")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestHandleOutputKeyEnterNoOpWhileExecuting(t *testing.T) {
	m := &model{
		screen:    screenOutput,
		executing: true,
		output:    viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, _ = m.handleOutputKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, screenOutput, m.screen, "Enter should not close output while executing")
}

func TestHandleOutputKeyOClosesOutput(t *testing.T) {
	m := &model{
		screen: screenOutput,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, cmd := m.handleOutputKey(tea.KeyPressMsg{Code: 'o'})

	assert.Equal(t, screenMain, m.screen, "o should close the output screen")
	assert.Nil(t, cmd, "expected nil cmd")
}

func TestOutputContentPreservedOnClose(t *testing.T) {
	const content = "repo1 ✓\nrepo2 ✗"

	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"Esc", tea.KeyPressMsg{Code: tea.KeyEsc}},
		{"q", tea.KeyPressMsg{Code: 'q'}},
		{"o", tea.KeyPressMsg{Code: 'o'}},
		{"Enter", tea.KeyPressMsg{Code: tea.KeyEnter}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &model{
				screen: screenOutput,
				output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
			}
			m.output.SetContent(content)

			_, _ = m.handleOutputKey(tc.key)

			assert.Equal(
				t,
				content,
				m.output.GetContent(),
				"output content should be preserved after %s",
				tc.name,
			)
		})
	}
}

func TestHandleEscOutputPreservesContent(t *testing.T) {
	const content = "repo1 ✓"

	m := &model{
		screen: screenOutput,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.output.SetContent(content)

	_, _ = m.handleEscKey()

	assert.Equal(t, screenMain, m.screen, "screen")
	assert.Equal(t, content, m.output.GetContent(), "output content should be preserved after Esc")
}

func TestHandleMainKeyOOpensOutputWithResults(t *testing.T) {
	m := &model{
		execResults: []execResult{{name: "repo1"}},
		output:      viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: 'o'})

	assert.Equal(t, screenOutput, m.screen, "o should open output screen when results exist")
}

func TestHandleMainKeyOOpensOutputWithoutResults(t *testing.T) {
	m := &model{
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}

	_, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: 'o'})

	assert.Equal(t, screenOutput, m.screen, "o should open output screen even with no results")
}

func TestSortedSelected(t *testing.T) {
	selected := map[string]bool{"c": true, "a": true, "b": false}
	got := sortedSelected(selected)

	assert.Equal(t, []string{"a", "c"}, got)
}

func TestPushSelectionHistory(t *testing.T) {
	m := &model{
		selected:  map[string]bool{"a": true, "b": true},
		persState: PersistentState{SelectionHistory: []SelectionEntry{}},
	}

	m.pushSelectionHistory()

	require.Len(t, m.persState.SelectionHistory, 1)

	entry := m.persState.SelectionHistory[0]
	assert.Equal(t, []string{"a", "b"}, entry.Repos)
	assert.False(t, entry.Timestamp.IsZero())

	// Push identical state — should be deduped
	m.pushSelectionHistory()

	assert.Len(t, m.persState.SelectionHistory, 1, "SelectionHistory should be deduped")

	// Push different state
	m.selected = map[string]bool{"a": true}
	m.pushSelectionHistory()

	assert.Len(t, m.persState.SelectionHistory, 2, "SelectionHistory length")
}

func TestOpenSelHistoryPopup(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Repos: map[string]config.Repo{"a": {}, "b": {}},
		},
		persState: PersistentState{
			SelectionHistory: []SelectionEntry{
				{Repos: []string{"a", "b"}},
			},
		},
	}
	m.initHistoryList()
	openSelHistoryPopup(m)

	assert.Equal(t, screenSelHistory, m.screen, "screen")
	assert.Equal(t, 0, m.historyList.Index(), "cursor")
}

func TestHandleSelHistoryKeyNavigation(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Repos: map[string]config.Repo{"a": {}, "b": {}, "c": {}, "d": {}},
		},
		persState: PersistentState{
			SelectionHistory: []SelectionEntry{
				{Repos: []string{"a", "b"}},
				{Repos: []string{"c"}},
				{Repos: []string{"d"}},
			},
		},
	}
	m.initTable()
	m.initHistoryList()

	openSelHistoryPopup(m)

	require.Equal(t, 0, m.historyList.Index(), "initial cursor")

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, m.historyList.Index(), "cursor after down")

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, m.historyList.Index(), "cursor after up")

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: 'j'})
	assert.Equal(t, 1, m.historyList.Index(), "cursor after j")

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: 'k'})
	assert.Equal(t, 0, m.historyList.Index(), "cursor after k")

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, m.historyList.Index(), "cursor at top")
}

func TestHandleSelHistoryKeyEmpty(t *testing.T) {
	m := &model{
		screen:    screenSelHistory,
		persState: PersistentState{SelectionHistory: []SelectionEntry{}},
	}
	m.initTable()
	m.initHistoryList()

	_, _ = m.handleSelHistoryKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, screenSelHistory, m.screen, "empty list, no-op")
}

func TestHandleSelHistoryRestore(t *testing.T) {
	m := &model{
		ctx:       t.Context(),
		repoOrder: []string{"a", "b", "c"},
		cfg: config.Config{
			Repos: map[string]config.Repo{
				"a": {},
				"b": {},
				"c": {},
			},
			Settings: config.Settings{Concurrency: 1},
		},
		persState: PersistentState{SelectionHistory: []SelectionEntry{}},
		mode:      modeSingle,
	}
	m.initTable()

	repos := []string{"a", "c"}
	_, cmd := m.handleSelHistoryRestore(repos)

	assert.Equal(t, modeNormal, m.mode, "mode should be modeNormal after restore")
	assert.True(t, m.selected["a"] && m.selected["c"] && !m.selected["b"],
		"restore should set correct selected repos")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.NotNil(t, cmd, "expected non-nil cmd (refresh) after restore")
	assert.Equal(t, modalNone, m.modal, "modal should not be set when all repos exist")
}

func TestHandleSelHistoryRestoreMissingRepo(t *testing.T) {
	m := &model{
		ctx:       t.Context(),
		repoOrder: []string{"a"},
		cfg: config.Config{
			Repos: map[string]config.Repo{
				"a": {},
			},
			Settings: config.Settings{Concurrency: 1},
		},
		persState: PersistentState{SelectionHistory: []SelectionEntry{}},
	}
	m.initTable()

	repos := []string{"a", "stale_repo"}
	_, _ = m.handleSelHistoryRestore(repos)

	assert.Equal(t, modalAlert, m.modal, "modal should be modalAlert when repos are missing")
	assert.NotEmpty(t, m.alertMsg, "alertMsg should be set when repos are missing")
	assert.True(t, m.selected["a"], "existing repos should still be selected")
}

func TestHandleGroupEnterFilterModeAll(t *testing.T) {
	m := &model{
		ctx: t.Context(),
		cfg: config.Config{
			Repos:  map[string]config.Repo{"r1": {Groups: []string{"work"}}},
			Groups: map[string]config.Group{"work": {Repos: []string{"r1"}}},
		},
		repoOrder:   []string{"r1"},
		selected:    map[string]bool{},
		groupFilter: "work",
	}
	m.initTable()
	m.initGroupList()
	openGroupPopup(m, groupFilterMode)
	m.groupList.Select(0)

	_, cmd := m.handleGroupEnter()

	assert.Empty(t, m.groupFilter, "groupFilter")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.NotNil(t, cmd, "expected non-nil cmd (refresh) after group selection")
}

func TestHandleGroupEnterAddModeExistingGroup(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	initialCfg := config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Groups: []string{"work"}},
			"repo2": {},
		},
		Groups: map[string]config.Group{"work": {Repos: []string{"repo1"}}},
	}
	require.NoError(t, config.Save(cfgPath, initialCfg))

	m := &model{
		ctx:       t.Context(),
		cfg:       initialCfg,
		opts:      Options{ConfigPath: cfgPath},
		repoOrder: []string{"repo1", "repo2"},
		selected:  map[string]bool{"repo2": true},
		groupMode: groupAddMode,
	}
	m.initTable()
	m.initGroupList()
	openGroupPopup(m, groupAddMode)
	m.groupList.Select(0)

	_, cmd := m.handleGroupEnter()

	assert.Nil(t, cmd, "expected nil cmd after adding to existing group")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.Len(t, m.cfg.Groups["work"].Repos, 2, "expected 2 repos in work group")
}

func TestHandleGroupEnterAddModeSaveFailure(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	initialCfg := config.Config{
		Repos:  map[string]config.Repo{"repo1": {}},
		Groups: map[string]config.Group{"work": {}},
	}
	require.NoError(t, config.Save(cfgPath, initialCfg))
	makeDirReadOnly(t, filepath.Dir(cfgPath))

	m := &model{
		ctx:       t.Context(),
		cfg:       initialCfg,
		opts:      Options{ConfigPath: cfgPath},
		repoOrder: []string{"repo1"},
		selected:  map[string]bool{"repo1": true},
		groupMode: groupAddMode,
	}
	m.initTable()
	m.initGroupList()
	openGroupPopup(m, groupAddMode)
	m.groupList.Select(0)

	_, cmd := m.handleGroupEnter()

	assert.Nil(t, cmd, "expected nil cmd on save failure")
	assert.Equal(t, modalAlert, m.modal, "modal should be modalAlert on save failure")
	assert.NotEmpty(t, m.alertMsg, "alertMsg should be set on save failure")
}

func TestHandleGroupEnterAddModeNew(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Repos: map[string]config.Repo{"repo1": {}},
		},
		repoOrder: []string{"repo1"},
		selected:  map[string]bool{"repo1": true},
		groupMode: groupAddMode,
	}
	m.initGroupList()
	openGroupPopup(m, groupAddMode)
	m.groupList.Select(0)
	m.initInput()

	_, cmd := m.handleGroupEnter()

	assert.Nil(t, cmd, "expected nil cmd when entering new group name")
	assert.True(t, m.groupNewInput, "groupNewInput should be true after selecting [new...]")
}

func TestHandleGroupNewInputEnter(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	initialCfg := config.Config{
		Repos: map[string]config.Repo{
			"repo1": {},
			"repo2": {},
		},
		Groups: map[string]config.Group{},
	}
	require.NoError(t, config.Save(cfgPath, initialCfg))

	m := &model{
		cfg:           initialCfg,
		opts:          Options{ConfigPath: cfgPath},
		repoOrder:     []string{"repo1", "repo2"},
		selected:      map[string]bool{"repo2": true},
		groupNewInput: true,
		screen:        screenGroup,
	}
	m.initInput()
	m.input.SetValue("my-group")

	_, cmd := m.handleGroupNewInput(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd, "expected nil cmd after creating new group")
	assert.False(t, m.groupNewInput, "groupNewInput should be false after Enter")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.Contains(t, m.cfg.Groups, "my-group", "my-group should exist in config after creation")
}

func TestHandleGroupNewInputEnterSaveFailure(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	initialCfg := config.Config{
		Repos: map[string]config.Repo{"repo1": {}},
	}
	require.NoError(t, config.Save(cfgPath, initialCfg))
	makeDirReadOnly(t, filepath.Dir(cfgPath))

	m := &model{
		cfg:           initialCfg,
		opts:          Options{ConfigPath: cfgPath},
		repoOrder:     []string{"repo1"},
		selected:      map[string]bool{"repo1": true},
		groupNewInput: true,
		screen:        screenGroup,
	}
	m.initInput()
	m.input.SetValue("my-group")

	_, cmd := m.handleGroupNewInput(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd, "expected nil cmd on save failure")
	assert.True(t, m.groupNewInput, "groupNewInput should remain true on save failure")
	assert.Equal(t, modalAlert, m.modal, "modal should be modalAlert on save failure")
	assert.NotEmpty(t, m.alertMsg, "alertMsg should be set on save failure")
}

func TestHandleGroupNewInputEmpty(t *testing.T) {
	m := &model{
		groupNewInput: true,
	}
	m.initInput()

	_, cmd := m.handleGroupNewInput(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd, "expected nil cmd for empty group name")
	assert.True(t, m.groupNewInput, "groupNewInput should remain true when name is empty")
}

func TestHandleGroupNewInputEsc(t *testing.T) {
	m := &model{
		groupNewInput: true,
	}
	m.initInput()

	_, cmd := m.handleGroupNewInput(tea.KeyPressMsg{Code: tea.KeyEsc})

	assert.Nil(t, cmd, "expected nil cmd after Esc")
	assert.False(t, m.groupNewInput, "groupNewInput should be false after Esc")
}

func TestHandleGroupKeyNavigation(t *testing.T) {
	m := &model{
		groupMode: groupFilterMode,
	}
	m.initGroupList()
	m.groupList.SetItems([]list.Item{
		groupItem{name: "a"},
		groupItem{name: "b"},
		groupItem{name: "c"},
	})
	m.groupList.Select(1)

	_, cmd := m.handleGroupKey(tea.KeyPressMsg{Code: 'k'})
	assert.Nil(t, cmd, "expected nil cmd after up")
	assert.Equal(t, 0, m.groupList.Index(), "cursor after up")

	_, cmd = m.handleGroupKey(tea.KeyPressMsg{Code: 'j'})
	assert.Nil(t, cmd, "expected nil cmd after down")
	assert.Equal(t, 1, m.groupList.Index(), "cursor after down")

	m.groupList.Select(0)
	m.handleGroupKey(tea.KeyPressMsg{Code: 'k'})
	assert.Equal(t, 0, m.groupList.Index(), "cursor at top")

	m.groupList.Select(2)
	m.handleGroupKey(tea.KeyPressMsg{Code: 'j'})
	assert.Equal(t, 2, m.groupList.Index(), "cursor at bottom")
}

func TestOpenGroupPopupAddMode(t *testing.T) {
	m := &model{
		cfg: config.Config{
			Repos: map[string]config.Repo{
				"r1": {Groups: []string{"work", "personal"}},
			},
			Groups: map[string]config.Group{
				"personal": {Repos: []string{"r1"}},
				"work":     {Repos: []string{"r1"}},
			},
		},
	}
	m.initGroupList()

	openGroupPopup(m, groupAddMode)

	assert.Equal(t, screenGroup, m.screen, "screen")
	assert.Len(t, m.groupList.Items(), 3, "expected 3 popup items")
	assert.Equal(t, groupAddMode, m.groupMode, "groupMode")
}

func TestHandleGroupKeyEnterOnAllFilterMode(t *testing.T) {
	m := newGroupFilterPopupModel(t)
	m.groupList.Select(0)

	_, cmd := m.handleGroupKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Empty(t, m.groupFilter, "groupFilter")
	assert.Equal(t, screenMain, m.screen, "screen")
	assert.NotNil(t, cmd, "expected non-nil cmd (refresh) after group selection")
}
