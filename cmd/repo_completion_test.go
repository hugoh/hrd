package cmd

import (
	"bytes"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runComplete invokes cobra's `__complete` for the given args and returns
// the combined stdout/stderr, which includes the offered completions
// followed by a ":<directive>" line and a "Completion ended with
// directive: ..." trailer.
func runComplete(t *testing.T, cfgPath string, args []string) string {
	t.Helper()

	app := NewApp()

	var out bytes.Buffer
	app.SetOut(&out)
	app.SetErr(&out)
	app.SetArgs(append([]string{"--config", cfgPath, "__complete"}, args...))

	require.NoError(t, app.Execute())

	return out.String()
}

func TestRepoAddCompletesDirsOnly(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	got := runComplete(t, cfgPath, []string{"repo", "add", ""})

	assert.Contains(t, got, "ShellCompDirectiveFilterDirs")
}

func TestRepoScanAddCompletesDirsOnly(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	got := runComplete(t, cfgPath, []string{"repo", "scan", "add", ""})

	assert.Contains(t, got, "ShellCompDirectiveFilterDirs")
}

func TestRepoScanLsCompletesDirsOnly(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	got := runComplete(t, cfgPath, []string{"repo", "scan", "ls", ""})

	assert.Contains(t, got, "ShellCompDirectiveFilterDirs")
}

func TestRepoRootAddCompletesDirsOnly(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	got := runComplete(t, cfgPath, []string{"repo", "root", "add", ""})

	assert.Contains(t, got, "ShellCompDirectiveFilterDirs")
}

func TestRepoRootRmCompletesRootNames(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Roots: map[string]config.Root{
			"myroot": {Path: "/tmp/myroot"},
		},
	})

	got := runComplete(t, cfgPath, []string{"repo", "root", "rm", ""})

	assert.Contains(t, got, "myroot")
}

func TestTUICompletesReposAndGroups(t *testing.T) {
	cfgPath := twoRepoGroupConfig(t)

	got := runComplete(t, cfgPath, []string{"tui", ""})

	assert.Contains(t, got, "repo-a")
	assert.Contains(t, got, "repo-b")
	assert.Contains(t, got, "@work")
}
