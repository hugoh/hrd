package cmd

import (
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
		Aliases: aliases,
	})
}

func TestAliasInHelp(t *testing.T) {
	cfgPath := aliasTestConfig(t, map[string]string{"sync": "pull --rebase"})

	app, head := NewAppForArgs([]string{"hrd", "-c", cfgPath, "sync", "--help"})
	bufferWriters(app)

	require.NoError(t, app.Run(t.Context(), head))
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

	err := runApp(t, cfgPath, []string{"branches"})
	require.NoError(t, err)
}

func TestAliasShadowingBuiltinIgnored(t *testing.T) {
	cfgPath := aliasTestConfig(t, map[string]string{"status": "!echo shadowed"})

	app, _ := NewAppForArgs([]string{"hrd", "-c", cfgPath})

	for _, c := range app.Commands {
		if c.Name == "status" {
			assert.NotContains(t, c.Usage, "alias", "built-in status must not be replaced")
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
		Aliases: map[string]string{"hello": "!echo hi"},
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

	err := runApp(t, cfgPath, []string{"last", "-i"})
	require.NoError(t, err)
}
