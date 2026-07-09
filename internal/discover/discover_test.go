package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	git.Register()
	m.Run()
}

func fakeGitDirAt(t *testing.T, dir string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o750))
}

// tree builds:
//
//	root/work/app             repo
//	root/work/app/vendor/dep  nested repo (must be skipped)
//	root/oss/app              repo
//	root/.archive/old         repo in hidden dir (must be skipped)
//	root/plain                not a repo
func tree(t *testing.T) string {
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

func TestRepos_FindsRepos(t *testing.T) {
	backend.ResetDetectCache()

	root := tree(t)

	found, warnings, err := Repos(root, 5)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.ElementsMatch(t, []string{
		filepath.Join(root, "work", "app"),
		filepath.Join(root, "oss", "app"),
	}, found)
}

func TestRepos_RespectsDepth(t *testing.T) {
	backend.ResetDetectCache()

	root := tree(t)

	found, _, err := Repos(root, 1)
	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestRepos_SkipsNested(t *testing.T) {
	backend.ResetDetectCache()

	root := tree(t)

	found, _, err := Repos(root, 5)
	require.NoError(t, err)
	assert.NotContains(t, found, filepath.Join(root, "work", "app", "vendor", "dep"))
}

func TestDepth(t *testing.T) {
	root := "/a"
	assert.Equal(t, 0, Depth(root, "/a"))
	assert.Equal(t, 1, Depth(root, "/a/b"))
	assert.Equal(t, 2, Depth(root, "/a/b/c"))
}
