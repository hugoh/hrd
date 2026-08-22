package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aliasTestConfig(t *testing.T, aliases map[string]string) string {
	t.Helper()

	return setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: setupRealGitRepo(t, false)},
		},
		Aliases: aliasSpecs(aliases),
	})
}

// aliasSpecs wraps plain command strings into AliasSpecs, for tests that
// don't care about per-backend variants.
func aliasSpecs(aliases map[string]string) map[string]config.AliasSpec {
	specs := make(map[string]config.AliasSpec, len(aliases))
	for name, cmd := range aliases {
		specs[name] = config.AliasSpec{Command: cmd}
	}

	return specs
}

func TestAliasInHelp(t *testing.T) {
	cfgPath := aliasTestConfig(t, map[string]string{"sync": "pull --rebase"})

	args := []string{"hrd", "-c", cfgPath, "sync", "--help"}
	app := NewAppForArgs(args)
	bufferWriters(app)

	require.NoError(t, RunApp(t.Context(), app, args))
}

func TestAliasShell(t *testing.T) {
	backend.ResetDetectCache()

	cfgPath := aliasTestConfig(t, map[string]string{"hello": "!echo hi"})

	out := runAppCapture(t, cfgPath, []string{"hello"})
	assert.Contains(t, out, "hi")
}

func TestAliasShellExtraArgs(t *testing.T) {
	backend.ResetDetectCache()

	cfgPath := aliasTestConfig(t, map[string]string{"say": "sh echo"})

	out := runAppCapture(t, cfgPath, []string{"say", "--", "extra words"})
	assert.Contains(t, out, "extra words")
}

func TestAliasBareSubcmd(t *testing.T) {
	backend.ResetDetectCache()

	cfgPath := aliasTestConfig(t, map[string]string{"last": "log -1 --oneline"})

	out := runAppCapture(t, cfgPath, []string{"last"})
	assert.Contains(t, out, "initial")
}

func TestAliasBackendPrefix(t *testing.T) {
	backend.ResetDetectCache()

	cfgPath := aliasTestConfig(t, map[string]string{"branches": "git branch --list"})

	err := runHRD(t, cfgPath, []string{"branches"})
	require.NoError(t, err)
}

func TestAliasShadowingBuiltinIgnored(t *testing.T) {
	cfgPath := aliasTestConfig(t, map[string]string{"status": "!echo shadowed"})

	app := NewAppForArgs([]string{"hrd", "-c", cfgPath})

	for _, c := range app.Commands() {
		if c.Name() == "status" {
			assert.NotContains(t, c.Short, "alias", "built-in status must not be replaced")
		}
	}
}

func TestAliasScopeAndFilter(t *testing.T) {
	backend.ResetDetectCache()

	dirty := setupRealGitRepo(t, true)
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"cleanrepo": {Path: setupRealGitRepo(t, false)},
			"dirtyrepo": {Path: dirty},
		},
		Aliases: aliasSpecs(map[string]string{"hello": "!echo hi"}),
	})

	out := runAppCapture(t, cfgPath, []string{"hello", "--dirty"})
	assert.Contains(t, out, "dirtyrepo")
	assert.NotContains(t, out, "cleanrepo")
}

func TestConfigPathFromArgs(t *testing.T) {
	def := config.DefaultPath()

	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{name: "no flag", args: []string{"hrd", "ls"}, want: def},
		{name: "short flag", args: []string{"hrd", "-c", "/tmp/x.toml", "ls"}, want: "/tmp/x.toml"},
		{name: "long flag", args: []string{"hrd", "--config", "/tmp/x.toml"}, want: "/tmp/x.toml"},
		{name: "long equals", args: []string{"hrd", "--config=/tmp/x.toml"}, want: "/tmp/x.toml"},
		{name: "short equals", args: []string{"hrd", "-c=/tmp/x.toml"}, want: "/tmp/x.toml"},
		{name: "dangling flag", args: []string{"hrd", "-c"}, want: def},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, configPathFromArgs(tt.args))
		})
	}
}

func TestAliasInteractiveBare(t *testing.T) {
	backend.ResetDetectCache()

	cfgPath := aliasTestConfig(t, map[string]string{"last": "log -1 --oneline"})

	err := runHRD(t, cfgPath, []string{"last", "-i"})
	require.NoError(t, err)
}

func TestAliasPerBackend_DispatchesMatchingVariant(t *testing.T) {
	backend.ResetDetectCache()

	gitDir := setupFakeGitRepo(t)
	jjDir := setupFakeJJRepo(t)

	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"gitrepo": {Path: gitDir},
			"jjrepo":  {Path: jjDir},
		},
		Aliases: map[string]config.AliasSpec{
			"whoami": {Backends: map[string]string{
				"git": "!echo it-was-git",
				"jj":  "!echo it-was-jj",
			}},
		},
	})

	out := runAppCapture(t, cfgPath, []string{"whoami"})
	assert.Contains(t, out, "it-was-git")
	assert.Contains(t, out, "it-was-jj")
}

func TestAliasPerBackend_SkipsReposWithNoVariant(t *testing.T) {
	backend.ResetDetectCache()

	gitDir := setupFakeGitRepo(t)
	jjDir := setupFakeJJRepo(t)

	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"gitrepo": {Path: gitDir},
			"jjrepo":  {Path: jjDir},
		},
		Aliases: map[string]config.AliasSpec{
			"gitonly": {Backends: map[string]string{
				"git": "!echo only-git",
			}},
		},
	})

	out := runAppCapture(t, cfgPath, []string{"gitonly"})
	assert.Contains(t, out, "only-git")
}

// TestAliasPerBackend_ColocatedRepoUsesActiveBackendOnly guards against a
// colocated jj/git repo (jj repos are git-colocated, so both markers are
// present on disk) running both backends' variants of a per-backend alias.
// Only the repo's active backend (jj wins over git for colocated repos, see
// backend.DetectAll) should run.
func TestAliasPerBackend_ColocatedRepoUsesActiveBackendOnly(t *testing.T) {
	backend.ResetDetectCache()

	bothDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bothDir, ".git"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(bothDir, ".jj"), 0o750))

	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"bothrepo": {Path: bothDir},
		},
		Aliases: map[string]config.AliasSpec{
			"whoami": {Backends: map[string]string{
				"git": "!echo it-was-git",
				"jj":  "!echo it-was-jj",
			}},
		},
	})

	out := runAppCapture(t, cfgPath, []string{"whoami"})
	assert.Contains(t, out, "it-was-jj")
	assert.NotContains(t, out, "it-was-git")
}

func TestAliasBuiltinUp_PresentWithNoUserConfig(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"repo1": {Path: setupRealGitRepo(t, false)},
		},
	})

	args := []string{"hrd", "-c", cfgPath, "up", "--help"}
	app := NewAppForArgs(args)
	bufferWriters(app)

	require.NoError(t, RunApp(t.Context(), app, args))
}

func TestAliasBuiltinUp_UserOverride(t *testing.T) {
	cfgPath := aliasTestConfig(t, map[string]string{"up": "!echo custom-up"})

	out := runAppCapture(t, cfgPath, []string{"up"})
	assert.Contains(t, out, "custom-up")
}
