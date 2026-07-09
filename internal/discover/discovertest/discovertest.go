// Package discovertest holds shared directory-tree fixtures for tests that
// exercise repo discovery, so cmd's scan tests and internal/discover's own
// tests don't each re-implement the same fixture.
package discovertest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const dirPerm = 0o750

// FakeGitDirAt creates a minimal .git layout under dir so backend.Detect
// recognizes it.
func FakeGitDirAt(t *testing.T, dir string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), dirPerm))
}

// Tree builds a directory tree for discovery tests:
//
//	root/work/app             repo
//	root/work/app/vendor/dep  nested repo (must be skipped)
//	root/oss/app              repo (name conflicts with work/app)
//	root/.archive/old         repo in hidden dir (must be skipped)
//	root/plain                not a repo
func Tree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	for _, p := range []string{"work/app", "work/app/vendor/dep", "oss/app", ".archive/old"} {
		dir := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(dir, dirPerm))
		FakeGitDirAt(t, dir)
	}

	require.NoError(t, os.MkdirAll(filepath.Join(root, "plain"), dirPerm))

	return root
}
