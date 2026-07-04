package tui

import (
	"fmt"

	"github.com/hugoh/hrd/internal/config"
)

// reloadConfig re-reads config from disk. New repos are auto-selected only
// if the whole prior selection was "all" — treating that as a standing
// mode rather than a frozen snapshot — otherwise a refresh could silently
// expand a curated selection. On a read error, m and its derived state are
// left untouched.
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

// mutateConfig reloads, mutates, and saves — the only safe way for the TUI
// to persist a change without clobbering a concurrent external write (e.g.
// a `hrd repo add` run while the TUI was open) with a stale in-memory copy.
// mutate must guard against names no longer present in the freshly loaded
// config rather than assuming the caller's selection is still valid.
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

// addSelectedToGroup tags the currently selected repos into group, via
// mutateConfig so a repo removed concurrently isn't resurrected.
func (m *model) addSelectedToGroup(group string) error {
	names := m.selectedNames()

	return m.mutateConfig(func(cfg *config.Config) {
		for _, repoName := range names {
			if _, ok := cfg.Repos[repoName]; !ok {
				continue
			}

			cfg.AddRepoToGroup(repoName, group)
		}
	})
}
