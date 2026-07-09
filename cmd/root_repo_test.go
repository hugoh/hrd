package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hugoh/hrd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoRootAdd(t *testing.T) {
	dir := t.TempDir()
	cfgPath := setupTestConfig(t, config.Config{})

	err := runHRD(t, cfgPath, []string{"repo", "root", "add", dir})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	require.Contains(t, cfg.Roots, filepath.Base(dir))
}

func TestRepoRootAddExplicitName(t *testing.T) {
	dir := t.TempDir()
	cfgPath := setupTestConfig(t, config.Config{})

	err := runHRD(t, cfgPath, []string{"repo", "root", "add", dir, "-n", "myroots"})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	require.Contains(t, cfg.Roots, "myroots")
	assert.Equal(t, dir, cfg.Roots["myroots"].Path)
}

func TestRepoRootAddDepthAndGroup(t *testing.T) {
	dir := t.TempDir()
	cfgPath := setupTestConfig(t, config.Config{})

	err := runHRD(t, cfgPath, []string{
		"repo", "root", "add", dir, "-n", "myroots", "--depth", "3", "-g", "personal",
	})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	require.Contains(t, cfg.Roots, "myroots")
	assert.Equal(t, 3, cfg.Roots["myroots"].Depth)
	assert.Equal(t, []string{"personal"}, cfg.Roots["myroots"].Groups)
}

func TestRepoRootAddNoArgs(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	err := runHRD(t, cfgPath, []string{"repo", "root", "add"})
	require.Error(t, err)
}

func TestRepoRootAddNameCollision(t *testing.T) {
	dir := t.TempDir()
	cfgPath := setupTestConfig(t, config.Config{
		Roots: map[string]config.Root{"myroots": {Path: "/somewhere/else"}},
	})

	err := runHRD(t, cfgPath, []string{"repo", "root", "add", dir, "-n", "myroots"})
	require.Error(t, err)
}

func TestRepoRootRm(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Roots: map[string]config.Root{"myroots": {Path: "/somewhere"}},
	})

	err := runHRD(t, cfgPath, []string{"repo", "root", "rm", "myroots"})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.NotContains(t, cfg.Roots, "myroots")
}

func TestRepoRootRmUnknown(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	err := runHRD(t, cfgPath, []string{"repo", "root", "rm", "nonexistent"})
	require.Error(t, err)
}

func TestRepoRootLs(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{
		Roots: map[string]config.Root{
			"myroots": {Path: "/somewhere", Depth: 2, Groups: []string{"personal"}},
		},
	})

	out := runAppCapture(t, cfgPath, []string{"repo", "root", "ls"})
	assert.Contains(t, out, "myroots")
	assert.Contains(t, out, "/somewhere")
}

func TestRepoRootLsEmpty(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})

	out := runAppCapture(t, cfgPath, []string{"repo", "root", "ls"})
	assert.Empty(t, strings.TrimSpace(out))
}
