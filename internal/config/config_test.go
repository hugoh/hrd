package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepo_ActiveBackend(t *testing.T) {
	tests := []struct {
		name     string
		repo     Repo
		expected string
	}{
		{"has backends", Repo{Backends: []string{"git", "jj"}}, "git"},
		{"empty backends", Repo{Backends: []string{}}, ""},
		{"nil backends", Repo{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.repo.ActiveBackend())
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	assert.Equal(t, defaultConcurrency, cfg.Settings.Concurrency)
	assert.NotNil(t, cfg.Repos)
	assert.NotNil(t, cfg.Groups)
}

func TestDefaultPath(t *testing.T) {
	path := DefaultPath()
	assert.Contains(t, path, "hrd")
	assert.Contains(t, path, "config.toml")

	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	customPath := DefaultPath()
	assert.Equal(t, "/custom/config/hrd/config.toml", customPath)
}

func TestLoad_NoFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	require.NoError(t, err)
	assert.Equal(t, defaultConcurrency, cfg.Settings.Concurrency)
	assert.NotNil(t, cfg.Repos)
	assert.NotNil(t, cfg.Groups)
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `[repos."myproject"]
path = "/home/user/myproject"
backends = ["git"]

[groups.work]
repos = ["myproject"]

[context]
current = "work"

[settings]
concurrency = 4
`
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, 4, cfg.Settings.Concurrency)
	assert.Equal(t, "work", cfg.Context.Current)
	assert.Contains(t, cfg.Repos, "myproject")
	assert.Equal(t, "/home/user/myproject", cfg.Repos["myproject"].Path)
	assert.Contains(t, cfg.Groups, "work")
}

func TestLoad_InvalidToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	err := os.WriteFile(path, []byte("invalid toml {{"), 0o644)
	require.NoError(t, err)

	_, err = Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing")
}

func TestLoad_NilMaps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	err := os.WriteFile(path, []byte("[settings]\nconcurrency = 2\n"), 0o644)
	require.NoError(t, err)

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.NotNil(t, cfg.Repos)
	assert.NotNil(t, cfg.Groups)
}

func TestLoad_ZeroConcurrency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	err := os.WriteFile(path, []byte("[settings]\nconcurrency = 0\n"), 0o644)
	require.NoError(t, err)

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, defaultConcurrency, cfg.Settings.Concurrency)
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := Config{
		Repos: map[string]Repo{
			"myproject": {Path: "/home/user/myproject", Backends: []string{"git"}},
		},
		Groups: map[string]Group{
			"work": {Repos: []string{"myproject"}},
		},
		Settings: Settings{
			Concurrency: 16,
		},
	}

	err := Save(path, cfg)
	require.NoError(t, err)

	_, err = os.Stat(path)
	require.NoError(t, err)

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, cfg.Repos, loaded.Repos)
	assert.Equal(t, cfg.Groups, loaded.Groups)
	assert.Equal(t, 16, loaded.Settings.Concurrency)
}

func TestResolveScope_NoNamesNoContext(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"zeta":  {Path: "/z"},
			"alpha": {Path: "/a"},
			"beta":  {Path: "/b"},
		},
	}

	names, err := cfg.ResolveScope(nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "zeta"}, names)
}

func TestResolveScope_WithContext(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"repo1": {Path: "/1"},
			"repo2": {Path: "/2"},
			"repo3": {Path: "/3"},
		},
		Groups: map[string]Group{
			"work": {Repos: []string{"repo1", "repo2"}},
		},
		Context: Context{Current: "work"},
	}

	names, err := cfg.ResolveScope(nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"repo1", "repo2"}, names)
}

func TestResolveScope_ContextNotFound(t *testing.T) {
	cfg := &Config{
		Repos:   map[string]Repo{"r1": {Path: "/1"}},
		Groups:  map[string]Group{},
		Context: Context{Current: "nonexistent"},
	}

	_, err := cfg.ResolveScope(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveScope_SingleNameIsGroup(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Path: "/1"},
			"r2": {Path: "/2"},
		},
		Groups: map[string]Group{
			"work": {Repos: []string{"r1"}},
		},
	}

	names, err := cfg.ResolveScope([]string{"work"})
	require.NoError(t, err)
	assert.Equal(t, []string{"r1"}, names)
}

func TestResolveScope_ExplicitRepos(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Path: "/1"},
			"r2": {Path: "/2"},
			"r3": {Path: "/3"},
		},
	}

	names, err := cfg.ResolveScope([]string{"r1", "r3"})
	require.NoError(t, err)
	assert.Equal(t, []string{"r1", "r3"}, names)
}

func TestResolveScope_UnknownRepo(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{"r1": {Path: "/1"}},
	}

	_, err := cfg.ResolveScope([]string{"r1", "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repo")
}

func TestAddRepo(t *testing.T) {
	cfg := &Config{Repos: map[string]Repo{}}
	cfg.AddRepo("myproject", Repo{Path: "/p", Backends: []string{"git"}})
	assert.Contains(t, cfg.Repos, "myproject")
	assert.Equal(t, "/p", cfg.Repos["myproject"].Path)
}

func TestRemoveRepo(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Path: "/1"},
			"r2": {Path: "/2"},
		},
		Groups: map[string]Group{
			"work": {Repos: []string{"r1", "r2"}},
			"all":  {Repos: []string{"r1", "r2"}},
		},
	}

	cfg.RemoveRepo("r1")
	assert.NotContains(t, cfg.Repos, "r1")
	assert.Contains(t, cfg.Repos, "r2")
	assert.Equal(t, []string{"r2"}, cfg.Groups["work"].Repos)
	assert.Equal(t, []string{"r2"}, cfg.Groups["all"].Repos)
}

func TestAddGroup(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Path: "/1"},
			"r2": {Path: "/2"},
		},
		Groups: map[string]Group{},
	}

	err := cfg.AddGroup("work", []string{"r1", "r2"})
	require.NoError(t, err)
	assert.Contains(t, cfg.Groups, "work")

	err = cfg.AddGroup("work", []string{"r1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"r1"}, cfg.Groups["work"].Repos)
}

func TestAddGroup_UnknownRepo(t *testing.T) {
	cfg := &Config{Repos: map[string]Repo{"r1": {Path: "/1"}}}

	err := cfg.AddGroup("work", []string{"r1", "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repo")
}
