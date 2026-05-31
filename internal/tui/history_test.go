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

func TestPushHistorySkipsEmptyPrefix(t *testing.T) {
	m := &model{persState: PersistentState{}}

	m.pushHistory("", "status")
	m.pushHistory("", "log")

	if len(m.persState.History) != 0 {
		t.Errorf("expected empty history, got %d entries", len(m.persState.History))
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
		{prefixNone, 0},
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
	m := &model{historyIdx: 5}

	m.historyReset()

	if m.historyIdx != -1 {
		t.Errorf("historyIdx = %d, want -1", m.historyIdx)
	}
}
