package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hugoh/hrd/internal/config"
)

var errUnknownGroup = errors.New("unknown group")

const labelAllRepos = "[all]"

func matchingGroups(
	repos []string,
	allRepoSet map[string]struct{},
	groups map[string]config.Group,
) ([]string, []string) {
	if isAllRepos(repos, allRepoSet) {
		return []string{labelAllRepos}, nil
	}

	if len(groups) == 0 {
		return nil, repos
	}

	repoSet := buildSet(repos)
	covered := make(map[string]struct{})

	var labels []string

	for name, g := range groups {
		if len(g.Repos) == 0 {
			continue
		}

		if !isSubset(g.Repos, repoSet) {
			continue
		}

		labels = append(labels, "@"+name)

		for _, r := range g.Repos {
			covered[r] = struct{}{}
		}
	}

	sort.Strings(labels)

	if len(labels) == 0 {
		return nil, repos
	}

	return labels, setDifference(repos, covered)
}

func buildSet(repos []string) map[string]struct{} {
	set := make(map[string]struct{}, len(repos))
	for _, r := range repos {
		set[r] = struct{}{}
	}

	return set
}

func isSubset(repos []string, repoSet map[string]struct{}) bool {
	for _, r := range repos {
		if _, ok := repoSet[r]; !ok {
			return false
		}
	}

	return true
}

func isAllRepos(repos []string, allRepoSet map[string]struct{}) bool {
	if len(repos) != len(allRepoSet) {
		return false
	}

	if len(repos) == 0 {
		return false
	}

	for _, r := range repos {
		if _, ok := allRepoSet[r]; !ok {
			return false
		}
	}

	return true
}

func setDifference(repos []string, covered map[string]struct{}) []string {
	var diff []string

	for _, r := range repos {
		if _, ok := covered[r]; !ok {
			diff = append(diff, r)
		}
	}

	return diff
}

func (m *model) groupLabel() string {
	if m.groupFilter != "" {
		return "@" + m.groupFilter
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
		m.groupFilter = ""
		names = m.allReposFallback()
	}

	sort.Strings(names)

	return names
}

func (m *model) allReposFallback() []string {
	return m.repoOrder
}

func (m *model) filteredRepos() []string {
	return m.activeRepoOrder()
}

// tableRepos returns the repos that should be shown in the table.
// In modeSelect all group repos are shown; otherwise only repos
// that are in the selected map are visible.
func (m *model) tableRepos() []string {
	all := m.filteredRepos()
	if m.mode == modeSelect {
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
	if m.mode == modeSingle {
		names := m.tableRepos()
		if m.cursor >= 0 && m.cursor < len(names) {
			return []string{names[m.cursor]}
		}

		return nil
	}

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

func makeSelectedMap(names []string) map[string]bool {
	selected := make(map[string]bool, len(names))
	for _, n := range names {
		selected[n] = true
	}

	return selected
}

func resolveGroupFilter(optGroup, lastGroup string, cfg config.Config) (string, error) {
	if optGroup != "" {
		stripped := strings.TrimPrefix(optGroup, "@")
		if _, ok := cfg.Groups[stripped]; ok {
			return stripped, nil
		}

		return "", fmt.Errorf("%w: %s", errUnknownGroup, optGroup)
	}

	if lastGroup != "" {
		if _, ok := cfg.Groups[lastGroup]; ok {
			return lastGroup, nil
		}
	}

	return "", nil
}

func sortedGroupNames(groups map[string]config.Group) []string {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
