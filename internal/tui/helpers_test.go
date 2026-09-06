package tui

import (
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/stretchr/testify/assert"
)

func TestSortedRepoKeys(t *testing.T) {
	repos := map[string]config.Repo{
		"zeta":  {},
		"alpha": {},
		"beta":  {},
	}

	assert.Equal(t, []string{"alpha", "beta", "zeta"}, sortedRepoKeys(repos))
}

func TestSortedRepoKeysEmpty(t *testing.T) {
	assert.Empty(t, sortedRepoKeys(map[string]config.Repo{}))
}

func TestSortedGroupNames(t *testing.T) {
	groups := map[string]config.Group{
		"work":     {},
		"personal": {},
	}

	assert.Equal(t, []string{"personal", "work"}, sortedGroupNames(groups))
}

func TestSortedGroupNamesEmpty(t *testing.T) {
	assert.Empty(t, sortedGroupNames(map[string]config.Group{}))
}

func TestGroupLabel(t *testing.T) {
	m := &model{cfg: config.Config{
		Groups: map[string]config.Group{"work": {}},
	}}

	t.Run("with group filter", func(t *testing.T) {
		m.groupFilter = "personal"
		assert.Equal(t, "@personal", m.groupLabel())
	})

	t.Run("no filter", func(t *testing.T) {
		m.groupFilter = ""
		assert.Equal(t, "all", m.groupLabel())
	})

	t.Run("ungrouped filter", func(t *testing.T) {
		m.groupFilter = config.ReservedNone
		assert.Equal(t, "ungrouped", m.groupLabel())
	})
}

func TestActiveRepoOrder(t *testing.T) {
	// repoOrder and group repo lists are sorted at construction;
	// activeRepoOrder preserves that order without re-sorting.
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		cfg: config.Config{
			Groups: map[string]config.Group{"work": {Repos: []string{"a", "b"}}},
		},
	}

	t.Run("no group filter uses repoOrder", func(t *testing.T) {
		m.groupFilter = ""
		assert.Equal(t, []string{"a", "b", "c"}, m.activeRepoOrder())
	})

	t.Run("known group filter", func(t *testing.T) {
		m.groupFilter = "work"
		assert.Equal(t, []string{"a", "b"}, m.activeRepoOrder())
	})

	t.Run("unknown group filter falls back to all repos", func(t *testing.T) {
		m.groupFilter = "nonexistent"
		assert.Equal(t, []string{"a", "b", "c"}, m.activeRepoOrder())
	})

	t.Run("empty group filter uses repoOrder", func(t *testing.T) {
		m.groupFilter = ""
		m.cfg.Groups = map[string]config.Group{}
		assert.Equal(t, []string{"a", "b", "c"}, m.activeRepoOrder())
	})

	t.Run("reserved none filter uses ungrouped repos", func(t *testing.T) {
		m.groupFilter = config.ReservedNone
		m.cfg = config.Config{
			Repos: map[string]config.Repo{
				"a": {Groups: []string{"work"}},
				"b": {},
				"c": {},
			},
		}
		assert.Equal(t, []string{"b", "c"}, m.activeRepoOrder())
	})
}

func TestActiveRepoOrder_AttentionFilter(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		cfg: config.Config{
			Groups: map[string]config.Group{"work": {Repos: []string{"a", "b"}}},
		},
		statuses: map[string]runner.StatusResult{
			"a": {Status: backend.RepoStatus{Dirty: true}},
			"b": {Status: backend.RepoStatus{}},
			"c": {Err: assert.AnError},
		},
	}

	t.Run("narrows to repos needing attention", func(t *testing.T) {
		m.groupFilter = ""
		m.attentionFilter = true
		assert.Equal(t, []string{"a"}, m.activeRepoOrder())
	})

	t.Run("composes with group filter", func(t *testing.T) {
		m.groupFilter = "work"
		m.attentionFilter = true
		assert.Equal(t, []string{"a"}, m.activeRepoOrder())
	})

	t.Run("composes with name filter", func(t *testing.T) {
		m.groupFilter = ""
		m.attentionFilter = true
		m.nameFilter = "a"
		assert.Equal(t, []string{"a"}, m.activeRepoOrder())
		m.nameFilter = ""
	})

	t.Run("off returns unfiltered", func(t *testing.T) {
		m.groupFilter = ""
		m.attentionFilter = false
		assert.Equal(t, []string{"a", "b", "c"}, m.activeRepoOrder())
	})
}

func TestSelectedNames(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "c": true},
	}

	assert.Equal(t, []string{"a", "c"}, m.selectedNames())
}

func TestSelectedNamesEmpty(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{},
	}

	assert.Empty(t, m.selectedNames())
}

func TestSelectedNames_SingleModeReturnsCursorRepo(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "c": true},
		mode:      modeSingle,
	}
	m.cursor = 1

	assert.Equal(t, []string{"c"}, m.selectedNames())
}

func TestSelectedNames_SingleModeNoneWhenCursorOutOfRange(t *testing.T) {
	m := &model{
		repoOrder: []string{"a"},
		selected:  map[string]bool{"a": true},
		mode:      modeSingle,
	}
	m.cursor = 5

	assert.Empty(t, m.selectedNames())
}

