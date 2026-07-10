package cmd

import (
	"strings"
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/discover/discovertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoAddRejectsReservedGroup(t *testing.T) {
	backend.ResetDetectCache()

	repoDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{})

	err := runHRD(t, cfgPath, []string{"repo", "add", "-g", "@@none", repoDir})
	require.Error(t, err)
}

func TestGroupAddRejectsReservedGroup(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{"r1": {Path: "/1"}},
	})

	err := runHRD(t, cfgPath, []string{"group", "add", "@@none", "r1"})
	require.Error(t, err)
}

// TestGroupAddStripsAtPrefix verifies that a single "@" is treated as
// input sugar (stripped) and never stored literally — this is what makes
// reserving the "@@" namespace for pseudo-groups safe.
func TestGroupAddStripsAtPrefix(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{"r1": {Path: "/1"}},
	})

	err := runHRD(t, cfgPath, []string{"group", "add", "@work", "r1"})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.NotContains(t, cfg.Groups, "@work")
	assert.Contains(t, cfg.Groups, "work")
}

func TestRepoScanAddRejectsReservedGroup(t *testing.T) {
	backend.ResetDetectCache()

	root := discovertest.Tree(t)
	cfgPath := setupTestConfig(t, config.Config{})

	err := runHRD(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd, "-g", "@@none", root})
	require.Error(t, err)
}

func TestRepoLsReservedNone(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"grouped":   {Path: "/g", Groups: []string{"work"}},
			"ungrouped": {Path: "/u"},
		},
	})

	out := runAppCapture(t, cfgPath, []string{"repo", "ls", "@@none"})
	assert.Contains(t, out, "ungrouped")
	assert.NotContains(t, out, "grouped\n")
}

func TestLsReservedNone(t *testing.T) {
	backend.ResetDetectCache()

	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"grouped":   {Path: setupFakeGitRepo(t), Groups: []string{"work"}},
			"ungrouped": {Path: setupFakeGitRepo(t)},
		},
	})

	out := runAppCapture(t, cfgPath, []string{"ls", "@@none", "--names"})
	assert.Equal(t, "ungrouped", strings.TrimSpace(out))
}

func TestLsAttentionFilter(t *testing.T) {
	cfgPath := cfgCleanAndDirtyRepo(t)

	out := runAppCapture(t, cfgPath, []string{"ls", "@@attention", "--names"})
	assert.Equal(t, "dirtyrepo", strings.TrimSpace(out))
}

// TestLsAttentionFilterCombinesWithExplicitFlag verifies that scoping to
// @@attention still applies when an explicit --dirty flag is also passed
// (redundant, but must not double-count or error).
func TestLsAttentionFilterCombinesWithExplicitFlag(t *testing.T) {
	cfgPath := cfgCleanAndDirtyRepo(t)

	out := runAppCapture(t, cfgPath, []string{"ls", "@@attention", "--dirty", "--names"})
	assert.Equal(t, "dirtyrepo", strings.TrimSpace(out))
}

func TestShellAttentionFilter(t *testing.T) {
	cfgPath := cfgCleanAndDirtyRepo(t)

	out := runAppCapture(t, cfgPath, []string{"shell", "@@attention", "--", "echo ran"})
	assert.Contains(t, out, "dirtyrepo")
	assert.NotContains(t, out, "cleanrepo")
}

// TestGroupLsAttentionSingleName is a regression test: Config.GroupRepos
// intentionally returns every repo for @@attention (correct for CLI-dispatch
// scope resolution), but "hrd group ls @@attention" printed that unfiltered
// result directly — showing all repos, not just the ones needing attention.
func TestGroupLsAttentionSingleName(t *testing.T) {
	cfgPath := cfgCleanAndDirtyRepo(t)

	out := runAppCapture(t, cfgPath, []string{"group", "ls", "@@attention"})
	assert.Equal(t, "dirtyrepo", strings.TrimSpace(out))
}

func TestGroupLsWithoutLiveShowsAttentionHint(t *testing.T) {
	cfgPath := cfgCleanAndDirtyRepo(t)

	out := runAppCapture(t, cfgPath, []string{"group", "ls"})
	assert.Contains(t, out, "@@attention  (pass --live to compute)")
}

func TestGroupLsLiveComputesAttentionMembership(t *testing.T) {
	cfgPath := cfgCleanAndDirtyRepo(t)

	out := runAppCapture(t, cfgPath, []string{"group", "ls", "--live"})
	assert.Contains(t, out, "@@attention")
	assert.Contains(t, out, "dirtyrepo")

	lines := strings.Split(out, "\n")

	var attentionLine string

	for i, line := range lines {
		if line == "@@attention" && i+1 < len(lines) {
			attentionLine = lines[i+1]
		}
	}

	assert.Equal(
		t,
		"  dirtyrepo",
		attentionLine,
		"only dirtyrepo should be listed under @@attention",
	)
}

func TestGroupListReservedFlag(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	out := runAppCapture(t, cfgPath, []string{"group", "--list-reserved"})
	assert.Contains(t, out, "@@none")
	assert.Contains(t, out, "@@attention")
	assert.Contains(t, out, "repos with no group")
}

// TestGroupBareShowsHelp verifies "hrd group" (no subcommand, no flag)
// doesn't error — it falls back to cobra's default help behavior. Cobra
// writes help to the command's Out writer, which the test harness redirects
// to an internal buffer (see bufferWriters), not stdout, so this only
// asserts on the error, not captured content.
func TestGroupBareShowsHelp(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	app := newTestApp()

	err := RunApp(t.Context(), app, []string{"hrd", "--config", cfgPath, "group"})
	require.NoError(t, err)
}

func TestRepoRootAddRejectsReservedGroup(t *testing.T) {
	dir := t.TempDir()
	cfgPath := setupTestConfig(t, config.Config{})

	err := runHRD(t, cfgPath, []string{"repo", "root", "add", dir, "-g", "@@none"})
	require.Error(t, err)
}
