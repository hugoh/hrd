package tui

import (
	"testing"

	"charm.land/bubbles/v2/textinput"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initInput() textinput.Model {
	ti := textinput.New()
	ti.CharLimit = 512

	return ti
}

func TestPushHistory(t *testing.T) {
	m := &model{persState: PersistentState{}}

	m.pushHistory("git", "status")
	require.Len(t, m.persState.History, 1)
	assert.Equal(t, HistoryEntry{Prefix: "git", Command: "status"}, m.persState.History[0])

	m.pushHistory("jj", "log")
	require.Len(t, m.persState.History, 2)
	assert.Equal(t, HistoryEntry{Prefix: "jj", Command: "log"}, m.persState.History[0])
	assert.Equal(t, HistoryEntry{Prefix: "git", Command: "status"}, m.persState.History[1])
}

func TestPushHistoryCap(t *testing.T) {
	m := &model{persState: PersistentState{}}

	for range maxHistoryLen + 5 {
		m.pushHistory("git", "")
	}

	assert.LessOrEqual(t, len(m.persState.History), maxHistoryLen)
}

func TestPushHistorySavesEmptyPrefix(t *testing.T) {
	m := &model{persState: PersistentState{}}

	m.pushHistory("", "status")
	m.pushHistory("", "log")

	require.Len(t, m.persState.History, 2)
	assert.Equal(t, HistoryEntry{Prefix: "", Command: "log"}, m.persState.History[0])
	assert.Equal(t, HistoryEntry{Prefix: "", Command: "status"}, m.persState.History[1])
}

func TestHistoryEntriesFiltersByHistoryFilterPrefix(t *testing.T) {
	m := &model{
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "git", Command: "log"},
				{Prefix: "sh", Command: "ls"},
				{Prefix: "git", Command: "status"},
			},
		},
		cmdPrefix:           prefixNone,
		historyFilterPrefix: "git",
	}

	got := m.historyEntries()
	want := []HistoryEntry{
		{Prefix: "git", Command: "log"},
		{Prefix: "git", Command: "status"},
	}

	require.Len(t, got, len(want))
	assert.Equal(t, want, got)
}

func TestHistoryEntriesAllPrefixes(t *testing.T) {
	history := []HistoryEntry{
		{Prefix: "git", Command: "log"},
		{Prefix: "git", Command: "status"},
		{Prefix: "jj", Command: "evolve"},
		{Prefix: "sh", Command: "make test"},
	}

	t.Run("unified bar no filter shows all", func(t *testing.T) {
		m := &model{
			persState:           PersistentState{History: history},
			cmdPrefix:           prefixNone,
			historyFilterPrefix: "",
		}

		assert.Len(t, m.historyEntries(), 4)
	})

	t.Run("unified bar git filter", func(t *testing.T) {
		m := &model{
			persState:           PersistentState{History: history},
			cmdPrefix:           prefixNone,
			historyFilterPrefix: "git",
		}

		assert.Len(t, m.historyEntries(), 2)
	})

	t.Run("unified bar jj filter", func(t *testing.T) {
		m := &model{
			persState:           PersistentState{History: history},
			cmdPrefix:           prefixNone,
			historyFilterPrefix: "jj",
		}

		assert.Len(t, m.historyEntries(), 1)
	})

	t.Run("unified bar sh filter", func(t *testing.T) {
		m := &model{
			persState:           PersistentState{History: history},
			cmdPrefix:           prefixNone,
			historyFilterPrefix: "sh",
		}

		assert.Len(t, m.historyEntries(), 1)
	})
}

func TestHistoryEntriesEmpty(t *testing.T) {
	m := &model{
		persState:           PersistentState{},
		cmdPrefix:           prefixNone,
		historyFilterPrefix: "git",
	}

	assert.Nil(t, m.historyEntries())
}

func TestHistoryEntriesNoMatch(t *testing.T) {
	m := &model{
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "sh", Command: "ls"},
				{Prefix: "sh", Command: "pwd"},
			},
		},
		cmdPrefix:           prefixNone,
		historyFilterPrefix: "git",
	}

	assert.Nil(t, m.historyEntries())
}

// gitHistoryModel builds a model filtered to the "git" history prefix, with
// entries and historyIdx as given — the fixture shared by tests exercising
// historyPrev/historyNext.
func gitHistoryModel(entries []HistoryEntry, historyIdx int) *model {
	return &model{
		input:               initInput(),
		persState:           PersistentState{History: entries},
		cmdPrefix:           prefixNone,
		historyFilterPrefix: "git",
		historyIdx:          historyIdx,
	}
}

func TestHistoryPrev(t *testing.T) {
	tests := []struct {
		name       string
		historyIdx int
		wantIdx    int
		wantValue  string
	}{
		{"first call", -1, 0, "git log"},
		{"clamps at end", 1, 1, "git status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := gitHistoryModel([]HistoryEntry{
				{Prefix: "git", Command: "log"},
				{Prefix: "git", Command: "status"},
			}, tt.historyIdx)

			m.historyPrev()

			assert.Equal(t, tt.wantIdx, m.historyIdx)
			assert.Equal(t, tt.wantValue, m.input.Value())
		})
	}
}

func TestHistoryPrevNoEntries(t *testing.T) {
	m := &model{
		persState:           PersistentState{},
		cmdPrefix:           prefixNone,
		historyFilterPrefix: "git",
		historyIdx:          -1,
	}

	m.historyPrev()

	assert.Equal(t, -1, m.historyIdx)
}

