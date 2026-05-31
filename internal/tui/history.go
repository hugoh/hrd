package tui

const maxHistoryLen = 100

// pushHistory prepends a history entry. Shortcut commands (empty prefix)
// are not saved to avoid polluting typed-command history.
func (m *model) pushHistory(prefix, cmdStr string) {
	if prefix == "" {
		return
	}

	entry := HistoryEntry{Prefix: prefix, Command: cmdStr}

	m.persState.History = append([]HistoryEntry{entry}, m.persState.History...)
	if len(m.persState.History) > maxHistoryLen {
		m.persState.History = m.persState.History[:maxHistoryLen]
	}
}

// historyEntries returns history entries matching the current command
// prefix, newest first. Returns nil when there are no matches.
func (m *model) historyEntries() []HistoryEntry {
	prefix := prefixLabels[m.cmdPrefix]

	var out []HistoryEntry

	for _, e := range m.persState.History {
		if e.Prefix == prefix {
			out = append(out, e)
		}
	}

	return out
}

// historyPrev moves backward in history (older entry) and fills the
// input with that command. Does nothing when no history matches.
func (m *model) historyPrev() {
	entries := m.historyEntries()
	if len(entries) == 0 {
		return
	}

	m.historyIdx = min(m.historyIdx+1, len(entries)-1)
	m.input.SetValue(entries[m.historyIdx].Command)
	m.input.SetCursor(len(entries[m.historyIdx].Command))
}

// historyNext moves forward in history (newer entry) and fills the
// input. Returns to a blank input when past the newest entry.
func (m *model) historyNext() {
	if m.historyIdx <= 0 {
		m.historyIdx = -1
		m.input.SetValue("")

		return
	}

	m.historyIdx--
	entries := m.historyEntries()
	m.input.SetValue(entries[m.historyIdx].Command)
	m.input.SetCursor(len(entries[m.historyIdx].Command))
}

// historyReset clears the history navigation index so the next up-arrow
// starts from the most recent matching entry.
func (m *model) historyReset() {
	m.historyIdx = -1
}
