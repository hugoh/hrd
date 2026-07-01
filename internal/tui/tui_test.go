package tui

import (
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestoreSelectedReposStillExist(t *testing.T) {
	lastRepos := []string{"a", "b", "c"}
	repos := map[string]config.Repo{"a": {}, "b": {}, "c": {}}

	got := restoreSelected(lastRepos, repos)
	assert.Equal(t, map[string]bool{"a": true, "b": true, "c": true}, got)
}

func TestRestoreSelectedSomeRemoved(t *testing.T) {
	lastRepos := []string{"a", "b", "c"}
	repos := map[string]config.Repo{"a": {}, "c": {}}

	got := restoreSelected(lastRepos, repos)
	assert.Equal(t, map[string]bool{"a": true, "c": true}, got)
}

func TestRestoreSelectedAllRemoved(t *testing.T) {
	lastRepos := []string{"a", "b"}
	repos := map[string]config.Repo{"c": {}, "d": {}}

	got := restoreSelected(lastRepos, repos)
	assert.Equal(t, map[string]bool{}, got)
}

func TestRestoreSelectedEmptyLastRepos(t *testing.T) {
	repos := map[string]config.Repo{"a": {}, "b": {}}

	got := restoreSelected(nil, repos)
	assert.Equal(t, map[string]bool{}, got)
}

func TestRestoreSelectedEmptyRepos(t *testing.T) {
	lastRepos := []string{"a", "b"}

	got := restoreSelected(lastRepos, map[string]config.Repo{})
	assert.Equal(t, map[string]bool{}, got)
}

// testGroupConfig returns a fresh config.Config per call so tests can't
// leak mutations to each other via a shared package-level fixture.
func testGroupConfig() config.Config {
	return config.Config{
		Groups: map[string]config.Group{
			"work":     {Repos: []string{"a", "b"}},
			"personal": {Repos: []string{"c", "d"}},
		},
	}
}

func TestResolveGroupFilterOptGroupTakesPriority(t *testing.T) {
	got, err := resolveGroupFilter("personal", "work", testGroupConfig())
	require.NoError(t, err)
	assert.Equal(t, "personal", got)
}

func TestResolveGroupFilterFallsBackToLastGroup(t *testing.T) {
	got, err := resolveGroupFilter("", "personal", testGroupConfig())
	require.NoError(t, err)
	assert.Equal(t, "personal", got)
}

func TestResolveGroupFilterLastGroupNotInConfig(t *testing.T) {
	got, err := resolveGroupFilter("", "nonexistent", testGroupConfig())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestResolveGroupFilterAllEmpty(t *testing.T) {
	got, err := resolveGroupFilter("", "", config.Config{Groups: map[string]config.Group{}})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestResolveGroupFilterOptGroupUnknown(t *testing.T) {
	_, err := resolveGroupFilter("nonexistent", "", testGroupConfig())
	assert.Error(t, err)
}

func TestResolveGroupFilterOptGroupWithAtPrefix(t *testing.T) {
	got, err := resolveGroupFilter("@personal", "", testGroupConfig())
	require.NoError(t, err)
	assert.Equal(t, "personal", got)
}

func TestResolveGroupFilterAtPrefixUnknown(t *testing.T) {
	_, err := resolveGroupFilter("@nonexistent", "", testGroupConfig())
	assert.Error(t, err)
}