func TestSelectedNames_SingleModeReturnsEmptyWhenNoneSelected(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{},
		mode:      modeSingle,
	}
	m.cursor = 1

	assert.Empty(t, m.selectedNames())
}

func TestAllSelected(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "b": true, "c": true},
	}

	assert.True(t, m.allSelected())
}

func TestAllSelectedNotAll(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "c": true},
	}

	assert.False(t, m.allSelected())
}

func TestAllSelectedEmpty(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{},
	}

	assert.False(t, m.allSelected())
}

func TestTableRepos_SelectModeReturnsAll(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true},
		mode:      modeSelect,
	}

	assert.Equal(t, []string{"a", "b", "c"}, m.tableRepos())
}

func TestTableRepos_SelectedOnlyWhenNotInSelectMode(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"b": true},
		mode:      modeNormal,
	}

	assert.Equal(t, []string{"b"}, m.tableRepos())
}

func TestSelectedCount(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "c": true},
	}

	assert.Equal(t, 2, m.selectedCount())
}

func TestSelectedCountEmpty(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{},
	}

	assert.Equal(t, 0, m.selectedCount())
}

func TestTotalCount(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
	}

	assert.Equal(t, 3, m.totalCount())
}

func TestTableRepos_EmptyWhenNoneSelected(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{},
		mode:      modeNormal,
	}

	assert.Empty(t, m.tableRepos())
}

func TestMatchingGroups_AllRepos(t *testing.T) {
	repos := []string{"a", "b", "c"}
	allRepoSet := map[string]struct{}{
		"a": {}, "b": {}, "c": {},
	}

	labels, uncovered := matchingGroups(repos, allRepoSet, nil)
	assert.Equal(t, []string{"[all]"}, labels)
	assert.Empty(t, uncovered)
}

func TestMatchingGroups_SubsetOneGroup(t *testing.T) {
	repos := []string{"a", "b", "c"}
	groups := map[string]config.Group{
		"work": {Repos: []string{"a", "b"}},
	}

	labels, uncovered := matchingGroups(repos, nil, groups)
	assert.Equal(t, []string{"@work"}, labels)
	assert.Equal(t, []string{"c"}, uncovered)
}

func TestMatchingGroups_SubsetMultiGroup(t *testing.T) {
	repos := []string{"a", "b", "c", "d"}
	groups := map[string]config.Group{
		"work": {Repos: []string{"a", "b"}},
		"team": {Repos: []string{"b", "c"}},
	}

	labels, uncovered := matchingGroups(repos, nil, groups)
	assert.Equal(t, []string{"@team", "@work"}, labels)
	assert.Equal(t, []string{"d"}, uncovered)
}

func TestMatchingGroups_AllCovered(t *testing.T) {
	repos := []string{"a", "b"}
	groups := map[string]config.Group{
		"work": {Repos: []string{"a", "b"}},
	}

	labels, uncovered := matchingGroups(repos, nil, groups)
	assert.Equal(t, []string{"@work"}, labels)
	assert.Empty(t, uncovered)
}

func TestMatchingGroups_AllCoveredMulti(t *testing.T) {
	repos := []string{"a", "b"}
	groups := map[string]config.Group{
		"work": {Repos: []string{"a", "b"}},
		"team": {Repos: []string{"a", "b"}},
	}

	labels, uncovered := matchingGroups(repos, nil, groups)
	assert.Equal(t, []string{"@team", "@work"}, labels)
	assert.Empty(t, uncovered)
}

func TestMatchingGroups_NoMatch(t *testing.T) {
	repos := []string{"a", "b"}
	groups := map[string]config.Group{
		"work": {Repos: []string{"c", "d"}},
	}

	labels, uncovered := matchingGroups(repos, nil, groups)
	assert.Nil(t, labels)
	assert.Equal(t, repos, uncovered)
}

func TestMatchingGroups_EmptyGroups(t *testing.T) {
	repos := []string{"a", "b"}

	labels, uncovered := matchingGroups(repos, nil, map[string]config.Group{})
	assert.Nil(t, labels)
	assert.Equal(t, repos, uncovered)
}

func TestMatchingGroups_UnsortedGroupRepos(t *testing.T) {
	repos := []string{"a", "b"}
	groups := map[string]config.Group{
		"work": {Repos: []string{"b", "a"}},
	}

	labels, uncovered := matchingGroups(repos, nil, groups)
	assert.Equal(t, []string{"@work"}, labels)
	assert.Empty(t, uncovered)
}

func TestMakeSelectedMap(t *testing.T) {
	got := makeSelectedMap([]string{"b", "a"})
	assert.True(t, got["a"])
	assert.True(t, got["b"])
	assert.False(t, got["c"])
}

func TestAdjacentOffset(t *testing.T) {
	offsets := []int{0, 5, 12, 30}

	tests := []struct {
		name    string
		current int
		dir     int
		want    int
		wantOK  bool
	}{
		{"next from top", 0, 1, 5, true},
		{"next from between", 6, 1, 12, true},
		{"next past last", 30, 1, 0, false},
		{"prev from last", 30, -1, 12, true},
		{"prev from between", 6, -1, 5, true},
		{"prev before first", 0, -1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := adjacentOffset(offsets, tt.current, tt.dir)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAdjacentOffsetEmpty(t *testing.T) {
	_, ok := adjacentOffset(nil, 0, 1)
	assert.False(t, ok)
}
