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

func TestRepoGroupRejectsReservedGroup(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{"r1": {Path: "/1"}},
	})

	err := runHRD(t, cfgPath, []string{"repo", "group", "r1", "@@none"})
	require.Error(t, err)
}

// TestRepoGroupStripsAtPrefix verifies that a single "@" is treated as
// input sugar (stripped) and never stored literally — this is what makes
// reserving the "@@" namespace for pseudo-groups safe.
func TestRepoGroupStripsAtPrefix(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{"r1": {Path: "/1"}},
	})

	err := runHRD(t, cfgPath, []string{"repo", "group", "r1", "@work"})
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

func TestRepoScanListRejectsReservedGroup(t *testing.T) {
	backend.ResetDetectCache()

	root, cfgPath := scanTreeWithTrackedApp(t)

	err := runHRD(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanList, "-g", "@@none", root})
	require.Error(t, err)
}

func TestRepoLsReservedNone(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"grouped":   {Path: "/g", Groups: []string{"work"}},
			"ungrouped": {Path: "/u"},
		},
	})

	out := runAppCapture(t, cfgPath, []string{"repo", "ls", "-g", "@@none"})
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

func TestRepoRootAddRejectsReservedGroup(t *testing.T) {
	dir := t.TempDir()
	cfgPath := setupTestConfig(t, config.Config{})

	err := runHRD(t, cfgPath, []string{"repo", "root", "add", dir, "-g", "@@none"})
	require.Error(t, err)
}
