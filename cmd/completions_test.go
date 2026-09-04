package cmd

import (
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/discover/discovertest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestCompleteRepos(t *testing.T) {
	cfg := &config.Config{
		Repos: map[string]config.Repo{
			"z-repo": {Path: "/tmp/z"},
			"a-repo": {Path: "/tmp/a"},
			"m-repo": {Path: "/tmp/m"},
		},
	}
	got := completeRepos(cfg)
	assert.Equal(t, []string{"a-repo", "m-repo", "z-repo"}, got)
}

func TestCompleteReposEmpty(t *testing.T) {
	cfg := &config.Config{Repos: map[string]config.Repo{}}
	assert.Empty(t, completeRepos(cfg))
}

func TestCompleteReposNil(t *testing.T) {
	cfg := &config.Config{}
	assert.Empty(t, completeRepos(cfg))
}

func TestCompleteGroups(t *testing.T) {
	cfg := &config.Config{
		Repos: map[string]config.Repo{
			"a": {Groups: []string{"work", "personal"}},
		},
		Groups: map[string]config.Group{
			"personal": {Repos: []string{"a"}},
			"work":     {Repos: []string{"a"}},
		},
	}
	got := completeGroups(cfg)
	assert.Equal(t, []string{"@personal", "@work"}, got)
}

func TestCompleteGroupsEmpty(t *testing.T) {
	cfg := &config.Config{Repos: map[string]config.Repo{}, Groups: map[string]config.Group{}}
	assert.Empty(t, completeGroups(cfg))
}

func TestCompleteGroupsNil(t *testing.T) {
	cfg := &config.Config{Groups: map[string]config.Group{}}
	assert.Empty(t, completeGroups(cfg))
}

func TestRepoGroupCompleterWritesReposAndGroups(t *testing.T) {
	cfgPath := twoRepoGroupConfig(t)

	completer := repoGroupCompleter(&cfgPath)
	got, _ := completer(nil, nil, "")

	assert.Contains(t, got, "repo-a")
	assert.Contains(t, got, "repo-b")
	assert.Contains(t, got, "@work")
	assert.GreaterOrEqual(t, len(got), 3)
}

func TestRepoGroupCompleterEmptyConfig(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	completer := repoGroupCompleter(&cfgPath)
	got, _ := completer(nil, nil, "")

	assert.Empty(t, got)
}

func TestRepoGroupCompleterBadPath(t *testing.T) {
	dir := t.TempDir()
	badPath := dir + "/" // path to a directory, not a file

	_, _ = reposOnlyCompleter(&badPath)(nil, nil, "")
	_, _ = groupsOnlyCompleter(&badPath)(nil, nil, "")
	_, _ = repoGroupCompleter(&badPath)(nil, nil, "")
}

func TestReposOnlyCompleter(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"myrepo": {Path: "/tmp/myrepo"},
		},
	})

	completer := reposOnlyCompleter(&cfgPath)
	got, _ := completer(nil, nil, "")

	assert.Contains(t, got, "myrepo")
}

func TestReposOrDirsCompleter(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"myrepo": {Path: "/tmp/myrepo"},
		},
	})

	got, directive := reposOrDirsCompleter(&cfgPath)(nil, nil, "")

	assert.Contains(t, got, "myrepo")
	assert.Equal(t, cobra.ShellCompDirectiveFilterDirs, directive)
}

func TestReposOrDirsCompleterBadPath(t *testing.T) {
	dir := t.TempDir()
	badPath := dir + "/"

	got, directive := reposOrDirsCompleter(&badPath)(nil, nil, "")

	assert.Nil(t, got)
	assert.Equal(t, cobra.ShellCompDirectiveFilterDirs, directive)
}

func TestRepoGroupCompleterIncludesRootDiscoveredRepos(t *testing.T) {
	root := discovertest.Tree(t)
	cfgPath := setupTestConfig(t, config.Config{
		Roots: map[string]config.Root{
			"work": {Path: root, Depth: 3},
		},
	})

	completer := repoGroupCompleter(&cfgPath)
	got, _ := completer(nil, nil, "")

	assert.Contains(t, got, "app")
}

func TestCompleteRoots(t *testing.T) {
	cfg := &config.Config{
		Roots: map[string]config.Root{
			"z-root": {Path: "/tmp/z"},
			"a-root": {Path: "/tmp/a"},
			"m-root": {Path: "/tmp/m"},
		},
	}
	got := completeRoots(cfg)
	assert.Equal(t, []string{"a-root", "m-root", "z-root"}, got)
}

func TestCompleteRootsEmpty(t *testing.T) {
	cfg := &config.Config{Roots: map[string]config.Root{}}
	assert.Empty(t, completeRoots(cfg))
}

func TestCompleteRootsNil(t *testing.T) {
	cfg := &config.Config{}
	assert.Empty(t, completeRoots(cfg))
}

func TestRootsOnlyCompleter(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Roots: map[string]config.Root{
			"myroot": {Path: "/tmp/myroot"},
		},
	})

	completer := rootsOnlyCompleter(&cfgPath)
	got, directive := completer(nil, nil, "")

	assert.Contains(t, got, "myroot")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
}

func TestDirsOnlyCompleter(t *testing.T) {
	got, directive := dirsOnlyCompleter(nil, nil, "")

	assert.Nil(t, got)
	assert.Equal(t, cobra.ShellCompDirectiveFilterDirs, directive)
}

func TestGroupsOnlyCompleter(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Groups: []string{"mygroup"}},
		},
	})

	completer := groupsOnlyCompleter(&cfgPath)
	got, _ := completer(nil, nil, "")

	assert.Contains(t, got, "@mygroup")
}
