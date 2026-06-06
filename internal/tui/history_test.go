package tui

import (
	"testing"

	"charm.land/bubbles/v2/textinput"
)

func initInput() textinput.Model {
	ti := textinput.New()
	ti.CharLimit = 512

	return ti
}

func TestPushHistory(t *testing.T) {
	m := &model{persState: PersistentState{}}

	m.pushHistory("git", "status")

	if len(m.persState.History) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(m.persState.History))
	}

	if m.persState.History[0] != (HistoryEntry{Prefix: "git", Command: "status"}) {
		t.Errorf(
			"history[0] = %v, want %v",
			m.persState.History[0],
			HistoryEntry{Prefix: "git", Command: "status"},
		)
	}

	m.pushHistory("jj", "log")

	if len(m.persState.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(m.persState.History))
	}

	if m.persState.History[0] != (HistoryEntry{Prefix: "jj", Command: "log"}) {
		t.Errorf(
			"history[0] = %v, want %v",
			m.persState.History[0],
			HistoryEntry{Prefix: "jj", Command: "log"},
		)
	}

	if m.persState.History[1] != (HistoryEntry{Prefix: "git", Command: "status"}) {
		t.Errorf(
			"history[1] = %v, want %v",
			m.persState.History[1],
			HistoryEntry{Prefix: "git", Command: "status"},
		)
	}
}

func TestPushHistoryCap(t *testing.T) {
	m := &model{persState: PersistentState{}}

	for range maxHistoryLen + 5 {
		m.pushHistory("git", "")
	}

	if len(m.persState.History) > maxHistoryLen {
		t.Errorf("history length %d exceeds max %d", len(m.persState.History), maxHistoryLen)
	}
}

func TestPushHistorySavesEmptyPrefix(t *testing.T) {
	m := &model{persState: PersistentState{}}

	m.pushHistory("", "status")
	m.pushHistory("", "log")

	if len(m.persState.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(m.persState.History))
	}

	if m.persState.History[0] != (HistoryEntry{Prefix: "", Command: "log"}) {
		t.Errorf("history[0] = %v, want {Prefix: '' Command: 'log'}", m.persState.History[0])
	}

	if m.persState.History[1] != (HistoryEntry{Prefix: "", Command: "status"}) {
		t.Errorf("history[1] = %v, want {Prefix: '' Command: 'status'}", m.persState.History[1])
	}
}

func TestHistoryEntriesFiltersByPrefix(t *testing.T) {
	m := &model{
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "git", Command: "log"},
				{Prefix: "sh", Command: "ls"},
				{Prefix: "git", Command: "status"},
			},
		},
		cmdPrefix: prefixGit,
	}

	got := m.historyEntries()
	want := []HistoryEntry{
		{Prefix: "git", Command: "log"},
		{Prefix: "git", Command: "status"},
	}

	if len(got) != len(want) {
		t.Fatalf("historyEntries() = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("historyEntries()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestHistoryEntriesAllPrefixes(t *testing.T) {
	tests := []struct {
		prefix cmdPrefix
		want   int
	}{
		{prefixNone, 4},
		{prefixGit, 2},
		{prefixJj, 1},
		{prefixShell, 1},
	}

	history := []HistoryEntry{
		{Prefix: "git", Command: "log"},
		{Prefix: "git", Command: "status"},
		{Prefix: "jj", Command: "evolve"},
		{Prefix: "sh", Command: "make test"},
	}

	for _, tt := range tests {
		label := prefixLabels[tt.prefix]

		m := &model{
			persState: PersistentState{History: history},
			cmdPrefix: tt.prefix,
		}

		got := m.historyEntries()

		if len(got) != tt.want {
			t.Errorf("prefix %q: historyEntries() = %d entries, want %d", label, len(got), tt.want)
		}
	}
}

func TestHistoryEntriesEmpty(t *testing.T) {
	m := &model{
		persState: PersistentState{},
		cmdPrefix: prefixGit,
	}

	got := m.historyEntries()

	if got != nil {
		t.Errorf("historyEntries() = %v, want nil", got)
	}
}

func TestHistoryEntriesNoMatch(t *testing.T) {
	m := &model{
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "sh", Command: "ls"},
				{Prefix: "sh", Command: "pwd"},
			},
		},
		cmdPrefix: prefixGit,
	}

	got := m.historyEntries()

	if got != nil {
		t.Errorf("historyEntries() = %v, want nil", got)
	}
}

