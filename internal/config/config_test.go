package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	assert.Equal(t, defaultConcurrency, cfg.Settings.Concurrency)
	assert.NotNil(t, cfg.Repos)
	assert.NotNil(t, cfg.Groups)
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	path := DefaultPath()
	assert.Contains(t, path, "hrd")
	assert.Contains(t, path, "config.toml")

	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	customPath := DefaultPath()
	assert.Equal(t, "/custom/config/hrd/config.toml", customPath)
}

func TestDefaultPathAlreadySet(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg/custom")

	path := DefaultPath()
	assert.Equal(t, "/xdg/custom/hrd/config.toml", path)
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
groups = ["work"]

[settings]
concurrency = 4
`
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, 4, cfg.Settings.Concurrency)
	assert.Contains(t, cfg.Repos, "myproject")
	assert.Equal(t, "/home/user/myproject", cfg.Repos["myproject"].Path)
	assert.Contains(t, cfg.Groups, "work")
	assert.Equal(t, []string{"myproject"}, cfg.Groups["work"].Repos)
}

func TestLoad_InvalidToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	err := os.WriteFile(path, []byte("invalid toml {{"), 0o644)
	require.NoError(t, err)

	_, err = Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding config")
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
			"myproject": {Path: "/home/user/myproject", Groups: []string{"work"}},
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
	assert.Equal(t, 16, loaded.Settings.Concurrency)
	assert.Contains(t, loaded.Groups, "work")
	assert.Equal(t, []string{"myproject"}, loaded.Groups["work"].Repos)
}

func TestResolveScope_NoNamesNoContext(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"zeta":  {Path: "/z"},
			"alpha": {Path: "/a"},
			"beta":  {Path: "/b"},
		},
	}
	cfg.rebuildGroupsCache()

	names, err := cfg.ResolveScope(nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "zeta"}, names)
}

func TestResolveScope_SingleNameIsGroup(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Path: "/1", Groups: []string{"work"}},
			"r2": {Path: "/2"},
		},
	}
	cfg.rebuildGroupsCache()

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
	cfg.rebuildGroupsCache()

	names, err := cfg.ResolveScope([]string{"r1", "r3"})
	require.NoError(t, err)
	assert.Equal(t, []string{"r1", "r3"}, names)
}

func TestResolveScope_UnknownRepo(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{"r1": {Path: "/1"}},
	}
	cfg.rebuildGroupsCache()

	_, err := cfg.ResolveScope([]string{"r1", "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown repo")
}

func TestAddRepo(t *testing.T) {
	cfg := &Config{Repos: map[string]Repo{}}
	cfg.AddRepo("myproject", Repo{Path: "/p"})
	assert.Contains(t, cfg.Repos, "myproject")
	assert.Equal(t, "/p", cfg.Repos["myproject"].Path)
}

func TestRemoveRepo(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Path: "/1", Groups: []string{"work", "all"}},
			"r2": {Path: "/2", Groups: []string{"work", "all"}},
		},
	}
	cfg.rebuildGroupsCache()

	cfg.RemoveRepo("r1")
	assert.NotContains(t, cfg.Repos, "r1")
	assert.Contains(t, cfg.Repos, "r2")
	assert.Equal(t, []string{"r2"}, cfg.Groups["work"].Repos)
	assert.Equal(t, []string{"r2"}, cfg.Groups["all"].Repos)
}

func TestAddRepoToGroupNew(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Path: "/1"},
			"r2": {Path: "/2", Groups: []string{"work"}},
		},
	}
	cfg.rebuildGroupsCache()

	cfg.AddRepoToGroup("r1", "work")

	assert.Equal(t, []string{"work"}, cfg.Repos["r1"].Groups)
	require.Contains(t, cfg.Groups, "work")
	assert.Equal(t, []string{"r1", "r2"}, cfg.Groups["work"].Repos)
}

func TestAddRepoToGroupExisting(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Groups: []string{"work"}},
		},
	}
	cfg.rebuildGroupsCache()

	cfg.AddRepoToGroup("r1", "work")

	assert.Equal(t, []string{"work"}, cfg.Repos["r1"].Groups)
	assert.Len(t, cfg.Groups["work"].Repos, 1)
}

func TestAddRepoToGroupNewGroup(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{"r1": {}},
	}
	cfg.rebuildGroupsCache()

	cfg.AddRepoToGroup("r1", "newgroup")

	assert.Equal(t, []string{"newgroup"}, cfg.Repos["r1"].Groups)
	require.Contains(t, cfg.Groups, "newgroup")
	assert.Equal(t, []string{"r1"}, cfg.Groups["newgroup"].Repos)
}

func TestRemoveRepoFromGroup(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Groups: []string{"work", "oss"}},
			"r2": {Groups: []string{"work"}},
		},
	}
	cfg.rebuildGroupsCache()

	cfg.RemoveRepoFromGroup("r1", "work")

	assert.Equal(t, []string{"oss"}, cfg.Repos["r1"].Groups)
	assert.Equal(t, []string{"r2"}, cfg.Groups["work"].Repos)
	require.Contains(t, cfg.Groups, "oss")
	assert.Equal(t, []string{"r1"}, cfg.Groups["oss"].Repos)
}

func TestRemoveRepoFromGroupRemovesEmptyGroup(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Groups: []string{"sole"}},
		},
	}
	cfg.rebuildGroupsCache()

	cfg.RemoveRepoFromGroup("r1", "sole")

	assert.Empty(t, cfg.Repos["r1"].Groups)
	assert.NotContains(t, cfg.Groups, "sole")
}

func TestRemoveRepoFromGroupNotExists(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{"r1": {Groups: []string{"work"}}},
	}
	cfg.rebuildGroupsCache()

	cfg.RemoveRepoFromGroup("r1", "nonexistent")

	assert.Equal(t, []string{"work"}, cfg.Repos["r1"].Groups)
	assert.Contains(t, cfg.Groups, "work")
}

func TestStripGroupPrefix(t *testing.T) {
	assert.Equal(t, "work", stripGroupPrefix("work"))
	assert.Equal(t, "work", stripGroupPrefix("@work"))
	assert.Empty(t, stripGroupPrefix(""))
	assert.Equal(t, "@work", stripGroupPrefix("@@work"))
}

func TestLoadFileReadError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "config.toml")

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, defaultConcurrency, cfg.Settings.Concurrency)
}

func TestSaveMkdirError(t *testing.T) {
	err := Save("/nonexistent-dir-12345/config.toml", Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating config dir")
}

func TestRebuildsGroupsCache(t *testing.T) {
	cfg := &Config{
		Repos: map[string]Repo{
			"r1": {Groups: []string{"work", "oss"}},
			"r2": {Groups: []string{"work"}},
			"r3": {},
		},
	}

	cfg.rebuildGroupsCache()

	require.Contains(t, cfg.Groups, "work")
	assert.Equal(t, []string{"r1", "r2"}, cfg.Groups["work"].Repos)
	require.Contains(t, cfg.Groups, "oss")
	assert.Equal(t, []string{"r1"}, cfg.Groups["oss"].Repos)
	assert.NotContains(t, cfg.Groups, "r3")
}

func TestLoad_DerivesGroupsFromRepos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `[repos.a]
path = "/a"
groups = ["work"]

[repos.b]
path = "/b"
groups = ["work", "personal"]

[repos.c]
path = "/c"
`
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)

	cfg, err := Load(path)
	require.NoError(t, err)

	require.Contains(t, cfg.Groups, "work")
	assert.Equal(t, []string{"a", "b"}, cfg.Groups["work"].Repos)
	require.Contains(t, cfg.Groups, "personal")
	assert.Equal(t, []string{"b"}, cfg.Groups["personal"].Repos)
	assert.NotContains(t, cfg.Groups, "c")
}
