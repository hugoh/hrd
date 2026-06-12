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

// fakeGitDirAt creates a minimal .git layout under dir so backend.Detect
// recognizes it.
func fakeGitDirAt(t *testing.T, dir string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o750))
}

// scanTree builds a directory tree for scan tests:
//
//	root/work/app        repo
//	root/work/app/vendor/dep  nested repo (must be skipped)
//	root/oss/app         repo (name conflicts with work/app)
//	root/.archive/old    repo in hidden dir (must be skipped)
//	root/plain           not a repo
func scanTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	for _, p := range []string{"work/app", "work/app/vendor/dep", "oss/app", ".archive/old"} {
		dir := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(dir, 0o750))
		fakeGitDirAt(t, dir)
	}

	require.NoError(t, os.MkdirAll(filepath.Join(root, "plain"), 0o750))

	return root
}

func TestRepoScan(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	cfgPath := setupTestConfig(t, config.Config{})

	err := runApp(t, cfgPath, []string{"repo", cmdNameScan, root})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	assert.Len(t, cfg.Repos, 2)
	assert.Equal(t, filepath.Join(root, "oss", "app"), cfg.Repos["app"].Path)
	assert.Equal(t, filepath.Join(root, "work", "app"), cfg.Repos["work-app"].Path)
}

func TestRepoScanIdempotent(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	cfgPath := setupTestConfig(t, config.Config{})

	require.NoError(t, runApp(t, cfgPath, []string{"repo", cmdNameScan, root}))
	require.NoError(t, runApp(t, cfgPath, []string{"repo", cmdNameScan, root}))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Len(t, cfg.Repos, 2)
}

func TestRepoScanDryRun(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	cfgPath := setupTestConfig(t, config.Config{})

	out := runAppCapture(t, cfgPath, []string{"repo", cmdNameScan, "--dry-run", root})
	assert.Contains(t, out, "would add")

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Empty(t, cfg.Repos)
}

func TestRepoScanGroup(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	cfgPath := setupTestConfig(t, config.Config{})

	require.NoError(t, runApp(t, cfgPath, []string{"repo", cmdNameScan, "-g", "work", root}))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"app", "work-app"}, cfg.Groups["work"].Repos)
}

func TestRepoScanDepthLimit(t *testing.T) {
	backend.ResetDetectCache()

	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c", "repo")
	require.NoError(t, os.MkdirAll(deep, 0o750))
	fakeGitDirAt(t, deep)

	cfgPath := setupTestConfig(t, config.Config{})

	require.NoError(t, runApp(t, cfgPath, []string{"repo", cmdNameScan, "--depth", "2", root}))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Empty(t, cfg.Repos, "repo below --depth should not be added")

	require.NoError(t, runApp(t, cfgPath, []string{"repo", cmdNameScan, root}))

	cfg, err = config.Load(cfgPath)
	require.NoError(t, err)
	assert.Len(t, cfg.Repos, 1)
}

func TestRepoScanNoArgs(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})
	err := runApp(t, cfgPath, []string{"repo", cmdNameScan})
	require.ErrorIs(t, err, errAtLeastOnePath)
}

func TestRepoScanNameConflictSkipped(t *testing.T) {
	backend.ResetDetectCache()

	root := t.TempDir()

	// Three repos named "app" whose parents are all named "x": the first
	// takes "app", the second the "x-app" fallback, the third has no
	// available name and must be skipped.
	for _, p := range []string{"a/x/app", "b/x/app", "c/x/app"} {
		dir := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(dir, 0o750))
		fakeGitDirAt(t, dir)
	}

	cfgPath := setupTestConfig(t, config.Config{})
	require.NoError(t, runApp(t, cfgPath, []string{"repo", cmdNameScan, root}))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Len(t, cfg.Repos, 2)
	assert.Contains(t, cfg.Repos, "app")
	assert.Contains(t, cfg.Repos, "x-app")
}

func TestRepoAddWithGroup(t *testing.T) {
	backend.ResetDetectCache()

	repoDir := setupFakeGitRepo(t)
	cfgPath := setupTestConfig(t, config.Config{})

	require.NoError(t, runApp(t, cfgPath, []string{"repo", "add", "-g", "@work", repoDir}))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	require.Len(t, cfg.Repos, 1)
	assert.Equal(t, []string{filepath.Base(repoDir)}, cfg.Groups["work"].Repos)
}