func TestHistoryPrevFirstCall(t *testing.T) {
	m := &model{
		input: initInput(),
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "git", Command: "log"},
				{Prefix: "git", Command: "status"},
			},
		},
		cmdPrefix:  prefixGit,
		historyIdx: -1,
	}

	m.historyPrev()

	if m.historyIdx != 0 {
		t.Errorf("historyIdx = %d, want 0", m.historyIdx)
	}

	if m.input.Value() != "log" {
		t.Errorf("input value = %q, want %q", m.input.Value(), "log")
	}
}

func TestHistoryPrevClampsAtEnd(t *testing.T) {
	m := &model{
		input: initInput(),
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "git", Command: "log"},
				{Prefix: "git", Command: "status"},
			},
		},
		cmdPrefix:  prefixGit,
		historyIdx: 1,
	}

	m.historyPrev()

	if m.historyIdx != 1 {
		t.Errorf("historyIdx = %d, want 1 (clamped)", m.historyIdx)
	}

	if m.input.Value() != "status" {
		t.Errorf("input value = %q, want %q", m.input.Value(), "status")
	}
}

func TestHistoryPrevNoEntries(t *testing.T) {
	m := &model{
		persState:  PersistentState{},
		cmdPrefix:  prefixGit,
		historyIdx: -1,
	}

	m.historyPrev()

	if m.historyIdx != -1 {
		t.Errorf("historyIdx = %d, want -1 (unchanged)", m.historyIdx)
	}
}

func TestHistoryNextFromNewest(t *testing.T) {
	m := &model{
		input: initInput(),
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "git", Command: "log"},
			},
		},
		cmdPrefix:  prefixGit,
		historyIdx: 0,
	}

	m.historyNext()

	if m.historyIdx != -1 {
		t.Errorf("historyIdx = %d, want -1", m.historyIdx)
	}

	if m.input.Value() != "" {
		t.Errorf("input value = %q, want empty", m.input.Value())
	}
}

func TestHistoryNextCyclesBack(t *testing.T) {
	m := &model{
		input: initInput(),
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "git", Command: "log"},
				{Prefix: "git", Command: "status"},
				{Prefix: "git", Command: "fetch"},
			},
		},
		cmdPrefix:  prefixGit,
		historyIdx: 2,
	}

	m.historyNext()

	if m.historyIdx != 1 {
		t.Errorf("historyIdx = %d, want 1", m.historyIdx)
	}

	if m.input.Value() != "status" {
		t.Errorf("input value = %q, want %q", m.input.Value(), "status")
	}
}

func TestHistoryNextAtMinBound(t *testing.T) {
	m := &model{
		input: initInput(),
		persState: PersistentState{
			History: []HistoryEntry{
				{Prefix: "git", Command: "log"},
			},
		},
		cmdPrefix:  prefixGit,
		historyIdx: -1,
	}

	m.historyNext()

	if m.historyIdx != -1 {
		t.Errorf("historyIdx = %d, want -1 (unchanged)", m.historyIdx)
	}

	if m.input.Value() != "" {
		t.Errorf("input value = %q, want empty", m.input.Value())
	}
}

func TestHistoryReset(t *testing.T) {
	m := &model{historyIdx: 5, historyFilterPrefix: "git"}

	m.historyReset()

	if m.historyIdx != -1 {
		t.Errorf("historyIdx = %d, want -1", m.historyIdx)
	}

	if m.historyFilterPrefix != "" {
		t.Errorf("historyFilterPrefix = %q, want empty", m.historyFilterPrefix)
	}
}

func TestFormatHistoryEntry_PrefixNoneSh(t *testing.T) {
	got := formatHistoryEntry(prefixNone, HistoryEntry{Prefix: "sh", Command: "ls ."})

	want := "!ls ."
	if got != want {
		t.Errorf("formatHistoryEntry() = %q, want %q", got, want)
	}
}

func TestFormatHistoryEntry_PrefixNoneGit(t *testing.T) {
	got := formatHistoryEntry(prefixNone, HistoryEntry{Prefix: "git", Command: "log"})

	want := "git log"
	if got != want {
		t.Errorf("formatHistoryEntry() = %q, want %q", got, want)
	}
}

