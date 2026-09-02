package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/sahilm/fuzzy"
)

var errUnknownGroup = errors.New("unknown group")

const (
	labelAllRepos  = "[all]"
	labelUngrouped = "[ungrouped]"
)

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
	covered := make(map[string]struct{}, len(repos))

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

	slices.Sort(labels)

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

// stripGroupPrefix removes a leading '@' from a group name if present,
// letting the group-add text input accept @work or work interchangeably.
func stripGroupPrefix(name string) string {
	return strings.TrimPrefix(name, "@")
}

func (m *model) groupLabel() string {
	switch {
	case m.groupFilter == config.ReservedNone:
		return "ungrouped"
	case m.groupFilter != "":
		return "@" + m.groupFilter
	default:
		return labelAll
	}
}

// activeRepoOrder returns the repos in scope: the filtered group's repos
// when a group filter is active, otherwise all repos, further narrowed by
// the "/" name filter. Both repoOrder and group repo lists are kept sorted
// at construction, so no sorting happens here — this runs several times
// per render. An unknown filter (stale persisted state) falls back to all
// repos.
func (m *model) activeRepoOrder() []string {
	base := m.repoOrder

	switch {
	case m.groupFilter == config.ReservedNone:
		base = m.cfg.UngroupedRepos()
	case m.groupFilter != "":
		if g, ok := m.cfg.Groups[m.groupFilter]; ok {
			base = g.Repos
		}
	}

	if m.attentionFilter {
		base = filterByAttention(base, m.statuses)
	}

	if m.nameFilter == "" {
		return base
	}

	return fuzzyFilter(base, m.nameFilter)
}

// filterByAttention narrows names to repos needing attention (dirty or out
// of sync with their remote). Repos with no loaded status yet, or whose last
// status fetch errored, are excluded.
func filterByAttention(names []string, statuses map[string]runner.StatusResult) []string {
	out := make([]string, 0, len(names))

	for _, n := range names {
		if res, ok := statuses[n]; ok && res.Err == nil && res.Status.NeedsAttention() {
			out = append(out, n)
		}
	}

	return out
}

// fuzzyFilter returns the names fuzzy-matching pattern, preserving the
// input order rather than match-score order so the table stays stable
// while typing.
func fuzzyFilter(names []string, pattern string) []string {
	matched := make(map[int]struct{})
	for _, match := range fuzzy.Find(pattern, names) {
		matched[match.Index] = struct{}{}
	}

	out := make([]string, 0, len(matched))

	for i, name := range names {
		if _, ok := matched[i]; ok {
			out = append(out, name)
		}
	}

	return out
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

// execResultNames returns the deduped repo names that produced an exec
// result, in arrival order — the set a side-effecting command actually
// touched, independent of what is selected once it finishes.
func execResultNames(results []execResult) []string {
	seen := make(map[string]bool, len(results))
	names := make([]string, 0, len(results))

	for _, er := range results {
		if er.name == "" || seen[er.name] {
			continue
		}

		seen[er.name] = true
		names = append(names, er.name)
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

	slices.Sort(keys)

	return keys
}

func restoreSelected(lastRepos []string, repos map[string]config.Repo) map[string]bool {
	selected := make(map[string]bool, len(lastRepos))

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

	slices.Sort(names)

	return names
}
