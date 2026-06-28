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

func TestRepoScanAdd(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	cfgPath := setupTestConfig(t, config.Config{})

	err := runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd, root})
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	assert.Len(t, cfg.Repos, 2)
	assert.Equal(t, filepath.Join(root, "oss", "app"), cfg.Repos["app"].Path)
	assert.Equal(t, filepath.Join(root, "work", "app"), cfg.Repos["work-app"].Path)
}

func TestRepoScanAddIdempotent(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	cfgPath := setupTestConfig(t, config.Config{})

	require.NoError(t, runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd, root}))
	require.NoError(t, runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd, root}))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Len(t, cfg.Repos, 2)
}

func TestRepoScanAddGroup(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	cfgPath := setupTestConfig(t, config.Config{})

	require.NoError(
		t,
		runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd, "-g", "work", root}),
	)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"app", "work-app"}, cfg.Groups["work"].Repos)
}

func TestRepoScanAddDepthLimit(t *testing.T) {
	backend.ResetDetectCache()

	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c", "repo")
	require.NoError(t, os.MkdirAll(deep, 0o750))
	fakeGitDirAt(t, deep)

	cfgPath := setupTestConfig(t, config.Config{})

	require.NoError(
		t,
		runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd, "--depth", "2", root}),
	)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Empty(t, cfg.Repos, "repo below --depth should not be added")

	require.NoError(t, runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd, root}))

	cfg, err = config.Load(cfgPath)
	require.NoError(t, err)
	assert.Len(t, cfg.Repos, 1)
}

func TestRepoScanAddNoArgs(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})
	err := runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd})
	require.ErrorIs(t, err, errAtLeastOnePath)
}

func TestRepoScanAddNameConflictSkipped(t *testing.T) {
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
	require.NoError(t, runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd, root}))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Len(t, cfg.Repos, 2)
	assert.Contains(t, cfg.Repos, "app")
	assert.Contains(t, cfg.Repos, "x-app")
}

func TestRepoScanAddPattern(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	cfgPath := setupTestConfig(t, config.Config{})

	// "app" matches both repos by basename; filter to confirm both are found
	require.NoError(
		t,
		runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd, "--pattern", "app", root}),
	)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Len(t, cfg.Repos, 2)

	// Reset and use a non-matching pattern
	cfgPath2 := setupTestConfig(t, config.Config{})
	require.NoError(
		t,
		runApp(
			t,
			cfgPath2,
			[]string{"repo", cmdNameScan, cmdNameScanAdd, "--pattern", "nomatch*", root},
		),
	)

	cfg2, err := config.Load(cfgPath2)
	require.NoError(t, err)
	assert.Empty(t, cfg2.Repos)
}

func TestRepoScanAddConfirm(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	cfgPath := setupTestConfig(t, config.Config{})

	// WalkDir visits oss/app before work/app (lexicographic order).
	// Accept the first repo, reject the second.
	responses := []bool{true, false}
	i := 0
	old := confirmFn
	confirmFn = func(string) bool {
		r := responses[i]
		i++

		return r
	}

	t.Cleanup(func() { confirmFn = old })

	require.NoError(
		t,
		runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanAdd, "--confirm", root}),
	)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Len(t, cfg.Repos, 1)
	assert.Contains(t, cfg.Repos, "app", "only the confirmed repo should be added")
}

func TestRepoScanListNoArgs(t *testing.T) {
	cfgPath := setupTestConfig(t, config.Config{})
	err := runApp(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanList})
	require.ErrorIs(t, err, errAtLeastOnePath)
}

func TestRepoScanList(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)

	// Pre-populate config with one of the two repos.
	ossApp := filepath.Join(root, "oss", "app")
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"app": {Path: ossApp},
		},
	})

	out := runAppCapture(t, cfgPath, []string{"repo", cmdNameScan, cmdNameScanList, root})

	// Both status values and both names should appear in the output.
	assert.Contains(t, out, "tracked")
	assert.Contains(t, out, "untracked")
	assert.Contains(t, out, "work-app")
}

func TestRepoScanListTracked(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	ossApp := filepath.Join(root, "oss", "app")
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"app": {Path: ossApp},
		},
	})

	out := runAppCapture(
		t,
		cfgPath,
		[]string{"repo", cmdNameScan, cmdNameScanList, "--tracked", root},
	)

	// Only the tracked repo ("app") should appear; work-app is untracked.
	assert.NotContains(t, out, "work-app")
	assert.NotContains(t, out, "untracked")
}

func TestRepoScanListUntracked(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	ossApp := filepath.Join(root, "oss", "app")
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"app": {Path: ossApp},
		},
	})

	out := runAppCapture(
		t,
		cfgPath,
		[]string{"repo", cmdNameScan, cmdNameScanList, "--untracked", root},
	)

	// Only the untracked repo should appear; "app" (ossApp) is already tracked.
	assert.Contains(t, out, "work-app")
}

func TestRepoScanListGroup(t *testing.T) {
	backend.ResetDetectCache()

	root := scanTree(t)
	ossApp := filepath.Join(root, "oss", "app")
	cfgPath := setupTestConfig(t, config.Config{
		Repos: map[string]config.Repo{
			"app":      {Path: ossApp},
			"work-app": {Path: filepath.Join(root, "work", "app")},
		},
	})

	// Assign only the "app" repo (oss/app) to the spoon group via scan list.
	args := []string{
		"repo", cmdNameScan, cmdNameScanList,
		"-p", "app", "-g", "spoon", "--tracked", root,
	}
	require.NoError(t, runApp(t, cfgPath, args))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"app", "work-app"}, cfg.Groups["spoon"].Repos)
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
