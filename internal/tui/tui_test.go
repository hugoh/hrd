package tui

import (
	"reflect"
	"testing"

	"github.com/hugoh/hrd/internal/config"
)

func TestRestoreSelectedReposStillExist(t *testing.T) {
	lastRepos := []string{"a", "b", "c"}
	repos := map[string]config.Repo{"a": {}, "b": {}, "c": {}}
	want := map[string]bool{"a": true, "b": true, "c": true}

	got := restoreSelected(lastRepos, repos)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("restoreSelected() = %v, want %v", got, want)
	}
}

func TestRestoreSelectedSomeRemoved(t *testing.T) {
	lastRepos := []string{"a", "b", "c"}
	repos := map[string]config.Repo{"a": {}, "c": {}}
	want := map[string]bool{"a": true, "c": true}

	got := restoreSelected(lastRepos, repos)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("restoreSelected() = %v, want %v", got, want)
	}
}

func TestRestoreSelectedAllRemoved(t *testing.T) {
	lastRepos := []string{"a", "b"}
	repos := map[string]config.Repo{"c": {}, "d": {}}
	want := map[string]bool{}

	got := restoreSelected(lastRepos, repos)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("restoreSelected() = %v, want %v", got, want)
	}
}

func TestRestoreSelectedEmptyLastRepos(t *testing.T) {
	repos := map[string]config.Repo{"a": {}, "b": {}}
	want := map[string]bool{}

	got := restoreSelected(nil, repos)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("restoreSelected() = %v, want %v", got, want)
	}
}

func TestRestoreSelectedEmptyRepos(t *testing.T) {
	lastRepos := []string{"a", "b"}
	want := map[string]bool{}

	got := restoreSelected(lastRepos, map[string]config.Repo{})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("restoreSelected() = %v, want %v", got, want)
	}
}

//nolint:gochecknoglobals // shared test config
var cfg = config.Config{
	Groups: map[string]config.Group{
		"work":     {Repos: []string{"a", "b"}},
		"personal": {Repos: []string{"c", "d"}},
	},
}

func TestResolveGroupFilterOptGroupTakesPriority(t *testing.T) {
	got := resolveGroupFilter("personal", "work", cfg)
	if got != "personal" {
		t.Errorf("resolveGroupFilter() = %q, want %q", got, "personal")
	}
}

func TestResolveGroupFilterFallsBackToLastGroup(t *testing.T) {
	got := resolveGroupFilter("", "personal", cfg)
	if got != "personal" {
		t.Errorf("resolveGroupFilter() = %q, want %q", got, "personal")
	}
}

func TestResolveGroupFilterLastGroupNotInConfig(t *testing.T) {
	got := resolveGroupFilter("", "nonexistent", cfg)
	if got != "" {
		t.Errorf("resolveGroupFilter() = %q, want %q", got, "")
	}
}

func TestResolveGroupFilterAllEmpty(t *testing.T) {
	got := resolveGroupFilter("", "", config.Config{Groups: map[string]config.Group{}})
	if got != "" {
		t.Errorf("resolveGroupFilter() = %q, want %q", got, "")
	}
}
