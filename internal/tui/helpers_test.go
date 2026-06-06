package tui

import (
	"reflect"
	"testing"

	"github.com/hugoh/hrd/internal/config"
)

func TestSortedRepoKeys(t *testing.T) {
	repos := map[string]config.Repo{
		"zeta":  {},
		"alpha": {},
		"beta":  {},
	}
	want := []string{"alpha", "beta", "zeta"}

	got := sortedRepoKeys(repos)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sortedRepoKeys() = %v, want %v", got, want)
	}
}

func TestSortedRepoKeysEmpty(t *testing.T) {
	got := sortedRepoKeys(map[string]config.Repo{})
	if len(got) != 0 {
		t.Errorf("sortedRepoKeys() = %v, want empty slice", got)
	}
}

func TestSortedGroupNames(t *testing.T) {
	groups := map[string]config.Group{
		"work":     {},
		"personal": {},
	}
	want := []string{"personal", "work"}

	got := sortedGroupNames(groups)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sortedGroupNames() = %v, want %v", got, want)
	}
}

func TestSortedGroupNamesEmpty(t *testing.T) {
	got := sortedGroupNames(map[string]config.Group{})
	if len(got) != 0 {
		t.Errorf("sortedGroupNames() = %v, want empty slice", got)
	}
}

func TestGroupLabel(t *testing.T) {
	m := &model{cfg: config.Config{
		Groups: map[string]config.Group{"work": {}},
	}}

	t.Run("with group filter", func(t *testing.T) {
		m.groupFilter = "personal"
		if got := m.groupLabel(); got != "@personal" {
			t.Errorf("groupLabel() = %q, want %q", got, "@personal")
		}
	})

	t.Run("no filter", func(t *testing.T) {
		m.groupFilter = ""
		if got := m.groupLabel(); got != "all" {
			t.Errorf("groupLabel() = %q, want %q", got, "all")
		}
	})
}

func TestActiveRepoOrder(t *testing.T) {
	m := &model{
		repoOrder: []string{"c", "a", "b"},
		cfg: config.Config{
			Groups: map[string]config.Group{"work": {Repos: []string{"b", "a"}}},
		},
	}

	t.Run("no group filter uses repoOrder", func(t *testing.T) {
		m.groupFilter = ""
		got := m.activeRepoOrder()
		want := []string{"a", "b", "c"}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("activeRepoOrder() = %v, want %v", got, want)
		}
	})

	t.Run("known group filter", func(t *testing.T) {
		m.groupFilter = "work"
		got := m.activeRepoOrder()
		want := []string{"a", "b"}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("activeRepoOrder() = %v, want %v", got, want)
		}
	})

	t.Run("unknown group filter cleared", func(t *testing.T) {
		m.groupFilter = "nonexistent"
		got := m.activeRepoOrder()
		want := []string{"a", "b", "c"}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("activeRepoOrder() = %v, want %v", got, want)
		}

		if m.groupFilter != "" {
			t.Errorf("groupFilter = %q, want empty", m.groupFilter)
		}
	})

	t.Run("empty group filter uses repoOrder", func(t *testing.T) {
		m.groupFilter = ""
		m.cfg.Groups = map[string]config.Group{}
		got := m.activeRepoOrder()
		want := []string{"a", "b", "c"}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("activeRepoOrder() = %v, want %v", got, want)
		}
	})
}

func TestAllReposFallback(t *testing.T) {
	m := &model{
		repoOrder: []string{"x", "y"},
	}

	got := m.allReposFallback()
	want := []string{"x", "y"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("allReposFallback() = %v, want %v", got, want)
	}
}

func TestSelectedNames(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "c": true},
	}

	got := m.selectedNames()
	want := []string{"a", "c"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("selectedNames() = %v, want %v", got, want)
	}
}

func TestSelectedNamesEmpty(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{},
	}

	got := m.selectedNames()
	if len(got) != 0 {
		t.Errorf("selectedNames() = %v, want empty", got)
	}
}

func TestSelectedNames_SingleModeReturnsCursorRepo(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "c": true},
		mode:      modeSingle,
	}
	m.cursor = 1

	got := m.selectedNames()
	want := []string{"c"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("selectedNames() = %v, want %v", got, want)
	}
}

func TestSelectedNames_SingleModeNoneWhenCursorOutOfRange(t *testing.T) {
	m := &model{
		repoOrder: []string{"a"},
		selected:  map[string]bool{"a": true},
		mode:      modeSingle,
	}
	m.cursor = 5

	got := m.selectedNames()
	if len(got) != 0 {
		t.Errorf("selectedNames() = %v, want empty", got)
	}
}

func TestSelectedNames_SingleModeReturnsEmptyWhenNoneSelected(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{},
		mode:      modeSingle,
	}
	m.cursor = 1

	got := m.selectedNames()
	if len(got) != 0 {
		t.Errorf("selectedNames() = %v, want empty when no repos selected", got)
	}
}

func TestAllSelected(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "b": true, "c": true},
	}

	if !m.allSelected() {
		t.Error("allSelected() = false, want true")
	}
}

func TestAllSelectedNotAll(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "c": true},
	}

	if m.allSelected() {
		t.Error("allSelected() = true, want false")
	}
}

func TestAllSelectedEmpty(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{},
	}

	if m.allSelected() {
		t.Error("allSelected() = true, want false")
	}
}

func TestTableRepos_SelectModeReturnsAll(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true},
		mode:      modeSelect,
	}

	got := m.tableRepos()
	want := []string{"a", "b", "c"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("tableRepos() = %v, want %v", got, want)
	}
}

func TestTableRepos_SelectedOnlyWhenNotInSelectMode(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"b": true},
		mode:      modeNormal,
	}

	got := m.tableRepos()
	want := []string{"b"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("tableRepos() = %v, want %v", got, want)
	}
}

func TestSelectedCount(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{"a": true, "c": true},
	}

	if got := m.selectedCount(); got != 2 {
		t.Errorf("selectedCount() = %d, want %d", got, 2)
	}
}

func TestSelectedCountEmpty(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b"},
		selected:  map[string]bool{},
	}

	if got := m.selectedCount(); got != 0 {
		t.Errorf("selectedCount() = %d, want %d", got, 0)
	}
}

func TestTotalCount(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
	}

	if got := m.totalCount(); got != 3 {
		t.Errorf("totalCount() = %d, want %d", got, 3)
	}
}

func TestTableRepos_EmptyWhenNoneSelected(t *testing.T) {
	m := &model{
		repoOrder: []string{"a", "b", "c"},
		selected:  map[string]bool{},
		mode:      modeNormal,
	}

	got := m.tableRepos()
	if len(got) != 0 {
		t.Errorf("tableRepos() = %v, want empty", got)
	}
}

func TestMakeSelectedMap(t *testing.T) {
	got := makeSelectedMap([]string{"b", "a"})
	if !got["a"] || !got["b"] {
		t.Error("makeSelectedMap should set all names to true")
	}

	if got["c"] {
		t.Error("makeSelectedMap should not include names not in input")
	}
}
