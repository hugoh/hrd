package tui

import "strings"

const maxHistoryLen = 100

// pushHistory prepends a history entry. Only called for typed commands
// (via handleInputKey), not for keyboard shortcuts.
func (m *model) pushHistory(prefix, cmdStr string) {
	entry := HistoryEntry{Prefix: prefix, Command: cmdStr}

	m.persState.History = append([]HistoryEntry{entry}, m.persState.History...)
	if len(m.persState.History) > maxHistoryLen {
		m.persState.History = m.persState.History[:maxHistoryLen]
	}
}

// formatHistoryEntry reconstructs the full command text from a history entry
// for display in the unified command bar. For per-VCS bars the bare command
// is returned directly.
func formatHistoryEntry(cp cmdPrefix, e HistoryEntry) string {
	if cp != prefixNone || e.Prefix == "" {
		return e.Command
	}

	if e.Prefix == "sh" {
		return "!" + e.Command
	}

	return e.Prefix + " " + e.Command
}

// historyFilterFromInput derives a history filter prefix from the current
// input value. Empty input returns "" (all history).
func (m *model) historyFilterFromInput() string {
	input := m.input.Value()

	if strings.HasPrefix(input, "!") {
		return "sh"
	}

	if strings.HasPrefix(input, "jj ") {
		return "jj"
	}

	if strings.HasPrefix(input, "git ") {
		return vcsGit
	}

	return ""
}

// historyEntries returns history entries matching the current command
// prefix, newest first. Returns nil when there are no matches.
// Unified command bar (prefixNone) uses historyFilterPrefix to scope.
func (m *model) historyEntries() []HistoryEntry {
	var filter string

	if m.cmdPrefix == prefixNone {
		filter = m.historyFilterPrefix
	} else {
		filter = prefixLabels[m.cmdPrefix]
	}

	if filter == "" {
		return m.persState.History
	}

	var out []HistoryEntry

	for _, e := range m.persState.History {
		if e.Prefix == filter {
			out = append(out, e)
		}
	}

	return out
}

// historyPrev moves backward in history (older entry) and fills the
// input with the full command. Does nothing when no history matches.
func (m *model) historyPrev() {
	entries := m.historyEntries()
	if len(entries) == 0 {
		return
	}

	m.historyIdx = min(m.historyIdx+1, len(entries)-1)
	e := entries[m.historyIdx]
	cmd := formatHistoryEntry(m.cmdPrefix, e)
	m.input.SetValue(cmd)
	m.input.SetCursor(len(cmd))
}

// historyNext moves forward in history (newer entry) and fills the
// input with the full command. Returns to a blank input when past
// the newest entry.
func (m *model) historyNext() {
	if m.historyIdx <= 0 {
		m.historyIdx = -1
		m.input.SetValue("")

		return
	}

	m.historyIdx--
	entries := m.historyEntries()
	e := entries[m.historyIdx]
	cmd := formatHistoryEntry(m.cmdPrefix, e)
	m.input.SetValue(cmd)
	m.input.SetCursor(len(cmd))
}

// historyReset clears the history navigation index and filter so the
// next up-arrow starts from the most recent matching entry.
func (m *model) historyReset() {
	m.historyIdx = -1
	m.historyFilterPrefix = ""
}

// updateHistoryFilter derives a filter prefix from the current input for
// the unified command bar. No-op for per-VCS bars.
func (m *model) updateHistoryFilter() {
	if m.cmdPrefix != prefixNone {
		return
	}

	if m.historyIdx == -1 {
		m.historyFilterPrefix = m.historyFilterFromInput()
	}
}