func TestFormatHistoryEntry_PrefixNoneJj(t *testing.T) {
	got := formatHistoryEntry(prefixNone, HistoryEntry{Prefix: "jj", Command: "status"})

	want := "jj status"
	if got != want {
		t.Errorf("formatHistoryEntry() = %q, want %q", got, want)
	}
}

func TestFormatHistoryEntry_PrefixNoneEmpty(t *testing.T) {
	got := formatHistoryEntry(prefixNone, HistoryEntry{Prefix: "", Command: "status"})

	want := "status"
	if got != want {
		t.Errorf("formatHistoryEntry() = %q, want %q", got, want)
	}
}

func TestFormatHistoryEntry_PerVcsBar(t *testing.T) {
	got := formatHistoryEntry(prefixGit, HistoryEntry{Prefix: "git", Command: "log"})

	want := "log"
	if got != want {
		t.Errorf("formatHistoryEntry() = %q, want %q", got, want)
	}
}

func TestHistoryFilterFromInput_Shell(t *testing.T) {
	m := &model{input: initInput()}
	m.input.SetValue("!ls .")

	if got := m.historyFilterFromInput(); got != "sh" {
		t.Errorf("historyFilterFromInput() = %q, want %q", got, "sh")
	}
}

func TestHistoryFilterFromInput_Jj(t *testing.T) {
	m := &model{input: initInput()}
	m.input.SetValue("jj status")

	if got := m.historyFilterFromInput(); got != "jj" {
		t.Errorf("historyFilterFromInput() = %q, want %q", got, "jj")
	}
}

func TestHistoryFilterFromInput_Git(t *testing.T) {
	m := &model{input: initInput()}
	m.input.SetValue("git log")

	if got := m.historyFilterFromInput(); got != "git" {
		t.Errorf("historyFilterFromInput() = %q, want %q", got, "git")
	}
}

func TestHistoryFilterFromInput_Empty(t *testing.T) {
	m := &model{input: initInput()}
	if got := m.historyFilterFromInput(); got != "" {
		t.Errorf("historyFilterFromInput() = %q, want empty", got)
	}
}

func TestHistoryFilterFromInput_NoPrefix(t *testing.T) {
	m := &model{input: initInput()}
	m.input.SetValue("status")

	if got := m.historyFilterFromInput(); got != "" {
		t.Errorf("historyFilterFromInput() = %q, want empty", got)
	}
}

func TestHistoryEntries_FilteredByHistoryFilterPrefix(t *testing.T) {
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

	if len(got) != len(want) {
		t.Fatalf("historyEntries() = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("historyEntries()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
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

	got := m.historyEntries()
	if len(got) != 1 {
		t.Errorf("historyEntries() = %d entries, want 1", len(got))
	}
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

	if m.input.Value() != "jj status" {
		t.Errorf("input value = %q, want %q", m.input.Value(), "jj status")
	}
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

	if m.input.Value() != "!ls ." {
		t.Errorf("input value = %q, want %q", m.input.Value(), "!ls .")
	}
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

	if m.input.Value() != "jj status" {
		t.Errorf("input value = %q, want %q", m.input.Value(), "jj status")
	}
}

func TestUpdateHistoryFilter_PrefixNone(t *testing.T) {
	m := &model{
		input:      initInput(),
		cmdPrefix:  prefixNone,
		historyIdx: -1,
	}
	m.input.SetValue("jj status")

	m.updateHistoryFilter()

	if m.historyFilterPrefix != "jj" {
		t.Errorf("historyFilterPrefix = %q, want %q", m.historyFilterPrefix, "jj")
	}
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

	if m.historyFilterPrefix != "jj" {
		t.Errorf(
			"historyFilterPrefix = %q, want %q (should not re-derive mid-flight)",
			m.historyFilterPrefix,
			"jj",
		)
	}
}

func TestUpdateHistoryFilter_PerVcsBarNoop(t *testing.T) {
	m := &model{
		input:               initInput(),
		cmdPrefix:           prefixGit,
		historyFilterPrefix: "old",
	}
	m.input.SetValue("status")

	m.updateHistoryFilter()

	if m.historyFilterPrefix != "old" {
		t.Errorf("historyFilterPrefix = %q, want %q (unchanged)", m.historyFilterPrefix, "old")
	}
}