func TestHistoryNextEndsAtEmptyInput(t *testing.T) {
	tests := []struct {
		name       string
		historyIdx int
	}{
		{"from newest", 0},
		{"at min bound", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := gitHistoryModel([]HistoryEntry{
				{Prefix: "git", Command: "log"},
			}, tt.historyIdx)

			m.historyNext()

			assert.Equal(t, -1, m.historyIdx)
			assert.Empty(t, m.input.Value())
		})
	}
}

func TestHistoryNextCyclesBack(t *testing.T) {
	m := gitHistoryModel([]HistoryEntry{
		{Prefix: "git", Command: "log"},
		{Prefix: "git", Command: "status"},
		{Prefix: "git", Command: "fetch"},
	}, 2)

	m.historyNext()

	assert.Equal(t, 1, m.historyIdx)
	assert.Equal(t, "git status", m.input.Value())
}

func TestHistoryReset(t *testing.T) {
	m := &model{historyIdx: 5, historyFilterPrefix: "git"}

	m.historyReset()

	assert.Equal(t, -1, m.historyIdx)
	assert.Empty(t, m.historyFilterPrefix)
}

func TestFormatHistoryEntry_PrefixNoneSh(t *testing.T) {
	got := formatHistoryEntry(prefixNone, HistoryEntry{Prefix: "sh", Command: "ls ."})
	assert.Equal(t, "!ls .", got)
}

func TestFormatHistoryEntry_PrefixNoneGit(t *testing.T) {
	got := formatHistoryEntry(prefixNone, HistoryEntry{Prefix: "git", Command: "log"})
	assert.Equal(t, "git log", got)
}

func TestFormatHistoryEntry_PrefixNoneJj(t *testing.T) {
	got := formatHistoryEntry(prefixNone, HistoryEntry{Prefix: "jj", Command: "status"})
	assert.Equal(t, "jj status", got)
}

func TestFormatHistoryEntry_PrefixNoneEmpty(t *testing.T) {
	got := formatHistoryEntry(prefixNone, HistoryEntry{Prefix: "", Command: "status"})
	assert.Equal(t, "status", got)
}

func TestFormatHistoryEntry_PerVcsBar(t *testing.T) {
	got := formatHistoryEntry(prefixNone, HistoryEntry{Prefix: "git", Command: "log"})
	assert.Equal(t, "git log", got)
}

func TestHistoryFilterFromInput_Shell(t *testing.T) {
	m := &model{input: initInput()}
	m.input.SetValue("!ls .")

	assert.Equal(t, "sh", m.historyFilterFromInput())
}

func TestHistoryFilterFromInput_Jj(t *testing.T) {
	m := &model{input: initInput()}
	m.input.SetValue("jj status")

	assert.Equal(t, "jj", m.historyFilterFromInput())
}

func TestHistoryFilterFromInput_Git(t *testing.T) {
	m := &model{input: initInput()}
	m.input.SetValue("git log")

	assert.Equal(t, "git", m.historyFilterFromInput())
}

func TestHistoryFilterFromInput_Empty(t *testing.T) {
	m := &model{input: initInput()}
	assert.Empty(t, m.historyFilterFromInput())
}

func TestHistoryFilterFromInput_NoPrefix(t *testing.T) {
	m := &model{input: initInput()}
	m.input.SetValue("status")

	assert.Empty(t, m.historyFilterFromInput())
}

func TestHistoryEntries_EmptyFilterReturnsAll(t *testing.T) {
	m := &model{
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "git", Command: "log"},
			},
		},
		cmdPrefix:           prefixNone,
		historyFilterPrefix: "",
	}

	assert.Len(t, m.historyEntries(), 1)
}

func TestHistoryPrev_UnifiedBarReconstructs(t *testing.T) {
	m := &model{
		input: initInput(),
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "jj", Command: "status"},
				{Prefix: "git", Command: "log"},
			},
		},
		cmdPrefix:  prefixNone,
		historyIdx: -1,
	}

	m.historyPrev()

	assert.Equal(t, "jj status", m.input.Value())
}

func TestHistoryPrev_UnifiedBarReconstructsShell(t *testing.T) {
	m := &model{
		input: initInput(),
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "sh", Command: "ls ."},
			},
		},
		cmdPrefix:  prefixNone,
		historyIdx: -1,
	}

	m.historyPrev()

	assert.Equal(t, "!ls .", m.input.Value())
}

func TestHistoryNext_UnifiedBarReconstructs(t *testing.T) {
	m := &model{
		input: initInput(),
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "jj", Command: "status"},
				{Prefix: "jj", Command: "log"},
			},
		},
		cmdPrefix:  prefixNone,
		historyIdx: 1,
	}

	m.historyNext()

	assert.Equal(t, "jj status", m.input.Value())
}

func TestUpdateHistoryFilter_PrefixNone(t *testing.T) {
	m := &model{
		input:      initInput(),
		cmdPrefix:  prefixNone,
		historyIdx: -1,
	}
	m.input.SetValue("jj status")

	m.updateHistoryFilter()

	assert.Equal(t, "jj", m.historyFilterPrefix)
}

func TestUpdateHistoryFilter_PrefixNoneMidNavigationSkips(t *testing.T) {
	m := &model{
		input:               initInput(),
		cmdPrefix:           prefixNone,
		historyIdx:          2,
		historyFilterPrefix: "jj",
	}
	m.input.SetValue("!ls .")

	m.updateHistoryFilter()

	assert.Equal(t, "jj", m.historyFilterPrefix)
}
