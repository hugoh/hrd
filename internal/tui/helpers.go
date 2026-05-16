package tui

import (
	"sort"

	"github.com/hugoh/hrd/internal/config"
)

func (m *model) selectedPrefix() string {
	return prefixLabels[m.cmdPrefix]
}

func (m *model) groupLabel() string {
	if m.groupFilter != "" {
		return "@" + m.groupFilter
	}

	if m.cfg.Context.Current != "" {
		return "@" + m.cfg.Context.Current
	}

	return labelAll
}

func (m *model) activeRepoOrder() []string {
	var names []string

	if m.groupFilter == "" {
		names = m.allReposFallback()
	} else if g, ok := m.cfg.Groups[m.groupFilter]; ok {
		names = g.Repos
	} else {
		names = m.repoOrder
	}

	sort.Strings(names)

	return names
}

func (m *model) allReposFallback() []string {
	if m.cfg.Context.Current != "" {
		if g, ok := m.cfg.Groups[m.cfg.Context.Current]; ok {
			return g.Repos
		}
	}

	return m.repoOrder
}

func (m *model) filteredRepos() []string {
	return m.activeRepoOrder()
}

// tableRepos returns the repos that should be shown in the table.
// In selectMode all group repos are shown; otherwise only repos
// that are in the selected map are visible.
func (m *model) tableRepos() []string {
	all := m.filteredRepos()
	if m.selectMode {
		return all
	}

	var shown []string

	for _, n := range all {
		if m.selected[n] {
			shown = append(shown, n)
		}
	}

	return shown
}

func (m *model) selectedNames() []string {
	var names []string

	for _, name := range m.activeRepoOrder() {
		if m.selected[name] {
			names = append(names, name)
		}
	}

	return names
}

func (m *model) selectedCount() int {
	count := 0

	for _, name := range m.activeRepoOrder() {
		if m.selected[name] {
			count++
		}
	}

	return count
}

func (m *model) totalCount() int {
	return len(m.activeRepoOrder())
}

func (m *model) allSelected() bool {
	for _, name := range m.activeRepoOrder() {
		if !m.selected[name] {
			return false
		}
	}

	return true
}

func resolveInitialPrefix(lastPrefix string) int {
	for i, label := range prefixLabels {
		if label == lastPrefix {
			return i
		}
	}

	return int(prefixGit)
}

func sortedRepoKeys(repos map[string]config.Repo) []string {
	keys := make([]string, 0, len(repos))
	for name := range repos {
		keys = append(keys, name)
	}

	sort.Strings(keys)

	return keys
}

func restoreSelected(lastRepos []string, repos map[string]config.Repo) map[string]bool {
	selected := make(map[string]bool)

	for _, name := range lastRepos {
		if _, ok := repos[name]; ok {
			selected[name] = true
		}
	}

	return selected
}

func resolveGroupFilter(optGroup, lastGroup string, cfg config.Config) string {
	if optGroup != "" {
		return optGroup
	}

	if lastGroup != "" {
		if _, ok := cfg.Groups[lastGroup]; ok {
			return lastGroup
		}
	}

	if cfg.Context.Current != "" {
		if _, ok := cfg.Groups[cfg.Context.Current]; ok {
			return cfg.Context.Current
		}
	}

	return ""
}

func sortedGroupNames(groups map[string]config.Group) []string {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
