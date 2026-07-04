package tui

import (
	"fmt"

	"github.com/hugoh/hrd/internal/config"
)

// reloadConfig re-reads config from disk, replacing m.cfg and recomputing
// repoOrder/groupFilter/selected while best-effort preserving the current
// selection and cursor position. Repos that disappeared are dropped from
// the selection. Newly-appeared repos are auto-selected only if every
// previously-known repo was already selected (i.e. the session was in a
// "select all" state) — that preserves "all repos" as a standing mode
// rather than a frozen snapshot. Otherwise a refresh can't silently expand
// a curated selection, so new repos start unselected and the user opts in
// explicitly (e.g. via the "a" select-all toggle).
// On any read error, m.cfg and all derived state are left untouched.
func (m *model) reloadConfig() error {
	fresh, err := config.Load(m.opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("reloading config: %w", err)
	}

	newSelected := m.rebuildSelection(fresh)

	newGroupFilter := m.groupFilter
	if newGroupFilter != "" {
		if _, ok := fresh.Groups[newGroupFilter]; !ok {
			newGroupFilter = ""
		}
	}

	cur := m.tableRepos()

	cursorName := ""
	if m.cursor >= 0 && m.cursor < len(cur) {
		cursorName = cur[m.cursor]
	}

	m.cfg = fresh
	m.repoOrder = sortedRepoKeys(fresh.Repos)
	m.selected = newSelected
	m.groupFilter = newGroupFilter
	m.updateTableRows()

	m.restoreCursorByName(cursorName)

	return nil
}

// rebuildSelection prunes m.selected to repos still present in fresh, and
// auto-selects newly-appeared repos when the prior selection covered every
// previously-known repo (see reloadConfig's doc comment).
func (m *model) rebuildSelection(fresh config.Config) map[string]bool {
	wasAllSelected := true

	for _, name := range m.repoOrder {
		if !m.selected[name] {
			wasAllSelected = false

			break
		}
	}

	newSelected := make(map[string]bool, len(m.selected))

	for name, sel := range m.selected {
		if _, ok := fresh.Repos[name]; ok {
			newSelected[name] = sel
		}
	}

	if wasAllSelected {
		for name := range fresh.Repos {
			if _, ok := newSelected[name]; !ok {
				newSelected[name] = true
			}
		}
	}

	return newSelected
}

// restoreCursorByName re-finds the given repo name in the (possibly
// reordered) table and moves the cursor to it. A no-op if name is empty or
// no longer present.
func (m *model) restoreCursorByName(name string) {
	if name == "" {
		return
	}

	for i, n := range m.tableRepos() {
		if n == name {
			m.cursor = i
			m.repoTable.SetCursor(i)

			return
		}
	}
}

// mutateConfig reloads config fresh from disk, applies mutate to it, saves
// the result, and — only on success — updates m.cfg to match. This is the
// only safe way for the TUI to persist a config change: it guarantees a
// TUI-initiated save can never clobber a concurrent external change (e.g. a
// `hrd repo add` run while the TUI was open) with a stale in-memory copy.
// mutate must guard against names that no longer exist in the freshly
// loaded config (e.g. a repo removed concurrently) rather than assuming
// the caller's stale selection is still valid.
func (m *model) mutateConfig(mutate func(cfg *config.Config)) error {
	fresh, err := config.Load(m.opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("reloading config: %w", err)
	}

	mutate(&fresh)

	if err := config.Save(m.opts.ConfigPath, fresh); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	m.cfg = fresh

	return nil
}
