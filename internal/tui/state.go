// Package tui provides the terminal UI for hrd, built on Bubble Tea. It
// manages repo list display, command input, output streaming, group
// filtering, and persistent state (history, selections) across sessions.
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/renameio/v2"
)

const currentStateVersion = 3

const selectionHistoryCap = 50

const (
	stateDirPerm  = 0o750
	stateFilePerm = 0o600
)

// HistoryEntry is a single typed command in execution history.
type HistoryEntry struct {
	Prefix  string `json:"p"`
	Command string `json:"c"`
}

// SelectionEntry records a set of selected repos at a point in time.
type SelectionEntry struct {
	Timestamp time.Time `json:"t,omitzero"`
	Repos     []string  `json:"r"`
}

// PersistentState holds TUI session state saved between runs.
type PersistentState struct {
	Version          int              `json:"version"`
	History          []HistoryEntry   `json:"history"`
	SelectionHistory []SelectionEntry `json:"selectionHistory"`
	LastRepos        []string         `json:"lastRepos"`
	LastGroup        string           `json:"lastGroup"`
}

// defaultStatePath returns the default path for the TUI state file.
func defaultStatePath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "hrd", "tui-state.json")
	}

	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".config", "hrd", "tui-state.json")
}

// loadState reads the persistent state from path. Missing file is not an
// error — a fresh state is returned.
//
//nolint:gosec // reading user-owned state file
func loadState(
	path string,
) (PersistentState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PersistentState{Version: currentStateVersion}, nil
		}

		return PersistentState{}, fmt.Errorf("reading tui state: %w", err)
	}

	var state PersistentState
	if err := json.Unmarshal(raw, &state); err != nil {
		// Corrupt state file — start fresh.
		return PersistentState{Version: currentStateVersion}, nil //nolint:nilerr
	}

	if state.Version != currentStateVersion {
		state.Version = currentStateVersion
		state.History = nil
		state.SelectionHistory = nil
	}

	if state.History == nil {
		state.History = []HistoryEntry{}
	}

	if state.SelectionHistory == nil {
		state.SelectionHistory = []SelectionEntry{}
	}

	return state, nil
}

// saveState writes the persistent state to path.
func saveState(path string, state PersistentState) error {
	state.Version = currentStateVersion

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling tui state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), stateDirPerm); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}

	if err := renameio.WriteFile(path, raw, stateFilePerm); err != nil {
		return fmt.Errorf("writing tui state: %w", err)
	}

	return nil
}
