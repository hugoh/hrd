package cmd

import (
	"bytes"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
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
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo-a": {Path: "/tmp/repo-a", Groups: []string{"work"}},
			"repo-b": {Path: "/tmp/repo-b"},
		},
	})

	buf := &bytes.Buffer{}
	app := &cli.Command{Writer: buf}
	completer := repoGroupCompleter(&cfgPath)
	completer(t.Context(), app)

	output := buf.String()
	assert.Contains(t, output, "repo-a")
	assert.Contains(t, output, "repo-b")
	assert.Contains(t, output, "@work")
	assert.GreaterOrEqual(t, len(output), len("repo-a\nrepo-b\n@work\n"))
}

func TestRepoGroupCompleterEmptyConfig(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	buf := &bytes.Buffer{}
	app := &cli.Command{Writer: buf}
	completer := repoGroupCompleter(&cfgPath)
	completer(t.Context(), app)

	assert.Empty(t, buf.String())
}

func TestRepoGroupCompleterBadPath(t *testing.T) {
	dir := t.TempDir()
	badPath := dir + "/" // path to a directory, not a file

	app := &cli.Command{Writer: &bytes.Buffer{}}
	reposOnlyCompleter(&badPath)(t.Context(), app)
	groupsOnlyCompleter(&badPath)(t.Context(), app)
	repoGroupCompleter(&badPath)(t.Context(), app)
}

func TestReposOnlyCompleter(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"myrepo": {Path: "/tmp/myrepo"},
		},
	})

	buf := &bytes.Buffer{}
	app := &cli.Command{Writer: buf}
	completer := reposOnlyCompleter(&cfgPath)
	completer(t.Context(), app)

	assert.Contains(t, buf.String(), "myrepo")
}

func TestGroupsOnlyCompleter(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Groups: []string{"mygroup"}},
		},
	})

	buf := &bytes.Buffer{}
	app := &cli.Command{Writer: buf}
	completer := groupsOnlyCompleter(&cfgPath)
	completer(t.Context(), app)

	assert.Contains(t, buf.String(), "@mygroup")
}
